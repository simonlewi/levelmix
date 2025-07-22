// Updated normalizer.go to handle both MP3 and WAV files
package audio

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	DefaultLUFS   = -7.0  // Default target LUFS optimized for DJ content
	MaxImpactLUFS = -5.0  // Higher output for loud content
	StreamingLUFS = -14.0 // Streaming standard
	PodcastLUFS   = -16.0 // Podcast standard
	BroadcastLUFS = -23.0 // Broadcast standard

	MaxLUFS = -2.0  // Prevent clipping
	MinLUFS = -30.0 // Prevent inaudible output
)

type OutputOptions struct {
	Codec        string   // e.g., "pcm_s16le", "flac", "libmp3lame", "aac"
	Bitrate      string   // e.g., "320k" for MP3
	ExtraOptions []string // Any additional FFmpeg options
}

func NormalizeLoudness(
	inputFile, outputFile string,
	targetLUFS float64,
	info *LoudnessInfo,
	options OutputOptions,
) error {
	filterChain := fmt.Sprintf("loudnorm=I=%f:TP=-1.5:LRA=11:measured_I=%f:measured_TP=%f:measured_LRA=%f:measured_thresh=%f:linear=true,",
		targetLUFS, info.InputI, info.InputTP, info.InputLRA, info.InputThresh)

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

	if len(options.ExtraOptions) > 0 {
		args = append(args, options.ExtraOptions...)
	}

	// Overwrite output file if it exists
	args = append(args, "-y", outputFile)

	cmd := exec.Command("ffmpeg", args...)
	return cmd.Run()
}
