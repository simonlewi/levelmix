package handlers

import (
	"fmt"
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

func getUploadLimit(tier int) int {
	switch tier {
	case 1:
		return -1 // Number of uploads per week for Free tier, unlimited for beta testing
	case 2:
		return 5 // Number of uploads per week for Premium tier
	case 3:
		return 20 // Number of uploads per week for Professional tier
	default:
		return -1 // Unlimited for beta testing
	}
}

func formatDuration(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, secs)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, secs)
	}
	return fmt.Sprintf("%ds", secs)
}
