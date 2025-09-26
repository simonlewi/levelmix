// Audio processing functions for LevelMix
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
	log.Printf("Starting loudness analysis for file: %s", inputFile)

	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		log.Printf("ERROR: Input file does not exist: %s", inputFile)
		return nil, fmt.Errorf("input file does not exist: %s", inputFile)
	}

	cmd := exec.Command("ffmpeg",
		"-i", inputFile,
		"-af", "loudnorm=print_format=json:I=-16:TP=-1.5:LRA=11",
		"-f", "null", "-")

	log.Printf("FFmpeg command: %s", strings.Join(cmd.Args, " "))

	output, err := cmd.CombinedOutput()

	log.Printf("FFmpeg output: %s", string(output))

	if err != nil {
		log.Printf("FFmpeg error: %v", err)
		log.Printf("FFmpeg exit code: %s", err.Error())
		return nil, fmt.Errorf("loudness analysis failed: %w", err)
	}

	// Parse the JSON output from FFmpeg
	return parseLoudnormOutput(output)
}

// getDuration gets the duration of an audio file using ffprobe.
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
	log.Printf("Starting adaptive loudness sampling for file: %s", inputFile)

	duration, err := getDuration(inputFile)
	if err != nil {
		return nil, err
	}
	log.Printf("File duration: %.2f seconds", duration)

	// Adaptive sampling strategy based on duration
	var sampleLength float64
	var samplePoints []float64

	switch {
	case duration <= 60: // Short files (<1 min)
		// Analyze the whole file for short clips
		log.Printf("Short file detected, analyzing entire file")
		return AnalyzeLoudness(inputFile)

	case duration <= 180: // Medium files (1-3 min)
		// Use 20-second samples at 3 points
		sampleLength = 20.0
		samplePoints = []float64{0.15, 0.50, 0.85}

	case duration <= 600: // Long files (3-10 min)
		// Use 30-second samples at 5 points
		sampleLength = 30.0
		samplePoints = []float64{0.10, 0.30, 0.50, 0.70, 0.90}

	default: // Very long files (>10 min)
		// Use 30-second samples at 7 points
		sampleLength = 30.0
		samplePoints = []float64{0.10, 0.25, 0.40, 0.55, 0.70, 0.80, 0.90}
	}

	// Calculate total sample coverage
	totalSampleTime := sampleLength * float64(len(samplePoints))
	coveragePercent := (totalSampleTime / duration) * 100
	log.Printf("Sampling strategy: %.0f-second samples at %d points (%.1f%% coverage)",
		sampleLength, len(samplePoints), coveragePercent)

	// If we're sampling more than 60% of the file, just analyze the whole thing
	if coveragePercent > 60 {
		log.Printf("Sample coverage >60%%, analyzing entire file instead")
		return AnalyzeLoudness(inputFile)
	}

	return performMultiSampleAnalysis(inputFile, samplePoints, sampleLength, duration)
}

// performMultiSampleAnalysis executes the sampling strategy with proper LUFS math
func performMultiSampleAnalysis(inputFile string, samplePoints []float64, sampleLength, duration float64) (*LoudnessInfo, error) {
	// Collect energy values for proper LUFS calculation
	var energyValues []float64
	var validSamples int

	// Also track peak values and variance
	var maxPeak float64 = -math.MaxFloat64
	var totalLRA float64
	var minLUFS float64 = math.MaxFloat64
	var maxLUFS float64 = -math.MaxFloat64

	for i, p := range samplePoints {
		startTime := duration * p

		// Ensure we don't go past the end
		if startTime+sampleLength > duration {
			startTime = duration - sampleLength
			if startTime < 0 {
				startTime = 0
				sampleLength = duration // Adjust sample length for very short files
			}
		}

		log.Printf("Analyzing sample %d/%d at %.1f%% (%.2fs - %.2fs)...",
			i+1, len(samplePoints), p*100, startTime, startTime+sampleLength)

		cmd := exec.Command("ffmpeg",
			"-ss", fmt.Sprintf("%.2f", startTime),
			"-t", fmt.Sprintf("%.2f", sampleLength),
			"-i", inputFile,
			"-af", "loudnorm=print_format=json:I=-16:TP=-1.5:LRA=11",
			"-f", "null", "-")

		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("Sample %d failed: %v", i+1, err)
			continue
		}

		info, err := parseLoudnormOutput(output)
		if err != nil {
			log.Printf("Failed to parse sample %d: %v", i+1, err)
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
		// LUFS = -0.691 + 10 * log10(energy)
		// Therefore: energy = 10^((LUFS + 0.691) / 10)
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

	// Check variance - if too high, recommend full analysis
	variance := maxLUFS - minLUFS
	if variance > 6.0 {
		log.Printf("WARNING: High LUFS variance detected (%.1f LU range)", variance)
		log.Printf("Consider using full analysis for more accurate results")
		// Could optionally fall back to full analysis here:
		// return AnalyzeLoudness(inputFile)
	}

	// Calculate the weighted average in energy domain, then convert back to LUFS
	var totalEnergy float64
	for _, energy := range energyValues {
		totalEnergy += energy
	}
	avgEnergy := totalEnergy / float64(len(energyValues))

	// Convert back to LUFS
	avgLUFS := 10*math.Log10(avgEnergy) - 0.691

	// Calculate average LRA
	avgLRA := totalLRA / float64(validSamples)

	log.Printf("Multi-sample analysis complete:")
	log.Printf("  - Integrated loudness: %.2f LUFS (from %d samples)", avgLUFS, validSamples)
	log.Printf("  - True Peak: %.2f dB", maxPeak)
	log.Printf("  - Average LRA: %.2f LU", avgLRA)
	log.Printf("  - LUFS variance: %.2f LU", variance)
	log.Printf("  - Coverage: %.1f seconds of %.1f total",
		float64(validSamples)*sampleLength, duration)

	return &LoudnessInfo{
		InputI:      avgLUFS,
		InputTP:     maxPeak,
		InputLRA:    avgLRA,
		InputThresh: avgLUFS - 10, // Estimate threshold
	}, nil
}

// Helper function to parse FFmpeg loudnorm JSON output
func parseLoudnormOutput(output []byte) (*LoudnessInfo, error) {
	// Find the JSON part in the output
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

	// Debug: Log the raw JSON to see what we're getting
	log.Printf("Raw JSON from FFmpeg: %s", jsonData)

	// FFmpeg outputs with underscores, map to struct fields
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

	// Parse each field with error checking
	if v, err := strconv.ParseFloat(data.InputI, 64); err == nil {
		info.InputI = v
	} else {
		log.Printf("WARNING: Failed to parse input_i: %s", data.InputI)
	}

	if v, err := strconv.ParseFloat(data.InputTP, 64); err == nil {
		info.InputTP = v
	} else {
		log.Printf("WARNING: Failed to parse input_tp: %s", data.InputTP)
	}

	if v, err := strconv.ParseFloat(data.InputLRA, 64); err == nil {
		info.InputLRA = v
	} else {
		log.Printf("WARNING: Failed to parse input_lra: %s", data.InputLRA)
	}

	if v, err := strconv.ParseFloat(data.InputThresh, 64); err == nil {
		info.InputThresh = v
	} else {
		log.Printf("WARNING: Failed to parse input_thresh: %s", data.InputThresh)
	}

	log.Printf("Loudness analysis results - Input: %.1f LUFS, Peak: %.1f dB, LRA: %.1f LU, Threshold: %.1f",
		info.InputI, info.InputTP, info.InputLRA, info.InputThresh)

	return &info, nil
}
