package audio

import (
	"testing"
)

func TestParseLoudnormOutput(t *testing.T) {
	sample := `
[Parsed_loudnorm_0 @ 0x7f8e1c] 
{
    "input_i" : "-16.54",
    "input_tp" : "-2.29",
    "input_lra" : "7.80",
    "input_thresh" : "-27.61"
}
`
	info, err := parseLoudnormOutput([]byte(sample))
	if err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if info.InputI != -16.54 {
		t.Errorf("Expected InputI to be -16.54, got %f", info.InputI)
	}
	if info.InputTP != -2.29 {
		t.Errorf("Expected InputTP to be -2.29, got %f", info.InputTP)
	}
	if info.InputLRA != 7.80 {
		t.Errorf("Expected InputLRA to be 7.80, got %f", info.InputLRA)
	}
	if info.InputThresh != -27.61 {
		t.Errorf("Expected InputThresh to be -27.61, got %f", info.InputThresh)
	}
}
