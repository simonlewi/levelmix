package audio

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Global semaphore to limit concurrent FFmpeg processes
var ffmpegSemaphore = make(chan struct{}, 4)

// SetDebugMode enables or disables debug logging
func SetDebugMode(enabled bool) {
	debugMode = enabled
	if enabled {
		log.Printf("[DEBUG] Debug mode enabled")
	}
}

// AnalyzeLoudness performs the first pass to measure audio loudness with timeout
func AnalyzeLoudness(inputFile string) (*LoudnessInfo, error) {
	return AnalyzeLoudnessWithTimeout(inputFile, 15*time.Minute)
}

// AnalyzeLoudnessWithTimeout performs loudness analysis with configurable timeout
func AnalyzeLoudnessWithTimeout(inputFile string, timeout time.Duration) (info *LoudnessInfo, err error) {
	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic recovered: %v", r)
			log.Printf("[ERROR] Panic in AnalyzeLoudness: %v", r)
		}
	}()

	// Acquire semaphore to limit concurrent FFmpeg processes
	select {
	case ffmpegSemaphore <- struct{}{}:
		defer func() { <-ffmpegSemaphore }()
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("timeout waiting for FFmpeg slot")
	}

	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("input file does not exist: %s", inputFile)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-i", inputFile,
		"-af", "loudnorm=print_format=json:I=-16:TP=-1.5:LRA=11",
		"-f", "null", "-")

	// Get pipes for streaming output
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Read output in a goroutine to prevent deadlock
	outputChan := make(chan []byte, 1)
	errorChan := make(chan error, 1)

	go func() {
		output := make([]byte, 0, 4096)
		buffer := make([]byte, 1024)
		for {
			n, err := stderr.Read(buffer)
			if n > 0 {
				output = append(output, buffer[:n]...)
			}
			if err != nil {
				if err.Error() != "EOF" && !strings.Contains(err.Error(), "file already closed") {
					errorChan <- err
				}
				break
			}
		}
		outputChan <- output
	}()

	// Wait for command to complete
	cmdErr := cmd.Wait()

	// Check if context timed out
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("ffmpeg analysis timed out after %v", timeout)
	}

	// Get output
	select {
	case output := <-outputChan:
		if cmdErr != nil {
			return nil, fmt.Errorf("ffmpeg analysis failed: %w", cmdErr)
		}
		result, err := parseLoudnormOutput(output)
		if err != nil {
			return nil, err
		}

		// Log result only in INFO level
		log.Printf("[INFO] Loudness analysis: %.1f LUFS, Peak: %.1f dB", result.InputI, result.InputTP)
		return result, nil

	case err := <-errorChan:
		return nil, fmt.Errorf("error reading ffmpeg output: %w", err)
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout reading ffmpeg output")
	}
}

// getDuration gets the duration of an audio file using ffprobe with timeout
func getDuration(inputFile string) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inputFile)

	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return 0, fmt.Errorf("ffprobe timed out")
		}
		return 0, fmt.Errorf("failed to get file duration: %w", err)
	}

	var duration float64
	if _, err := fmt.Sscanf(string(output), "%f", &duration); err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}
	return duration, nil
}

// AnalyzeLoudnessAdaptiveSample performs adaptive sampling based on file duration
// Note: For DJ mixes with high dynamic range, full analysis (Precise mode) is recommended
func AnalyzeLoudnessAdaptiveSample(inputFile string) (*LoudnessInfo, error) {
	// Check disk space before processing
	if err := checkDiskSpace(); err != nil {
		return nil, fmt.Errorf("insufficient disk space: %w", err)
	}

	duration, err := getDuration(inputFile)
	if err != nil {
		return nil, err
	}

	// Adaptive sampling strategy based on duration
	var sampleLength float64
	var samplePoints []float64

	switch {
	case duration <= 60: // Short files (<1 min)
		// Analyze the whole file for short clips
		return AnalyzeLoudness(inputFile)

	case duration <= 180: // Medium files (1-3 min)
		sampleLength = 20.0
		samplePoints = []float64{0.15, 0.50, 0.85}

	case duration <= 600: // Long files (3-10 min)
		sampleLength = 30.0
		samplePoints = []float64{0.10, 0.30, 0.50, 0.70, 0.90}

	default: // Very long files (>10 min)
		sampleLength = 30.0
		samplePoints = []float64{0.10, 0.25, 0.40, 0.55, 0.70, 0.80, 0.90}
	}

	// Calculate total sample coverage
	totalSampleTime := sampleLength * float64(len(samplePoints))
	coveragePercent := (totalSampleTime / duration) * 100

	// If we're sampling more than 60% of the file, just analyze the whole thing
	if coveragePercent > 60 {
		return AnalyzeLoudness(inputFile)
	}

	if debugMode {
		log.Printf("[DEBUG] Adaptive analysis: %d samples from %.1fs file (%.1f%% coverage)",
			len(samplePoints), duration, coveragePercent)
	}

	return performMultiSampleAnalysis(inputFile, samplePoints, sampleLength, duration)
}

// performMultiSampleAnalysis executes the sampling strategy with proper LUFS math
func performMultiSampleAnalysis(inputFile string, samplePoints []float64, sampleLength, duration float64) (*LoudnessInfo, error) {
	var energyValues []float64
	var validSamples int
	var failedSamples int
	var maxPeak float64 = -math.MaxFloat64
	var totalLRA float64
	var minLUFS float64 = math.MaxFloat64
	var maxLUFS float64 = -math.MaxFloat64

	var mu sync.Mutex
	maxFailures := len(samplePoints) / 2 // Allow up to 50% failure rate

	for i, p := range samplePoints {
		startTime := duration * p

		// Ensure we don't go past the end
		if startTime+sampleLength > duration {
			startTime = duration - sampleLength
			if startTime < 0 {
				startTime = 0
				sampleLength = duration
			}
		}

		// Acquire semaphore for each sample
		select {
		case ffmpegSemaphore <- struct{}{}:
			// Acquired
		case <-time.After(30 * time.Second):
			failedSamples++
			if failedSamples > maxFailures {
				return nil, fmt.Errorf("too many failed samples (%d/%d)", failedSamples, len(samplePoints))
			}
			continue
		}

		// Process sample with timeout
		func() {
			defer func() { <-ffmpegSemaphore }() // Release semaphore

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			cmd := exec.CommandContext(ctx,
				"ffmpeg",
				"-ss", fmt.Sprintf("%.2f", startTime),
				"-t", fmt.Sprintf("%.2f", sampleLength),
				"-i", inputFile,
				"-af", "loudnorm=print_format=json:I=-16:TP=-1.5:LRA=11",
				"-f", "null", "-")

			output, err := cmd.CombinedOutput()
			if err != nil {
				mu.Lock()
				failedSamples++
				mu.Unlock()

				if ctx.Err() == context.DeadlineExceeded {
					log.Printf("[WARN] Sample %d timed out at position %.1fs", i, startTime)
				} else if debugMode {
					log.Printf("[DEBUG] Sample %d failed at position %.1fs: %v", i, startTime, err)
				}

				// Early failure detection
				if failedSamples > maxFailures {
					return
				}
				return
			}

			info, err := parseLoudnormOutput(output)
			if err != nil {
				mu.Lock()
				failedSamples++
				mu.Unlock()
				if debugMode {
					log.Printf("[DEBUG] Sample %d: failed to parse output: %v", i, err)
				}
				return
			}

			mu.Lock()
			defer mu.Unlock()

			// Track variance for quality check
			if info.InputI < minLUFS {
				minLUFS = info.InputI
			}
			if info.InputI > maxLUFS {
				maxLUFS = info.InputI
			}

			// Convert LUFS to linear energy for proper averaging
			energy := math.Pow(10, (info.InputI+0.691)/10)
			energyValues = append(energyValues, energy)

			// Track peak values
			if info.InputTP > maxPeak {
				maxPeak = info.InputTP
			}

			totalLRA += info.InputLRA
			validSamples++

			if debugMode {
				log.Printf("[DEBUG] Sample %d: %.1f LUFS at position %.1fs", i, info.InputI, startTime)
			}
		}()

		// Check for early exit
		if failedSamples > maxFailures {
			return nil, fmt.Errorf("too many failed samples (%d/%d)", failedSamples, len(samplePoints))
		}
	}

	if validSamples == 0 {
		return nil, fmt.Errorf("failed to get any valid loudness samples")
	}

	// Check variance - if too high, warn but continue
	variance := maxLUFS - minLUFS
	if variance > 6.0 {
		log.Printf("[WARN] High loudness variance: %.1f LU (may indicate inconsistent mix)", variance)
	}

	// Calculate the weighted average in energy domain, then convert back to LUFS
	var totalEnergy float64
	for _, energy := range energyValues {
		totalEnergy += energy
	}
	avgEnergy := totalEnergy / float64(len(energyValues))
	avgLUFS := 10*math.Log10(avgEnergy) - 0.691
	avgLRA := totalLRA / float64(validSamples)

	log.Printf("[INFO] Multi-sample analysis: %.1f LUFS, Peak: %.1f dB (%d/%d samples)",
		avgLUFS, maxPeak, validSamples, len(samplePoints))

	return &LoudnessInfo{
		InputI:      avgLUFS,
		InputTP:     maxPeak,
		InputLRA:    avgLRA,
		InputThresh: avgLUFS - 10,
	}, nil
}

// parseLoudnormOutput parses FFmpeg loudnorm JSON output
func parseLoudnormOutput(output []byte) (*LoudnessInfo, error) {
	outputStr := string(output)
	jsonStart := strings.Index(outputStr, "{")
	if jsonStart == -1 {
		// Only log in debug mode - this can be verbose
		if debugMode {
			log.Printf("[DEBUG] No JSON found in ffmpeg output (first 200 chars): %s",
				truncateString(outputStr, 200))
		}
		return nil, fmt.Errorf("no JSON data found in ffmpeg output")
	}

	jsonStr := outputStr[jsonStart:]
	jsonEnd := strings.LastIndex(jsonStr, "}") + 1
	if jsonEnd == 0 {
		return nil, fmt.Errorf("malformed JSON data in ffmpeg output")
	}

	jsonData := jsonStr[:jsonEnd]

	// FFmpeg outputs with underscores, map them correctly
	var data struct {
		InputI       string `json:"input_i"`
		InputTP      string `json:"input_tp"`
		InputLRA     string `json:"input_lra"`
		InputThresh  string `json:"input_thresh"`
		TargetOffset string `json:"target_offset"`
	}

	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		if debugMode {
			log.Printf("[DEBUG] Failed to parse JSON: %s", truncateString(jsonData, 500))
		}
		return nil, fmt.Errorf("failed to parse JSON data: %w", err)
	}

	// Convert string values to float64
	var info LoudnessInfo
	if v, err := strconv.ParseFloat(data.InputI, 64); err == nil {
		info.InputI = v
	} else {
		return nil, fmt.Errorf("failed to parse input_i: %w", err)
	}
	if v, err := strconv.ParseFloat(data.InputTP, 64); err == nil {
		info.InputTP = v
	} else {
		return nil, fmt.Errorf("failed to parse input_tp: %w", err)
	}
	if v, err := strconv.ParseFloat(data.InputLRA, 64); err == nil {
		info.InputLRA = v
	} else {
		return nil, fmt.Errorf("failed to parse input_lra: %w", err)
	}
	if v, err := strconv.ParseFloat(data.InputThresh, 64); err == nil {
		info.InputThresh = v
	} else {
		return nil, fmt.Errorf("failed to parse input_thresh: %w", err)
	}

	return &info, nil
}

// checkDiskSpace checks if there's sufficient disk space
func checkDiskSpace() error {
	tempDir := "/tmp/levelmix"

	// Ensure the directory exists
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("cannot create temp directory: %w", err)
	}

	// Try to create a small test file to verify write permissions
	testFile := tempDir + "/levelmix_test_" + strconv.FormatInt(time.Now().Unix(), 10)
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("cannot write to temp directory: %w", err)
	}
	f.Close()
	os.Remove(testFile)

	return nil
}

// SetFFmpegConcurrency allows adjusting the global FFmpeg concurrency limit
func SetFFmpegConcurrency(n int) {
	if n < 1 {
		n = 1
	}
	if n > 10 {
		n = 10 // Cap at reasonable maximum
	}

	// Create new semaphore with new limit
	newSem := make(chan struct{}, n)

	// Drain old semaphore
	oldSem := ffmpegSemaphore
	ffmpegSemaphore = newSem
	close(oldSem)

	log.Printf("[INFO] FFmpeg concurrency limit set to %d", n)
}

// truncateString truncates a string to maxLen for logging
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
