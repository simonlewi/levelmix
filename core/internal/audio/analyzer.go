package audio

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// AnalyzeLoudness performs the first pass to measure audio loudness (full-file analysis)
func AnalyzeLoudness(inputFile string) (*LoudnessInfo, error) {
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("input file does not exist: %s", inputFile)
	}

	cmd := exec.Command("ffmpeg",
		"-i", inputFile,
		"-af", "loudnorm=print_format=json:I=-16:TP=-1.5:LRA=11",
		"-f", "null", "-")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("loudness analysis failed: %w", err)
	}

	return parseLoudnormOutput(output)
}

// getDuration gets the duration of an audio file using ffprobe
func getDuration(inputFile string) (float64, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", inputFile)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get file duration: %w", err)
	}
	var duration float64
	if _, err := fmt.Sscanf(string(output), "%f", &duration); err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}
	return duration, nil
}

// AnalyzeLoudnessAdaptiveSample performs adaptive sampling based on file duration
func AnalyzeLoudnessAdaptiveSample(inputFile string) (*LoudnessInfo, error) {
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

	log.Printf("Adaptive analysis: %d samples from %.1fs file", len(samplePoints), duration)
	return performMultiSampleAnalysis(inputFile, samplePoints, sampleLength, duration)
}

// performMultiSampleAnalysis executes the sampling strategy with proper LUFS math
func performMultiSampleAnalysis(inputFile string, samplePoints []float64, sampleLength, duration float64) (*LoudnessInfo, error) {
	var energyValues []float64
	var validSamples int
	var maxPeak float64 = -math.MaxFloat64
	var totalLRA float64
	var minLUFS float64 = math.MaxFloat64
	var maxLUFS float64 = -math.MaxFloat64

	for _, p := range samplePoints {
		startTime := duration * p

		// Ensure we don't go past the end
		if startTime+sampleLength > duration {
			startTime = duration - sampleLength
			if startTime < 0 {
				startTime = 0
				sampleLength = duration
			}
		}

		cmd := exec.Command("ffmpeg",
			"-ss", fmt.Sprintf("%.2f", startTime),
			"-t", fmt.Sprintf("%.2f", sampleLength),
			"-i", inputFile,
			"-af", "loudnorm=print_format=json:I=-16:TP=-1.5:LRA=11",
			"-f", "null", "-")

		output, err := cmd.CombinedOutput()
		if err != nil {
			continue // Skip failed samples silently
		}

		info, err := parseLoudnormOutput(output)
		if err != nil {
			continue
		}

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
	}

	if validSamples == 0 {
		return nil, fmt.Errorf("failed to get any valid loudness samples")
	}

	// Check variance - if too high, warn but continue
	variance := maxLUFS - minLUFS
	if variance > 6.0 {
		log.Printf("High loudness variance detected: %.1f LU", variance)
	}

	// Calculate the weighted average in energy domain, then convert back to LUFS
	var totalEnergy float64
	for _, energy := range energyValues {
		totalEnergy += energy
	}
	avgEnergy := totalEnergy / float64(len(energyValues))
	avgLUFS := 10*math.Log10(avgEnergy) - 0.691
	avgLRA := totalLRA / float64(validSamples)

	log.Printf("Analysis result: %.1f LUFS, Peak: %.1f dB", avgLUFS, maxPeak)

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
		return nil, fmt.Errorf("failed to parse JSON data: %w", err)
	}

	// Convert string values to float64
	var info LoudnessInfo
	if v, err := strconv.ParseFloat(data.InputI, 64); err == nil {
		info.InputI = v
	}
	if v, err := strconv.ParseFloat(data.InputTP, 64); err == nil {
		info.InputTP = v
	}
	if v, err := strconv.ParseFloat(data.InputLRA, 64); err == nil {
		info.InputLRA = v
	}
	if v, err := strconv.ParseFloat(data.InputThresh, 64); err == nil {
		info.InputThresh = v
	}

	return &info, nil
}
