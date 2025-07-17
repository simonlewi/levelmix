package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/simonlewi/levelmix/pkg/storage"
)

type DashboardHandler struct {
	metadata storage.MetadataStorage
}

func NewDashboardHandler(metadata storage.MetadataStorage) *DashboardHandler {
	return &DashboardHandler{
		metadata: metadata,
	}
}

func (h *DashboardHandler) ShowDashboard(c *gin.Context) {
	userInterface, exists := c.Get("user")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	user := userInterface.(*storage.User)

	// Get user stats
	stats, err := h.metadata.GetUserStats(c.Request.Context(), user.ID)
	if err != nil {
		// Create default stats if not found
		stats = &storage.UserUploadStats{
			UserID: user.ID,
		}
	}

	// Get recent jobs
	jobs, err := h.metadata.GetUserJobs(c.Request.Context(), user.ID, 10, 0)
	if err != nil {
		jobs = []*storage.ProcessingJob{}
	}

	// Get audio files for each job
	jobsWithFiles := make([]map[string]interface{}, 0)
	for _, job := range jobs {
		audioFile, err := h.metadata.GetAudioFileByJobID(c.Request.Context(), job.ID)
		jobData := map[string]interface{}{
			"job": job,
		}
		if err == nil {
			jobData["file"] = audioFile
		}
		jobsWithFiles = append(jobsWithFiles, jobData)
	}

	// Calculate tier info
	tierName := getTierName(user.SubscriptionTier)
	uploadLimit := getUploadLimit(user.SubscriptionTier)
	uploadsRemaining := uploadLimit
	if uploadLimit > 0 {
		uploadsRemaining = uploadLimit - stats.UploadsThisMonth
		if uploadsRemaining < 0 {
			uploadsRemaining = 0
		}
	}

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"CurrentPage":      "dashboard",
		"user":            user,
		"stats":           stats,
		"jobs":            jobsWithFiles,
		"tierName":        tierName,
		"uploadLimit":     uploadLimit,
		"uploadsRemaining": uploadsRemaining,
		"processingTime":  formatDuration(stats.TotalProcessingTimeSeconds),
	})
}

func (h *DashboardHandler) GetHistory(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get pagination params
	offset := 0
	limit := 20

	jobs, err := h.metadata.GetUserJobs(c.Request.Context(), userID.(string), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch history"})
		return
	}

	// Return partial HTML for HTMX
	c.HTML(http.StatusOK, "history_rows.html", gin.H{
		"jobs": jobs,
	})
}

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
		return 1
	case 2:
		return 4
	case 3:
		return -1 // Unlimited
	default:
		return 1
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