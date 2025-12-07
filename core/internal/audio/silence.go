package audio

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type SilenceInfo struct {
	HasStartSilence bool
	HasEndSilence   bool
	TrimStart       float64 // Where audio actually begins (skip this much from start)
	TrimEnd         float64 // Where audio actually ends (stop here)
	TotalDuration   float64
}

// DetectSilence finds silence at start/end of audio and returns trim points
func DetectSilence(inputFile string) (*SilenceInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Get total duration first
	duration, err := getDurationForSilence(ctx, inputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get duration: %w", err)
	}

	// Run silence detection
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", inputFile,
		"-af", "silencedetect=noise=-70dB:d=0.5",
		"-f", "null", "-")

	output, err := cmd.CombinedOutput()
	if err != nil && ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("silence detection timed out")
	}
	// FFmpeg returns non-zero for null output, that's OK

	info := parseSilenceOutput(output, duration)

	if info.HasStartSilence || info.HasEndSilence {
		log.Printf("[INFO] Silence detected: trim %.2fs from start, %.2fs from end (%.1fs → %.1fs)",
			info.TrimStart,
			duration-info.TrimEnd,
			duration,
			info.ContentDuration())
	} else {
		log.Printf("[INFO] No significant silence detected at start/end")
	}

	return info, nil
}

func getDurationForSilence(ctx context.Context, inputFile string) (float64, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inputFile)

	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	duration, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0, err
	}

	return duration, nil
}

func parseSilenceOutput(output []byte, totalDuration float64) *SilenceInfo {
	info := &SilenceInfo{
		TotalDuration: totalDuration,
		TrimStart:     0,             // Default: start from beginning
		TrimEnd:       totalDuration, // Default: go to end
	}

	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")

	// Track all silence periods as (start, end) pairs
	type silencePeriod struct {
		start float64
		end   float64
	}
	var periods []silencePeriod
	var pendingStart float64 = -1

	for _, line := range lines {
		// Parse silence_start
		if idx := strings.Index(line, "silence_start:"); idx != -1 {
			valueStr := strings.TrimSpace(line[idx+len("silence_start:"):])
			// Handle any trailing content
			if spaceIdx := strings.IndexAny(valueStr, " \t\n"); spaceIdx != -1 {
				valueStr = valueStr[:spaceIdx]
			}
			if start, err := strconv.ParseFloat(valueStr, 64); err == nil {
				pendingStart = start
			}
		}

		// Parse silence_end
		if idx := strings.Index(line, "silence_end:"); idx != -1 {
			valueStr := strings.TrimSpace(line[idx+len("silence_end:"):])
			// Split on | to get just the end time (format: "silence_end: 2.5 | silence_duration: 2.5")
			if pipeIdx := strings.Index(valueStr, "|"); pipeIdx != -1 {
				valueStr = strings.TrimSpace(valueStr[:pipeIdx])
			}
			if end, err := strconv.ParseFloat(valueStr, 64); err == nil {
				if pendingStart >= 0 {
					periods = append(periods, silencePeriod{start: pendingStart, end: end})
					pendingStart = -1
				}
			}
		}
	}

	// If there's a silence_start with no corresponding end, it extends to EOF
	if pendingStart >= 0 {
		periods = append(periods, silencePeriod{start: pendingStart, end: totalDuration})
	}

	if debugMode {
		log.Printf("[DEBUG] Found %d silence periods in %.1fs file", len(periods), totalDuration)
		for i, p := range periods {
			log.Printf("[DEBUG]   Period %d: %.2fs - %.2fs (duration: %.2fs)", i+1, p.start, p.end, p.end-p.start)
		}
	}

	// Analyze periods to find start/end silence
	for _, p := range periods {
		// Opening silence: starts at or very near 0
		if p.start < 0.1 {
			info.HasStartSilence = true
			info.TrimStart = p.end
		}

		// Trailing silence: extends to or very near the end of the file
		if p.end >= totalDuration-0.1 {
			info.HasEndSilence = true
			info.TrimEnd = p.start
		}
	}

	// Safety: ensure we have valid trim points
	if info.TrimStart >= info.TrimEnd {
		log.Printf("[WARN] Invalid trim points (start=%.2f >= end=%.2f), disabling trim",
			info.TrimStart, info.TrimEnd)
		info.TrimStart = 0
		info.TrimEnd = totalDuration
		info.HasStartSilence = false
		info.HasEndSilence = false
	}

	return info
}

// NeedsTrimming returns true if any silence should be removed
func (s *SilenceInfo) NeedsTrimming() bool {
	if s == nil {
		return false
	}
	return s.HasStartSilence || s.HasEndSilence
}

// TrimFilter returns the atrim filter string, or empty if no trimming needed
func (s *SilenceInfo) TrimFilter() string {
	if !s.NeedsTrimming() {
		return ""
	}

	// Add small buffer (50ms) to avoid cutting into audio
	start := s.TrimStart
	end := s.TrimEnd

	if s.HasStartSilence && start > 0.05 {
		start -= 0.05
	}
	if s.HasEndSilence && end < s.TotalDuration-0.05 {
		end += 0.05
	}

	return fmt.Sprintf("atrim=start=%.3f:end=%.3f,asetpts=PTS-STARTPTS", start, end)
}

// ContentDuration returns duration after trimming
func (s *SilenceInfo) ContentDuration() float64 {
	if s == nil {
		return 0
	}
	return s.TrimEnd - s.TrimStart
}
