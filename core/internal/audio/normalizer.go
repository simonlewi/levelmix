package audio

import (
	"fmt"
	"os/exec"
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
	filterChain := fmt.Sprintf("loudnorm=I=%f:TP=-1.5:LRA=11:measured_I=%f:measured_TP=%f:measured_LRA=%f:measured_thresh=%f:linear=true,alimiter=limit=0.9:attack=5:release=50:level=disabled",
		targetLUFS, info.InputI, info.InputTP, info.InputLRA, info.InputThresh)

	args := []string{
		"-i", inputFile,
		"-af", filterChain,
	}

	if options.Codec != "" {
		args = append(args, "-c:a", options.Codec)
	} else {
		args = append(args, "-c:a", "pcm_s16le")
	}

	if options.Bitrate != "" {
		args = append(args, "-b:a", options.Bitrate)
	}

	if len(options.ExtraOptions) > 0 {
		args = append(args, options.ExtraOptions...)
	}

	args = append(args, outputFile)
	cmd := exec.Command("ffmpeg", args...)
	return cmd.Run()
}
