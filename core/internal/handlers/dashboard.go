package handlers

import (
	"log"
	"net/http"
	"time"

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
			UserID:                     user.ID,
			ProcessingTimeThisMonth:    0,
			MonthResetAt:               getMonthStart(time.Now()),
			TotalUploads:               0,
			TotalProcessingTimeSeconds: 0,
		}
		// Attempt to create the default stats in DB
		if err := h.metadata.UpdateUserStats(c.Request.Context(), stats); err != nil {
			log.Printf("Dashboard: Failed to create initial user stats for %s: %v", user.ID, err)
		}
	}

	// Ensure monthly stats are up-to-date before displaying
	now := time.Now()
	currentMonthStart := getMonthStart(now)

	if stats.MonthResetAt.Before(currentMonthStart) {
		log.Printf("Dashboard: Monthly reset triggered for user %s. Old reset: %v, New reset: %v",
			user.ID, stats.MonthResetAt, currentMonthStart)
		stats.ProcessingTimeThisMonth = 0
		stats.MonthResetAt = currentMonthStart
		// Update in DB if reset occurred
		if err := h.metadata.UpdateUserStats(c.Request.Context(), stats); err != nil {
			log.Printf("Dashboard: Failed to update user stats on monthly reset: %v", err)
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
		// Add dereferenced TargetLUFS for template
		if job.TargetLUFS != nil {
			jobData["targetLUFS"] = *job.TargetLUFS
			jobData["hasTargetLUFS"] = true
		}
		jobsWithFiles = append(jobsWithFiles, jobData)
	}

	// Calculate tier info
	tierName := getTierName(user.SubscriptionTier)
	processingTimeLimit := getProcessingTimeLimit(user.SubscriptionTier)

	// Calculate processing time remaining
	processingTimeRemaining := processingTimeLimit
	if processingTimeLimit > 0 {
		processingTimeRemaining = processingTimeLimit - stats.ProcessingTimeThisMonth
		if processingTimeRemaining < 0 {
			processingTimeRemaining = 0
		}
	}

	// Calculate processing time percentage for progress bar
	processingTimePercent := 0
	if processingTimeLimit > 0 {
		processingTimePercent = int((float64(stats.ProcessingTimeThisMonth) / float64(processingTimeLimit)) * 100)
		if processingTimePercent > 100 {
			processingTimePercent = 100
		}
	}

	c.HTML(http.StatusOK, "dashboard.html", GetTemplateData(c, gin.H{
		"CurrentPage":                    "dashboard",
		"PageTitle":                      "Dashboard",
		"user":                           user,
		"stats":                          stats,
		"jobs":                           jobsWithFiles,
		"tierName":                       tierName,
		"processingTimeLimit":            processingTimeLimit,
		"processingTimeRemaining":        formatDuration(processingTimeRemaining),
		"processingTimeUsed":             formatDuration(stats.ProcessingTimeThisMonth),
		"processingTimeTotal":            formatDurationAsHours(processingTimeLimit),
		"processingTime":                 formatDuration(stats.TotalProcessingTimeSeconds),
		"processingTimeRemainingSeconds": processingTimeRemaining,
		"processingTimePercent":          processingTimePercent,
	}))
}

// GetHistory returns the user's processing history (for HTMX pagination)
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
