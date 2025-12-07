package audio

import (
	"fmt"
	"log"
	"strings"
)

// ProcessAudioWithMode processes audio using the specified mode
// This is the main entry point that routes to appropriate analysis method
func ProcessAudioWithMode(inputFile, outputFile string, targetLUFS float64, options OutputOptions, mode ProcessingMode, silenceInfo *SilenceInfo) error {
	var loudnessInfo *LoudnessInfo
	var err error

	switch mode {
	case ModeFast:
		// Fast mode: Use adaptive sampling for balanced speed/accuracy
		// Best for: podcasts, radio content, shorter mixes
		// Note: For DJ mixes with dramatic dynamics, Precise mode is recommended
		log.Printf("[INFO] Fast mode: Using adaptive sampling analysis")
		loudnessInfo, err = AnalyzeLoudnessAdaptiveSample(inputFile)
		if err != nil {
			return fmt.Errorf("adaptive analysis failed: %w", err)
		}

	case ModePrecise:
		// Precise mode: Full file analysis for maximum accuracy
		// Best for: DJ mixes, live sets, content with high dynamic range
		// Takes longer but captures true dynamics for optimal normalization
		log.Printf("[INFO] Precise mode: Using full file analysis")
		loudnessInfo, err = AnalyzeLoudness(inputFile)
		if err != nil {
			return fmt.Errorf("full analysis failed: %w", err)
		}

	default:
		return fmt.Errorf("unknown processing mode: %s", mode)
	}

	// Log the analysis results
	log.Printf("[INFO] Analysis complete: %.1f LUFS, LRA: %.1f, Peak: %.1f dB",
		loudnessInfo.InputI, loudnessInfo.InputLRA, loudnessInfo.InputTP)

	// Normalize using dynamics-aware single-pass processing
	// No segment cutting - preserves original audio structure perfectly
	return NormalizeLoudness(inputFile, outputFile, targetLUFS, loudnessInfo, options, silenceInfo)
}

// ValidateProcessingMode checks if the processing mode is valid and returns the canonical form
func ValidateProcessingMode(mode string) (ProcessingMode, error) {
	switch strings.ToLower(mode) {
	case "fast", "quick", "adaptive":
		return ModeFast, nil
	case "precise", "accurate", "full":
		return ModePrecise, nil
	default:
		return ModePrecise, fmt.Errorf("invalid processing mode: %s. Use 'fast' or 'precise'", mode)
	}
}

// RecommendProcessingMode suggests the optimal mode based on content type
// This can be used by the UI to guide users
func RecommendProcessingMode(contentType string) ProcessingMode {
	switch strings.ToLower(contentType) {
	case "dj", "djmix", "dj-mix", "mix", "liveset", "live-set":
		// DJ mixes benefit from precise analysis to capture break/drop dynamics
		return ModePrecise
	case "podcast", "speech", "voice", "interview":
		// Podcasts are typically consistent - fast mode is fine
		return ModeFast
	case "radio", "broadcast":
		// Radio content is usually pre-processed - fast mode works
		return ModeFast
	default:
		// When in doubt, precise is safer
		return ModePrecise
	}
}
