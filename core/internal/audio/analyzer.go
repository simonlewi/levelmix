package audio

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

type LoudnessInfo struct {
	InputI      float64 // Integrated LUFS
	InputTP     float64 // True Peak
	InputLRA    float64 // Loudness Range
	InputThresh float64 // Threshold
}

type loudnormOutput struct {
	InputI      string `json:"input_i"`
	InputTP     string `json:"input_tp"`
	InputLRA    string `json:"input_lra"`
	InputThresh string `json:"input_thresh"`
}

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

	var data loudnormOutput
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON data: %w", err)
	}

	// Parse string values to float64
	var info LoudnessInfo
	fmt.Sscanf(data.InputI, "%f", &info.InputI)
	fmt.Sscanf(data.InputTP, "%f", &info.InputTP)
	fmt.Sscanf(data.InputLRA, "%f", &info.InputLRA)
	fmt.Sscanf(data.InputThresh, "%f", &info.InputThresh)

	return &info, nil
}

func AnalyzeLoudness(inputFile string) (*LoudnessInfo, error) {
	log.Printf("Starting loudness analysis for file: %s", inputFile)

	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		log.Printf("ERROR: Input file does not exist: %s", inputFile)
		return nil, fmt.Errorf("input file does not exist: %s", inputFile)
	}

	cmd := exec.Command("ffmpeg",
		"-i", inputFile,
		"-af", "loudnorm=print_format=json:I=-16:TP=-1.5:LRA=10",
		"-f", "null", "-")

	log.Printf("FFmpeg command: %s", strings.Join(cmd.Args, " "))

	output, err := cmd.CombinedOutput()

	log.Printf("FFmpeg output: %s", string(output))

	if err != nil {
		log.Printf("FFmpeg error: %v", err)
		log.Printf("FFmpeg exit code: %s", err.Error())
		return nil, fmt.Errorf("loudness analysis failed: %w", err)
	}

	// Parse the JSON output from FFMPEG
	return parseLoudnormOutput(output)
}
