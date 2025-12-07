package audio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeLoudness(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "levelmix-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testCases := []struct {
		name       string
		inputFile  string
		targetLUFS float64
		options    OutputOptions
		wantError  bool
	}{
		{
			name:       "Basic normalization",
			inputFile:  "testdata/sample.wav",
			targetLUFS: -14.0,
			options: OutputOptions{
				Codec: "pcm_s16le",
			},
			wantError: false,
		},
		{
			name:       "MP3 output",
			inputFile:  "testdata/sample.wav",
			targetLUFS: -14.0,
			options: OutputOptions{
				Codec:   "libmp3lame",
				Bitrate: "320k",
			},
			wantError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := AnalyzeLoudness(tc.inputFile)
			if err != nil {
				t.Fatalf("Failed to analyze input file: %v", err)
			}

			outputFile := filepath.Join(tmpDir, "output.wav")

			err = NormalizeLoudness(tc.inputFile, outputFile, tc.targetLUFS, info, tc.options, &SilenceInfo{})
			if (err != nil) != tc.wantError {
				t.Errorf("NormalizeLoudness() error = %v, wantError %v", err, tc.wantError)
				return
			}

			if tc.wantError {
				return
			}

			if _, err := os.Stat(outputFile); os.IsNotExist(err) {
				t.Errorf("Output file was not created")
				return
			}

			outputInfo, err := AnalyzeLoudness(outputFile)
			if err != nil {
				t.Fatalf("Failed to analyze output file: %v", err)
			}

			if diff := abs(outputInfo.InputI - tc.targetLUFS); diff > 0.5 {
				t.Errorf("Output LUFS = %.2f, want %.2f (±0.5)", outputInfo.InputI, tc.targetLUFS)
			}
		})
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
