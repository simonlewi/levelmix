package handlers

import (
	"fmt"
	"time"
)

func getTierName(tier int) string {
	switch tier {
	case 1:
		return "Free"
	case 2:
		return "Premium"
	case 3:
		return "Professional"
	default:
		return "Free"
	}
}

// getMonthStart returns the start of the month (1st day at 00:00:00)
func getMonthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// getProcessingTimeLimit returns the monthly processing time limit in seconds
// Returns -1 for unlimited
func getProcessingTimeLimit(tier int) int {
	switch tier {
	case 1:
		return 7200 // 2 hours per month for Free tier (7200 seconds after beta)
	case 2:
		return 36000 // 10 hours per month for Premium tier (€9/month)
	case 3:
		return 144000 // 40 hours per month for Professional tier (€24/month)
	default:
		return 7200 // Default to free tier (7200 seconds after beta)
	}
}

// formatDuration formats seconds into human-readable format
func formatDuration(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, secs)
	}
	return fmt.Sprintf("%ds", secs)
}

// formatDurationDecimal formats seconds into hours with decimal (e.g., "2.5h")
func formatDurationDecimal(seconds int) string {
	hours := float64(seconds) / 3600.0
	if hours >= 1.0 {
		return fmt.Sprintf("%.1fh", hours)
	}
	minutes := seconds / 60
	if minutes >= 1 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%ds", seconds)
}

// formatDurationAsHours formats seconds as hours only (e.g., "10h")
func formatDurationAsHours(seconds int) string {
	hours := seconds / 3600
	return fmt.Sprintf("%dh", hours)
}
