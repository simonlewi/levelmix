package audio

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// NormalizeLoudness performs the second pass using measured values for accurate normalization
func NormalizeLoudness(inputFile, outputFile string, targetLUFS float64, info *LoudnessInfo, options OutputOptions) error {
	log.Printf("Starting normalization: %s -> %s (target: %.1f LUFS)", inputFile, outputFile, targetLUFS)

	numThreads := runtime.NumCPU()
	log.Printf("Using %d CPU threads for FFmpeg processing", numThreads)

	// Validate LUFS range
	if targetLUFS < MinLUFS || targetLUFS > MaxLUFS {
		return fmt.Errorf("target LUFS %.1f is outside valid range (%.1f to %.1f)", targetLUFS, MinLUFS, MaxLUFS)
	}

	// Use measured values for linear normalization (most accurate method)
	filterChain := fmt.Sprintf("loudnorm=I=%f:TP=-1.5:LRA=11:measured_I=%f:measured_TP=%f:measured_LRA=%f:measured_thresh=%f:linear=true",
		targetLUFS, info.InputI, info.InputTP, info.InputLRA, info.InputThresh)

	args := []string{
		// Input optimization flags
		"-threads", fmt.Sprintf("%d", numThreads),
		"-thread_queue_size", "512",
		"-i", inputFile,
		// Audio filter
		"-af", filterChain,
		// Output optimization flags
		"-threads", fmt.Sprintf("%d", numThreads),
		"-preset", "ultrafast",
		"-movflags", "+faststart",
		"-max_muxing_queue_size", "9999",
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
			// Default to high-quality PCM for unknown formats
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

	log.Printf("FFmpeg normalize command: ffmpeg %s", strings.Join(args, " "))

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Printf("FFmpeg error: %v", err)
		log.Printf("FFmpeg output: %s", string(output))
		return fmt.Errorf("normalization failed: %w", err)
	}

	log.Printf("Normalization completed successfully")
	return nil
}

// Utility function to validate LUFS values
func ValidateLUFS(lufs float64) error {
	if lufs < MinLUFS || lufs > MaxLUFS {
		return fmt.Errorf("LUFS value %.1f is outside valid range (%.1f to %.1f)", lufs, MinLUFS, MaxLUFS)
	}
	return nil
}

// Utility function to get preset LUFS value by name
func GetPresetLUFS(preset string) (float64, error) {
	switch strings.ToLower(preset) {
	case "default", "club", "dj":
		return DefaultLUFS, nil
	case "streaming", "spotify", "apple":
		return StreamingLUFS, nil
	case "podcast":
		return PodcastLUFS, nil
	case "broadcast", "radio":
		return BroadcastLUFS, nil
	case "festival", "loud", "max":
		return MaxImpactLUFS, nil
	default:
		return 0, fmt.Errorf("unknown preset: %s", preset)
	}
}
