// core/internal/audio/fast-processing.go
package audio

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
)

// FastNormalizeLoudness performs single-pass normalization for speed over accuracy
// This uses FFmpeg's loudnorm filter without the analysis pass, making it much faster
// but less precise than the two-pass method in normalizer.go
func FastNormalizeLoudness(inputFile, outputFile string, targetLUFS float64, options OutputOptions) error {
	log.Printf("Starting FAST normalization: %s -> %s (target: %.1f LUFS)", inputFile, outputFile, targetLUFS)

	// Validate LUFS range
	if targetLUFS < MinLUFS || targetLUFS > MaxLUFS {
		return fmt.Errorf("target LUFS %.1f is outside valid range (%.1f to %.1f)", targetLUFS, MinLUFS, MaxLUFS)
	}

	// Single-pass loudnorm filter (no measured values, faster but less accurate)
	filterChain := fmt.Sprintf("loudnorm=I=%f:TP=-1.5:LRA=11", targetLUFS)

	args := []string{
		"-i", inputFile,
		"-af", filterChain,
	}

	// Determine output format based on file extension or options
	outputExt := strings.ToLower(filepath.Ext(outputFile))

	if options.Codec != "" {
		args = append(args, "-c:a", options.Codec)
	} else {
		switch outputExt {
		case ".mp3":
			args = append(args, "-c:a", "libmp3lame")
		case ".wav":
			args = append(args, "-c:a", "pcm_s16le")
		case ".flac":
			args = append(args, "-c:a", "flac")
		default:
			args = append(args, "-c:a", "pcm_s16le")
		}
	}

	if options.Bitrate != "" {
		args = append(args, "-b:a", options.Bitrate)
	} else {
		// Auto-select bitrate based on format
		switch outputExt {
		case ".mp3":
			args = append(args, "-b:a", "320k")
		case ".wav", ".flac":
			// Lossless formats don't need bitrate
		}
	}

	// Set sample rate to maintain quality
	args = append(args, "-ar", "44100")

	// Add any extra options
	if len(options.ExtraOptions) > 0 {
		args = append(args, options.ExtraOptions...)
	}

	// Overwrite output file if it exists
	args = append(args, "-y", outputFile)

	log.Printf("FFmpeg FAST normalize command: ffmpeg %s", strings.Join(args, " "))

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Printf("FFmpeg FAST error: %v", err)
		log.Printf("FFmpeg FAST output: %s", string(output))
		return fmt.Errorf("fast normalization failed: %w", err)
	}

	log.Printf("FAST normalization completed successfully")
	return nil
}

// EstimateLoudness performs a quick analysis to get approximate loudness levels
// This is faster than the full analysis but less accurate
func EstimateLoudness(inputFile string) (*LoudnessInfo, error) {
	log.Printf("Starting loudness estimation for file: %s", inputFile)

	// Use a shorter analysis duration for speed
	cmd := exec.Command("ffmpeg",
		"-t", "30", // Analyze only first 30 seconds
		"-i", inputFile,
		"-af", "loudnorm=print_format=json:I=-16:TP=-1.5:LRA=11",
		"-f", "null", "-")

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("FFmpeg FAST estimation error: %v", err)
		return nil, fmt.Errorf("fast loudness estimation failed: %w", err)
	}

	// Parse the same way as the full analysis
	return parseLoudnormOutput(output)
}

// ProcessingMode represents different processing modes
type ProcessingMode string

const (
	ModePrecise ProcessingMode = "precise" // Two-pass, accurate but slower
	ModeFast    ProcessingMode = "fast"    // Single-pass, faster but less accurate
)

// ProcessAudioWithMode processes audio using the specified mode
func ProcessAudioWithMode(inputFile, outputFile string, targetLUFS float64, options OutputOptions, mode ProcessingMode) error {
	switch mode {
	case ModeFast:
		return FastNormalizeLoudness(inputFile, outputFile, targetLUFS, options)
	case ModePrecise:
		// Use the existing two-pass method
		info, err := AnalyzeLoudness(inputFile)
		if err != nil {
			return fmt.Errorf("analysis failed: %w", err)
		}
		return NormalizeLoudness(inputFile, outputFile, targetLUFS, info, options)
	default:
		return fmt.Errorf("unknown processing mode: %s", mode)
	}
}

// ValidateProcessingMode checks if the processing mode is valid
func ValidateProcessingMode(mode string) (ProcessingMode, error) {
	switch strings.ToLower(mode) {
	case "fast":
		return ModeFast, nil
	case "precise", "accurate":
		return ModePrecise, nil
	default:
		return ModePrecise, fmt.Errorf("invalid processing mode: %s. Use 'fast' or 'precise'", mode)
	}
}
