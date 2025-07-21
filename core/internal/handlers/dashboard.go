package handlers

import (
	"log"
	"net/http"
	"time" // Import time package for Weekday and Date functions

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
			// Initialize weekly stats for new users
			UploadsThisWeek: 0,
			WeekResetAt:     time.Now(), // Set to current time, will be adjusted by checkUploadLimits if needed
		}
		// Attempt to create the default stats in DB if they don't exist
		// This ensures WeekResetAt is persisted for new users
		if err := h.metadata.UpdateUserStats(c.Request.Context(), stats); err != nil {
			// Log error but proceed, as we have default stats in memory
			// Consider more robust error handling if this is critical
		}
	}

	// Ensure weekly stats are up-to-date before displaying
	// This logic is similar to checkUploadLimits but for display purposes
	now := time.Now()
	weekday := now.Weekday()
	if weekday == time.Sunday {
		weekday = 7 // Treat Sunday as the 7th day for consistent week start
	}
	daysSinceMonday := weekday - time.Monday
	currentWeekStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -int(daysSinceMonday))

	if stats.WeekResetAt.Before(currentWeekStart) {
		stats.UploadsThisWeek = 0
		stats.WeekResetAt = currentWeekStart
		// Update in DB if reset occurred
		if err := h.metadata.UpdateUserStats(c.Request.Context(), stats); err != nil {
			log.Printf("Failed to update user stats on weekly reset in DashboardHandler: %v", err)
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
		audioFile, err := h.metadata.GetAudioFile(c.Request.Context(), job.AudioFileID)
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

	// Calculate uploads remaining based on weekly stats
	uploadsRemaining := uploadLimit
	if uploadLimit > 0 { // Check if it's not an unlimited plan
		uploadsRemaining = uploadLimit - stats.UploadsThisWeek // Use UploadsThisWeek
		if uploadsRemaining < 0 {
			uploadsRemaining = 0
		}
	}

	templateData := gin.H{
		"CurrentPage":      "dashboard",
		"PageTitle":        "Dashboard",
		"user":             user,
		"stats":            stats, // stats now contains updated weekly counts
		"jobs":             jobsWithFiles,
		"tierName":         tierName,
		"uploadLimit":      uploadLimit,
		"uploadsRemaining": uploadsRemaining,
		"processingTime":   formatDuration(stats.TotalProcessingTimeSeconds),
		// No need to explicitly pass "uploadsThisWeek" as it's part of "stats"
	}

	// IMPORTANT: Use GetTemplateData to add common variables like IsLoggedIn
	templateData = GetTemplateData(c, templateData)

	c.HTML(http.StatusOK, "dashboard.html", templateData)
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
