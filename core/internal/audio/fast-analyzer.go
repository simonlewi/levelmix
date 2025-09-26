// core/internal/audio/fast-analyzer.go
package audio

import (
	"fmt"
	"log"
	"strings"
)

// ProcessAudioWithMode processes audio using the specified mode
// This is the main entry point that routes to appropriate analysis method
func ProcessAudioWithMode(inputFile, outputFile string, targetLUFS float64, options OutputOptions, mode ProcessingMode) error {
	var loudnessInfo *LoudnessInfo
	var err error

	switch mode {
	case ModeFast:
		// Fast mode: Use adaptive sampling for balanced speed/accuracy
		log.Printf("Fast mode: Using adaptive sampling analysis")
		loudnessInfo, err = AnalyzeLoudnessAdaptiveSample(inputFile)
		if err != nil {
			return fmt.Errorf("adaptive analysis failed: %w", err)
		}

	case ModePrecise:
		// Precise mode: Full file analysis for maximum accuracy
		log.Printf("Precise mode: Using full file analysis")
		loudnessInfo, err = AnalyzeLoudness(inputFile)
		if err != nil {
			return fmt.Errorf("full analysis failed: %w", err)
		}

	default:
		return fmt.Errorf("unknown processing mode: %s", mode)
	}

	// Both modes use the same normalization with measured values
	return NormalizeLoudness(inputFile, outputFile, targetLUFS, loudnessInfo, options)
}

// ValidateProcessingMode checks if the processing mode is valid
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
