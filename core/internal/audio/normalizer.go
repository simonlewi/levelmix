package audio

import (
	"fmt"
	"log"
	"math"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func NormalizeLoudness(inputFile, outputFile string, targetLUFS float64, info *LoudnessInfo, options OutputOptions, silenceInfo *SilenceInfo) error {
	numThreads := runtime.NumCPU()

	if targetLUFS < MinLUFS || targetLUFS > MaxLUFS {
		return fmt.Errorf("target LUFS %.1f is outside valid range (%.1f to %.1f)", targetLUFS, MinLUFS, MaxLUFS)
	}

	log.Printf("[INFO] Input: %.1f LUFS (LRA: %.1f, Peak: %.1f dB)", info.InputI, info.InputLRA, info.InputTP)

	useDJLevelNormalization := (targetLUFS == DJMixLUFS || targetLUFS == StreamingLUFS)

	var adjustedTarget float64
	var processingNote string
	var gainDB float64
	var predictedPeak float64
	var volumeReduction float64

	if useDJLevelNormalization && targetLUFS == StreamingLUFS {
		// For Streaming preset, first normalize to DJ level, then reduce volume
		djLevel := DJMixLUFS // -5.0 LUFS
		adjustedTarget, processingNote = calculateDynamicsAwareTarget(djLevel, info)
		gainDB = adjustedTarget - info.InputI
		predictedPeak = info.InputTP + gainDB
		volumeReduction = targetLUFS - djLevel
		log.Printf("[INFO] Streaming preset: normalizing to %.1f LUFS, then reducing by %.1f dB to reach %.1f LUFS",
			adjustedTarget, -volumeReduction, targetLUFS)
	} else {
		// Standard approach for DJ, Podcast, and Broadcast presets
		adjustedTarget, processingNote = calculateDynamicsAwareTarget(targetLUFS, info)
		gainDB = adjustedTarget - info.InputI
		predictedPeak = info.InputTP + gainDB
		volumeReduction = 0.0
		log.Printf("[INFO] Target: %.1f LUFS → Adjusted: %.1f LUFS (%s)", targetLUFS, adjustedTarget, processingNote)
	}

	log.Printf("[INFO] Applying %.1f dB gain for normalization (predicted peak: %.1f dB)", gainDB, predictedPeak)

	// Build filter chain
	var filters []string

	// Add trim filter first (if we have silence to remove)
	if silenceInfo != nil && silenceInfo.NeedsTrimming() {
		trimFilter := silenceInfo.TrimFilter()
		filters = append(filters, trimFilter)
		log.Printf("[INFO] Trimming: removing %.2fs from start, %.2fs from end",
			silenceInfo.TrimStart,
			silenceInfo.TotalDuration-silenceInfo.TrimEnd)
	}

	// Add normalization chain: gain + limiter (at DJ level)
	if predictedPeak > -1.0 {
		// Peaks will hit 0dB - limit and apply -1dB headroom
		filters = append(filters, fmt.Sprintf("volume=%.2fdB", gainDB))
		filters = append(filters, "alimiter=limit=1.0:level=false:attack=20:release=200")
		filters = append(filters, "volume=-1dB")
		log.Printf("[INFO] Normalizing with gain + limiter + headroom (-1dB)")
	} else {
		// Peaks stay safe - still apply limiter for consistency
		filters = append(filters, fmt.Sprintf("volume=%.2fdB", gainDB))
		filters = append(filters, "alimiter=limit=1.0:level=false:attack=20:release=200")
		log.Printf("[INFO] Normalizing with gain + limiter (peaks at %.1f dB)", predictedPeak)
	}

	// Add final volume reduction for quieter presets
	if volumeReduction != 0 {
		filters = append(filters, fmt.Sprintf("volume=%.2fdB", volumeReduction))
		log.Printf("[INFO] Reducing output by %.1f dB to reach final target", -volumeReduction)
	}

	filterChain := strings.Join(filters, ",")

	args := buildFFmpegArgs(inputFile, outputFile, filterChain, numThreads, options)

	log.Printf("[INFO] Processing audio (dynamics preserved)...")

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Printf("[ERROR] FFmpeg error: %v", err)
		log.Printf("[ERROR] FFmpeg output: %s", string(output))
		return fmt.Errorf("normalization failed: %w", err)
	}

	log.Printf("[INFO] Normalization complete")
	return nil
}

func calculateDynamicsAwareTarget(targetLUFS float64, info *LoudnessInfo) (float64, string) {
	lra := info.InputLRA

	var adjustment float64
	var description string

	switch {
	case lra < 4.0:
		adjustment = 0
		description = "low dynamics - direct targeting"
	case lra < 6.0:
		adjustment = (lra - 4.0) * 0.25
		description = fmt.Sprintf("moderate dynamics - +%.1f dB adjustment", adjustment)
	case lra < 9.0:
		adjustment = 0.5 + (lra-6.0)*0.33
		description = fmt.Sprintf("good dynamics - +%.1f dB adjustment", adjustment)
	case lra < 12.0:
		adjustment = 1.5 + (lra-9.0)*0.5
		description = fmt.Sprintf("high dynamics - +%.1f dB adjustment", adjustment)
	default:
		adjustment = math.Min(3.0+(lra-12.0)*0.25, 5.0)
		description = fmt.Sprintf("very high dynamics - +%.1f dB adjustment", adjustment)
	}

	adjusted := targetLUFS + adjustment

	if adjusted > MaxLUFS {
		description = fmt.Sprintf("%s (capped at %.0f LUFS)", description, MaxLUFS)
		return MaxLUFS, description
	}

	if adjusted < MinLUFS {
		return MinLUFS, fmt.Sprintf("%s (capped at %.0f LUFS)", description, MinLUFS)
	}

	return adjusted, description
}

func buildFFmpegArgs(inputFile, outputFile, filterChain string, numThreads int, options OutputOptions) []string {
	args := []string{
		"-threads", fmt.Sprintf("%d", numThreads),
		"-thread_queue_size", "512",
		"-i", inputFile,
		"-af", filterChain,
		"-threads", fmt.Sprintf("%d", numThreads),
		"-preset", "ultrafast",
		"-movflags", "+faststart",
		"-max_muxing_queue_size", "9999",
	}

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
	} else if outputExt == ".mp3" {
		args = append(args, "-b:a", "320k")
	}

	args = append(args, "-ar", "44100")

	if len(options.ExtraOptions) > 0 {
		args = append(args, options.ExtraOptions...)
	}

	args = append(args, "-y", outputFile)
	return args
}

func ValidateLUFS(lufs float64) error {
	if lufs < MinLUFS || lufs > MaxLUFS {
		return fmt.Errorf("LUFS value %.1f is outside valid range (%.1f to %.1f)", lufs, MinLUFS, MaxLUFS)
	}
	return nil
}

func GetPresetLUFS(preset string) (float64, error) {
	presetLower := strings.ToLower(preset)

	switch presetLower {
	case "default", "club", "dj":
		return DJMixLUFS, nil
	case "streaming", "spotify", "apple":
		return StreamingLUFS, nil
	case "podcast":
		return PodcastLUFS, nil
	case "broadcast", "radio":
		return BroadcastLUFS, nil
	default:
		return 0, fmt.Errorf("unknown preset: %s", preset)
	}
}
