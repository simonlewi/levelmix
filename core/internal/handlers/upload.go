package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/simonlewi/levelmix/core/internal/audio"
	"github.com/simonlewi/levelmix/pkg/storage"
)

type UploadHandler struct {
	storage     storage.AudioStorage
	metadata    storage.MetadataStorage
	queue       *audio.QueueManager
	redisClient *redis.Client
}

func NewUploadHandler(s storage.AudioStorage, m storage.MetadataStorage, q *audio.QueueManager, redisURL string) *UploadHandler {
	var redisClient *redis.Client
	if redisURL != "" {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     redisURL,
			Password: os.Getenv("REDIS_PASSWORD"),
		})
	}
	return &UploadHandler{
		storage:     s,
		metadata:    m,
		queue:       q,
		redisClient: redisClient,
	}
}

func (h *UploadHandler) GetPresignedUploadURL(c *gin.Context) {
	// Get authenticated user
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required. Please log in to upload files."})
		return
	}

	currentUser, ok := userInterface.(*storage.User)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user session. Please log in again."})
		return
	}

	// Get file info from query params
	filename := c.Query("filename")
	filesizeStr := c.Query("filesize")

	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Filename is required"})
		return
	}

	if filesizeStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File size is required"})
		return
	}

	filesize, err := strconv.ParseInt(filesizeStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file size"})
		return
	}

	log.Printf("GetPresignedUploadURL: User %s requesting presigned URL for %s (%d bytes)", currentUser.ID, filename, filesize)

	// Create a mock FileHeader for validation using existing validateFile logic
	mockHeader := &multipart.FileHeader{
		Filename: filename,
		Size:     filesize,
	}

	// Validate file using your existing validation logic
	if err := h.validateFile(mockHeader, currentUser.SubscriptionTier); err != nil {
		log.Printf("GetPresignedUploadURL: File validation failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check upload limits using your existing logic
	if err := h.checkUploadLimits(c, currentUser); err != nil {
		log.Printf("GetPresignedUploadURL: Upload limit check failed: %v", err)
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	// Determine file format
	fileFormat := getFileExtension(filename)
	if fileFormat == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Could not determine audio format from file extension"})
		return
	}

	// Generate unique file ID
	fileID := generateID()
	key := h.storage.GetUploadKey(fileID, fileFormat)
	contentType := getContentTypeFromFormat(fileFormat)

	uploadURL, err := h.storage.GetPresignedUploadURL(c.Request.Context(), key, contentType, 15*time.Minute)
	if err != nil {
		log.Printf("GetPresignedUploadURL: Failed to generate presigned URL: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate upload URL"})
		return
	}

	log.Printf("GetPresignedUploadURL: Generated presigned URL for file %s (key: %s)", fileID, key)

	c.JSON(http.StatusOK, gin.H{
		"upload_url":   uploadURL,
		"file_id":      fileID,
		"method":       "PUT",
		"content_type": contentType,
	})
}

// ConfirmUpload confirms that a file has been uploaded to S3 and starts processing
func (h *UploadHandler) ConfirmUpload(c *gin.Context) {
	fileID := c.PostForm("file_id")
	originalFilename := c.PostForm("filename")

	if fileID == "" || originalFilename == "" {
		h.returnError(c, "Missing file ID or filename")
		return
	}

	// Get authenticated user
	userInterface, exists := c.Get("user")
	if !exists {
		h.returnError(c, "Authentication required. Please log in to upload files.")
		return
	}

	currentUser, ok := userInterface.(*storage.User)
	if !ok {
		h.returnError(c, "Invalid user session. Please log in again.")
		return
	}

	userIDFromContext := currentUser.ID
	userTier := currentUser.SubscriptionTier
	isPremium := currentUser.SubscriptionTier > 1

	log.Printf("ConfirmUpload: User %s confirming upload for file %s", userIDFromContext, fileID)

	// Get target LUFS
	targetLUFS, err := h.parseTargetLUFS(c.PostForm("target_lufs"))
	if err != nil {
		log.Printf("ConfirmUpload: LUFS parsing failed: %v", err)
		h.returnError(c, err.Error())
		return
	}

	// Get processing mode
	processingModeStr := c.PostForm("processing_mode")
	if processingModeStr == "" {
		processingModeStr = "fast"
	}

	processingMode, err := audio.ValidateProcessingMode(processingModeStr)
	if err != nil {
		log.Printf("ConfirmUpload: Invalid processing mode: %v", err)
		h.returnError(c, "Invalid processing mode selected")
		return
	}

	log.Printf("ConfirmUpload: Processing mode selected: %s", processingMode)

	// Validate custom LUFS usage - only Premium/Pro users can use custom values
	if h.isCustomLUFS(targetLUFS) && userTier < 2 {
		log.Printf("ConfirmUpload: Custom LUFS attempted by non-premium user (Tier: %d)", userTier)
		h.returnError(c, "Custom LUFS targets are only available for Premium and Professional users")
		return
	}

	// Verify file actually exists in S3
	fileFormat := getFileExtension(originalFilename)
	if fileFormat == "" {
		h.returnError(c, "Could not determine audio format from file extension")
		return
	}

	key := h.storage.GetUploadKey(fileID, fileFormat)

	info, err := h.storage.GetObjectInfo(c.Request.Context(), key)
	if err != nil {
		log.Printf("ConfirmUpload: Failed to verify S3 upload for %s: %v", fileID, err)
		h.returnError(c, "Upload verification failed. The file was not found in storage. If you selected this file from cloud storage (Google Drive, Dropbox, OneDrive, etc.), please download it to your device first and try again.")
		return
	}

	log.Printf("ConfirmUpload: Verified S3 upload: %s (%d bytes)", fileID, info.Size)

	// Store metadata
	audioFile := &storage.AudioFile{
		ID:               fileID,
		UserID:           &userIDFromContext,
		OriginalFilename: originalFilename,
		FileSize:         info.Size,
		Format:           fileFormat,
		Status:           "uploaded",
		LUFSTarget:       targetLUFS,
		CreatedAt:        time.Now(),
	}

	if err := h.metadata.CreateAudioFile(c.Request.Context(), audioFile); err != nil {
		log.Printf("ConfirmUpload: Failed to save audio file metadata for %s: %v", fileID, err)
		// Clean up S3 file since metadata creation failed
		h.storage.Delete(c.Request.Context(), key)
		h.returnError(c, "Failed to save file metadata")
		return
	}

	// Prepare job for queueing
	jobID := generateID()

	job := &storage.ProcessingJob{
		ID:          jobID,
		AudioFileID: fileID,
		UserID:      userIDFromContext,
		Status:      "queued",
		TargetLUFS:  &targetLUFS,
		CreatedAt:   time.Now(),
	}

	log.Printf("ConfirmUpload: Creating job record for file %s with jobID %s and UserID '%s'", fileID, jobID, job.UserID)

	if err := h.metadata.CreateJob(c.Request.Context(), job); err != nil {
		log.Printf("ConfirmUpload: Failed to create job record for %s: %v", fileID, err)
		h.cleanup(c, fileID, fileFormat)
		h.returnError(c, "Failed to create processing job")
		return
	}

	task := audio.ProcessTask{
		JobID:          jobID,
		FileID:         fileID,
		TargetLUFS:     targetLUFS,
		UserID:         userIDFromContext,
		IsPremium:      isPremium,
		ProcessingMode: processingMode,
	}

	log.Printf("ConfirmUpload: Enqueueing processing task for job %s", jobID)
	if err := h.queue.EnqueueProcessing(c.Request.Context(), task); err != nil {
		log.Printf("ConfirmUpload: Failed to queue processing task for job %s: %v", jobID, err)
		h.cleanup(c, fileID, fileFormat)
		h.returnError(c, "Failed to queue processing")
		return
	}

	// Increment upload stats (same logic as HandleUpload)
	stats, err := h.metadata.GetUserStats(c.Request.Context(), currentUser.ID)
	if err != nil {
		log.Printf("ConfirmUpload: Could not retrieve user stats for %s, creating new: %v", currentUser.ID, err)
		stats = &storage.UserUploadStats{
			UserID:                     currentUser.ID,
			UploadsThisWeek:            0,
			WeekResetAt:                time.Now(),
			TotalUploads:               0,
			TotalProcessingTimeSeconds: 0,
			ProcessingTimeThisMonth:    0,
			MonthResetAt:               getMonthStart(time.Now()),
		}
		if createErr := h.metadata.CreateUserStats(c.Request.Context(), stats); createErr != nil {
			log.Printf("ConfirmUpload: Failed to create initial user stats for %s: %v", currentUser.ID, createErr)
		}
	}

	stats.UploadsThisWeek++
	stats.TotalUploads++

	now := time.Now()
	weekday := now.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	daysSinceMonday := weekday - time.Monday
	currentWeekStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -int(daysSinceMonday))

	if stats.WeekResetAt.Before(currentWeekStart) {
		log.Printf("ConfirmUpload: Weekly reset triggered for user %s. Old reset: %v, New reset: %v", currentUser.ID, stats.WeekResetAt, currentWeekStart)
		stats.UploadsThisWeek = 0
		stats.WeekResetAt = currentWeekStart
		if err := h.metadata.UpdateUserStats(c.Request.Context(), stats); err != nil {
			log.Printf("ConfirmUpload: Failed to update user stats on weekly reset for %s: %v", currentUser.ID, err)
		}
	}

	if err := h.metadata.UpdateUserStats(c.Request.Context(), stats); err != nil {
		log.Printf("ConfirmUpload: Failed to update user stats with processing time for user %s: %v", currentUser.ID, err)
	} else {
		log.Printf("ConfirmUpload: User stats updated for %s. UploadsThisWeek: %d, TotalUploads: %d", currentUser.ID, stats.UploadsThisWeek, stats.TotalUploads)
	}

	// Return processing state HTML
	processingHTML := h.generateProcessingHTML(fileID, jobID)
	c.Data(http.StatusOK, "text/html", []byte(processingHTML))
}

func (h *UploadHandler) HandleUpload(c *gin.Context) {
	// Get uploaded file
	fileHeader, err := c.FormFile("audio_file")
	if err != nil {
		h.returnError(c, "No file uploaded")
		return
	}

	// Get authenticated user (now required)
	userInterface, exists := c.Get("user")
	if !exists {
		h.returnError(c, "Authentication required. Please log in to upload files.")
		return
	}

	currentUser, ok := userInterface.(*storage.User)
	if !ok {
		h.returnError(c, "Invalid user session. Please log in again.")
		return
	}

	userIDFromContext := currentUser.ID
	userTier := currentUser.SubscriptionTier
	isPremium := currentUser.SubscriptionTier > 1
	log.Printf("UploadHandler: User ID from context: '%s' (Tier: %d)", userIDFromContext, currentUser.SubscriptionTier)

	// Check upload limits
	if err := h.checkUploadLimits(c, currentUser); err != nil {
		log.Printf("UploadHandler: Upload limit check failed for user %s: %v", userIDFromContext, err)
		h.returnError(c, err.Error())
		return
	}

	// Validate file with tier-based rules (size, format)
	if err := h.validateFile(fileHeader, userTier); err != nil {
		log.Printf("UploadHandler: File validation failed: %v", err)
		h.returnError(c, err.Error())
		return
	}

	// Determine file format from extension for storage
	fileFormat := getFileExtension(fileHeader.Filename)
	if fileFormat == "" {
		h.returnError(c, "Could not determine audio format from file extension.")
		return
	}

	// Generate unique file ID
	fileID := generateID()

	// Get target LUFS
	targetLUFS, err := h.parseTargetLUFS(c.PostForm("target_lufs"))
	if err != nil {
		log.Printf("UploadHandler: LUFS parsing failed: %v", err)
		h.returnError(c, err.Error())
		return
	}

	// Get processing mode (new feature)
	processingModeStr := c.PostForm("processing_mode")
	if processingModeStr == "" {
		processingModeStr = "fast" // Default to fast mode
	}

	processingMode, err := audio.ValidateProcessingMode(processingModeStr)
	if err != nil {
		log.Printf("UploadHandler: Invalid processing mode: %v", err)
		h.returnError(c, "Invalid processing mode selected")
		return
	}

	log.Printf("UploadHandler: Processing mode selected: %s", processingMode)

	// Validate custom LUFS usage - only Premium/Pro users can use custom values
	if h.isCustomLUFS(targetLUFS) && userTier < 2 {
		log.Printf("UploadHandler: Custom LUFS attempted by non-premium user (Tier: %d)", userTier)
		h.returnError(c, "Custom LUFS targets are only available for Premium and Professional users")
		return
	}

	// Open uploaded file
	file, err := fileHeader.Open()
	if err != nil {
		log.Printf("UploadHandler: Failed to open file header: %v", err)
		h.returnError(c, "Failed to process file")
		return
	}
	defer file.Close()

	// Upload to S3
	log.Printf("UploadHandler: Attempting to upload file %s (%s) to S3", fileID, fileFormat)
	if err := h.storage.Upload(c.Request.Context(), fileID, file, fileFormat); err != nil {
		log.Printf("UploadHandler: S3 upload failed for file %s: %v", fileID, err)
		// Clean up metadata if S3 upload fails
		h.metadata.DeleteAudioFile(c.Request.Context(), fileID)
		// Need to pass full key to delete from S3
		h.storage.Delete(c.Request.Context(), h.storage.GetUploadKey(fileID, fileFormat))
		// Assuming DeleteJob exists and takes jobID
		// h.metadata.DeleteJob(c.Request.Context(), jobID)
		h.returnError(c, "Failed to store file")
		return
	}

	// Store metadata
	audioFile := &storage.AudioFile{
		ID:               fileID,
		UserID:           &userIDFromContext,
		OriginalFilename: fileHeader.Filename,
		FileSize:         fileHeader.Size,
		Format:           fileFormat, // Use the determined fileFormat
		Status:           "uploaded",
		LUFSTarget:       targetLUFS,
		CreatedAt:        time.Now(),
	}

	if err := h.metadata.CreateAudioFile(c.Request.Context(), audioFile); err != nil {
		log.Printf("UploadHandler: Failed to save audio file metadata for %s: %v", fileID, err)
		// Ensure cleanup uses the correct key with format
		h.storage.Delete(c.Request.Context(), h.storage.GetUploadKey(fileID, fileFormat))
		h.returnError(c, "Failed to save file metadata")
		return
	}

	// Prepare job for queueing
	jobID := generateID()

	job := &storage.ProcessingJob{
		ID:          jobID,
		AudioFileID: fileID,
		UserID:      userIDFromContext,
		Status:      "queued",
		TargetLUFS:  &targetLUFS,
		CreatedAt:   time.Now(),
	}

	log.Printf("UploadHandler: Creating job record for file %s with jobID %s and UserID '%s'", fileID, jobID, job.UserID)

	if err := h.metadata.CreateJob(c.Request.Context(), job); err != nil {
		log.Printf("UploadHandler: Failed to create job record for %s: %v", fileID, err)
		h.cleanup(c, fileID, fileFormat) // Pass fileFormat to cleanup
		h.returnError(c, "Failed to create processing job")
		return
	}

	task := audio.ProcessTask{
		JobID:          jobID,
		FileID:         fileID,
		TargetLUFS:     targetLUFS,
		UserID:         userIDFromContext,
		IsPremium:      isPremium,
		ProcessingMode: processingMode,
	}

	log.Printf("UploadHandler: Enqueueing processing task for job %s", jobID)
	if err := h.queue.EnqueueProcessing(c.Request.Context(), task); err != nil {
		log.Printf("UploadHandler: Failed to queue processing task for job %s: %v", jobID, err)
		h.cleanup(c, fileID, fileFormat) // Pass fileFormat to cleanup
		h.returnError(c, "Failed to queue processing")
		return
	}

	// Increment upload stats
	stats, err := h.metadata.GetUserStats(c.Request.Context(), currentUser.ID)
	if err != nil {
		log.Printf("UploadHandler: Could not retrieve user stats for %s, creating new: %v", currentUser.ID, err)
		stats = &storage.UserUploadStats{
			UserID:                     currentUser.ID,
			UploadsThisWeek:            0,
			WeekResetAt:                time.Now(),
			TotalUploads:               0,
			TotalProcessingTimeSeconds: 0,
		}
		if createErr := h.metadata.CreateUserStats(c.Request.Context(), stats); createErr != nil {
			log.Printf("UploadHandler: Failed to create initial user stats for %s: %v", currentUser.ID, createErr)
		}
	}

	stats.UploadsThisWeek++
	stats.TotalUploads++

	now := time.Now()
	weekday := now.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	daysSinceMonday := weekday - time.Monday
	currentWeekStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -int(daysSinceMonday))

	if stats.WeekResetAt.Before(currentWeekStart) {
		log.Printf("checkUploadLimits: Weekly reset triggered for user %s. Old reset: %v, New reset: %v", currentUser.ID, stats.WeekResetAt, currentWeekStart)
		stats.UploadsThisWeek = 0
		stats.WeekResetAt = currentWeekStart
		if err := h.metadata.UpdateUserStats(c.Request.Context(), stats); err != nil {
			log.Printf("checkUploadLimits: Failed to update user stats on weekly reset for %s: %v", currentUser.ID, err)
		}
	}

	if err := h.metadata.UpdateUserStats(c.Request.Context(), stats); err != nil {
		log.Printf("UploadHandler: Failed to update user stats with processing time for user %s: %v", currentUser.ID, err)
	} else {
		log.Printf("UploadHandler: User stats updated for %s. UploadsThisWeek: %d, TotalUploads: %d", currentUser.ID, stats.UploadsThisWeek, stats.TotalUploads)
	}

	// Return processing state HTML
	processingHTML := h.generateProcessingHTML(fileID, jobID)
	c.Data(http.StatusOK, "text/html", []byte(processingHTML))
}

func (h *UploadHandler) GetStatus(c *gin.Context) {
	fileID := c.Param("id")
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File ID required"})
		return
	}

	// First check Redis for real-time progress
	if h.redisClient != nil {
		key := fmt.Sprintf("progress:%s", fileID)
		result := h.redisClient.HGetAll(c.Request.Context(), key)

		if data, err := result.Result(); err == nil && len(data) > 0 {
			progress := 0
			status := "queued"
			silenceTrimmed := false

			if p, exists := data["progress"]; exists {
				if parsed, err := strconv.Atoi(p); err == nil {
					progress = parsed
				}
			}
			if s, exists := data["status"]; exists {
				status = s
			}
			if st, exists := data["silence_trimmed"]; exists && st == "true" {
				silenceTrimmed = true
			}

			// Also fetch file metadata from database to include duration, format, etc.
			response := gin.H{
				"status":         status,
				"progress":       progress,
				"fileID":         fileID,
				"silenceTrimmed": silenceTrimmed,
			}

			// Get audio file metadata
			if audioFile, err := h.metadata.GetAudioFile(c.Request.Context(), fileID); err == nil {
				response["filename"] = audioFile.OriginalFilename
				response["format"] = audioFile.Format
				response["fileSize"] = audioFile.FileSize
				if audioFile.DurationSeconds != nil {
					response["durationSeconds"] = *audioFile.DurationSeconds
				}
			}

			c.JSON(http.StatusOK, response)
			return
		}
	}

	// Fallback to database (existing logic)
	audioFile, err := h.metadata.GetAudioFile(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	job, err := h.metadata.GetJobByFileID(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":   audioFile.Status,
			"progress": 0,
			"fileID":   fileID,
		})
		return
	}

	progress := getProgressFromStatus(job.Status)
	response := gin.H{
		"status":   job.Status,
		"progress": progress,
		"fileID":   fileID,
		"filename": audioFile.OriginalFilename,
		"format":   audioFile.Format,
		"fileSize": audioFile.FileSize,
	}

	// Add duration if available
	if audioFile.DurationSeconds != nil {
		response["durationSeconds"] = *audioFile.DurationSeconds
	}

	if job.Status == "failed" && job.ErrorMessage != nil {
		response["error"] = *job.ErrorMessage
	}

	c.JSON(http.StatusOK, response)
}

func (h *UploadHandler) RetryJob(c *gin.Context) {
	fileID := c.Param("id")
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File ID required"})
		return
	}

	// Get the job to verify it exists and get its status
	job, err := h.metadata.GetJobByFileID(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	// Only allow retrying failed jobs
	if job.Status != "failed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Can only retry failed jobs"})
		return
	}

	// Get user to check premium status
	user, err := h.metadata.GetUser(c.Request.Context(), job.UserID)
	if err != nil {
		log.Printf("RetryJob: Failed to get user %s: %v", job.UserID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user information"})
		return
	}

	// Determine if user is premium (tier 2 = Premium, tier 3 = Professional)
	isPremium := user.SubscriptionTier >= 2

	// Default to precise mode for retry (we don't store this in the job)
	processingMode := audio.ModePrecise

	// Get target LUFS (use default if not set)
	targetLUFS := audio.DefaultLUFS
	if job.TargetLUFS != nil {
		targetLUFS = *job.TargetLUFS
	}

	// Create a new processing task
	task := audio.ProcessTask{
		JobID:          job.ID,
		FileID:         fileID,
		UserID:         job.UserID,
		TargetLUFS:     targetLUFS,
		IsPremium:      isPremium,
		ProcessingMode: processingMode,
	}

	log.Printf("RetryJob: Re-enqueueing processing task for job %s (file %s)", job.ID, fileID)
	if err := h.queue.EnqueueProcessing(c.Request.Context(), task); err != nil {
		log.Printf("RetryJob: Failed to re-queue processing task for job %s: %v", job.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retry job"})
		return
	}

	// Update job status back to queued
	job.Status = "queued"
	job.ErrorMessage = nil
	job.CompletedAt = nil

	if err := h.metadata.UpdateJob(c.Request.Context(), job); err != nil {
		log.Printf("RetryJob: Failed to update job status to queued for %s: %v", job.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update job status"})
		return
	}

	// Update audio file status
	if err := h.metadata.UpdateStatus(c.Request.Context(), fileID, "queued"); err != nil {
		log.Printf("RetryJob: Failed to update file status to queued for %s: %v", fileID, err)
	}

	log.Printf("Job %s successfully re-queued for retry", job.ID)

	c.JSON(http.StatusOK, gin.H{
		"status":  "queued",
		"message": "Job queued for retry successfully",
	})
}

func (h *UploadHandler) CancelJob(c *gin.Context) {
	fileID := c.Param("id")
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File ID required"})
		return
	}

	// Get the job to verify it exists and get its status
	job, err := h.metadata.GetJobByFileID(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	// Don't allow cancelling completed or already failed jobs
	if job.Status == "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot cancel completed job"})
		return
	}

	if job.Status == "failed" || job.Status == "cancelled" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Job already terminated"})
		return
	}

	// Mark job as cancelled in Redis (for in-progress jobs)
	if h.redisClient != nil {
		cancelKey := fmt.Sprintf("cancel:%s", fileID)
		if err := h.redisClient.Set(c.Request.Context(), cancelKey, "true", 10*time.Minute).Err(); err != nil {
			log.Printf("Failed to set cancel flag in Redis for job %s: %v", fileID, err)
		}

		// Update progress to indicate cancellation
		progressKey := fmt.Sprintf("progress:%s", fileID)
		h.redisClient.HMSet(c.Request.Context(), progressKey, map[string]interface{}{
			"status":     "cancelled",
			"progress":   0,
			"updated_at": time.Now().Unix(),
		})
	}

	// Update job status in database
	cancelledMsg := "Job cancelled by user"
	job.Status = "cancelled"
	job.ErrorMessage = &cancelledMsg
	now := time.Now()
	job.CompletedAt = &now

	if err := h.metadata.UpdateJob(c.Request.Context(), job); err != nil {
		log.Printf("Failed to update job status to cancelled for %s: %v", fileID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel job"})
		return
	}

	// Update audio file status
	if err := h.metadata.UpdateStatus(c.Request.Context(), fileID, "cancelled"); err != nil {
		log.Printf("Failed to update file status to cancelled for %s: %v", fileID, err)
	}

	// Get audio file to determine format for cleanup
	audioFile, err := h.metadata.GetAudioFile(c.Request.Context(), fileID)
	if err == nil {
		// Clean up uploaded file from S3
		h.cleanup(c, fileID, audioFile.Format)
	}

	log.Printf("Job %s cancelled by user", job.ID)

	c.JSON(http.StatusOK, gin.H{
		"status":  "cancelled",
		"message": "Job cancelled successfully",
	})
}

func (h *UploadHandler) checkUploadLimits(c *gin.Context, user *storage.User) error {
	stats, err := h.metadata.GetUserStats(c.Request.Context(), user.ID)
	if err != nil {
		log.Printf("checkUploadLimits: No stats found for user %s, initializing default. Error: %v", user.ID, err)
		stats = &storage.UserUploadStats{
			UserID:                     user.ID,
			ProcessingTimeThisMonth:    0,
			MonthResetAt:               getMonthStart(time.Now()),
			TotalUploads:               0,
			TotalProcessingTimeSeconds: 0,
			// Keep old fields during transition
			UploadsThisWeek: 0,
			WeekResetAt:     time.Now(),
		}
		if createErr := h.metadata.CreateUserStats(c.Request.Context(), stats); createErr != nil {
			log.Printf("checkUploadLimits: Failed to create initial user stats for %s: %v", user.ID, createErr)
		}
	}

	// Use NEW monthly processing time limit
	processingTimeLimit := getProcessingTimeLimit(user.SubscriptionTier)
	if processingTimeLimit == -1 {
		log.Printf("checkUploadLimits: User %s has unlimited processing time.", user.ID)
		return nil
	}

	// Check for MONTHLY reset (not weekly)
	now := time.Now()
	currentMonthStart := getMonthStart(now)

	if stats.MonthResetAt.Before(currentMonthStart) {
		log.Printf("checkUploadLimits: Monthly reset triggered for user %s. Old reset: %v, New reset: %v", user.ID, stats.MonthResetAt, currentMonthStart)
		stats.ProcessingTimeThisMonth = 0
		stats.MonthResetAt = currentMonthStart
		if err := h.metadata.UpdateUserStats(c.Request.Context(), stats); err != nil {
			log.Printf("checkUploadLimits: Failed to update user stats on monthly reset for %s: %v", user.ID, err)
		}
	}

	// Check if user has reached monthly processing time limit
	if stats.ProcessingTimeThisMonth >= processingTimeLimit {
		usedTime := formatDurationDecimal(stats.ProcessingTimeThisMonth)
		limitTime := formatDurationDecimal(processingTimeLimit)
		log.Printf("checkUploadLimits: User %s reached monthly processing time limit (%d/%d seconds).", user.ID, stats.ProcessingTimeThisMonth, processingTimeLimit)
		return fmt.Errorf("You've reached your monthly processing time limit (%s / %s used). Resets on the 1st of next month. Upgrade your plan for more time.", usedTime, limitTime)
	}

	log.Printf("checkUploadLimits: User %s is within limits (%d/%d seconds).", user.ID, stats.ProcessingTimeThisMonth, processingTimeLimit)
	return nil
}

// returnError sends an inline error response that the frontend can handle
func (h *UploadHandler) returnError(c *gin.Context, message string) {
	errorHTML := fmt.Sprintf(`
		<div id="error-state" class="state-transition">
			<div class="text-center">
				<div class="w-16 h-16 bg-red-500 rounded-full flex items-center justify-center mx-auto mb-6">
					<svg class="w-8 h-8 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
					</svg>
				</div>
				<h2 class="text-3xl font-bold mb-4 text-red-400">Upload Failed</h2>
				<p class="text-gray-300 mb-8">%s</p>
				<button onclick="uploadAnother()"
						class="w-full bg-cyan-400 text-gray-900 px-6 py-3 rounded-lg font-bold hover:bg-cyan-300 transition-colors">
					Try Again
				</button>
			</div>
		</div>`, html.EscapeString(message))

	c.Data(http.StatusOK, "text/html", []byte(errorHTML))
}

// generateProcessingHTML creates the processing state HTML
func (h *UploadHandler) generateProcessingHTML(fileID, jobID string) string {
	return fmt.Sprintf(`
		<div id="processing-state" class="state-transition" data-file-id="%s" data-job-id="%s">
			<div class="text-center">
				<div class="spinner mx-auto mb-6"></div>
				<h2 class="text-3xl font-bold mb-4">Processing Your Audio</h2>
				<p id="status-text" class="text-gray-300 mb-8">Queued for processing...</p>
				
				<div class="bg-gray-800 rounded-lg p-6 mb-6">
					<div class="flex justify-between items-center mb-2">
						<span class="text-sm text-gray-400">Progress</span>
						<span id="progress-text" class="text-sm text-cyan-400">0%%</span>
					</div>
					<div class="w-full bg-gray-700 rounded-full h-2">
						<div id="progress-bar" class="progress-bar h-2 rounded-full" style="width: 0%%"></div>
					</div>
				</div>
				
				<p class="text-gray-400 text-sm">This usually takes 1-2 minutes depending on file size</p>
			</div>
		</div>`, fileID, jobID)
}

// parseTargetLUFS parses and validates the target LUFS value
func (h *UploadHandler) parseTargetLUFS(lufsStr string) (float64, error) {
	if lufsStr == "" {
		return audio.DefaultLUFS, nil
	}

	// Handle "custom" value - this shouldn't happen with the new frontend,
	// but keeping for backwards compatibility
	if lufsStr == "custom" {
		return audio.DefaultLUFS, nil
	}

	parsed, err := strconv.ParseFloat(lufsStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid LUFS value: %s", lufsStr)
	}

	// Validate LUFS range (expanded range for custom values)
	if parsed < -30 || parsed > -2 {
		return 0, fmt.Errorf("LUFS target must be between -30 and -2, got %.1f", parsed)
	}

	return parsed, nil
}

// cleanup removes uploaded file, processed file, and metadata on error
func (h *UploadHandler) cleanup(ctx *gin.Context, fileID string, fileFormat string) {
	// Try to delete the uploaded file (ignore errors)
	uploadKey := h.storage.GetUploadKey(fileID, fileFormat)
	if err := h.storage.Delete(ctx.Request.Context(), uploadKey); err != nil {
		log.Printf("cleanup: Failed to delete uploaded file %s: %v", uploadKey, err)
	}

	// Try to delete the processed file if it exists (ignore errors)
	processedKey := h.storage.GetProcessedKey(fileID, fileFormat)
	if err := h.storage.Delete(ctx.Request.Context(), processedKey); err != nil {
		log.Printf("cleanup: Failed to delete processed file %s: %v", processedKey, err)
	}

	// Try to delete metadata (ignore errors)
	if err := h.metadata.DeleteAudioFile(ctx.Request.Context(), fileID); err != nil {
		log.Printf("cleanup: Failed to delete metadata for file %s: %v", fileID, err)
	}
}

func (h *UploadHandler) validateFile(fileHeader *multipart.FileHeader, userTier int) error {
	// Check if file is provided
	if fileHeader == nil {
		return fmt.Errorf("no file provided")
	}

	// Dynamic file size limits based on user tier
	var maxSize int64
	switch userTier {
	case 1: // Free tier
		maxSize = int64(300 * 1024 * 1024) // 300MB
	case 2, 3: // Premium/Pro tiers
		maxSize = int64(5 * 1024 * 1024 * 1024) // 5GB (effectively unlimited for most use cases)
	default:
		maxSize = int64(300 * 1024 * 1024) // Default to free tier
	}

	if fileHeader.Size > maxSize {
		if userTier == 1 {
			return fmt.Errorf("file too large (max 300MB). Upgrade to Premium for larger files")
		}
		return fmt.Errorf("file too large (max %dGB)", maxSize/(1024*1024*1024))
	}

	// Check minimum file size (1KB to avoid empty files)
	minSize := int64(1024)
	if fileHeader.Size < minSize {
		return fmt.Errorf("file too small (min 1KB)")
	}

	// Check file extension based on user tier
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))

	switch userTier {
	case 1: // Free tier - MP3 only
		if ext != ".mp3" {
			return fmt.Errorf("Only MP3 files are supported. Upgrade to Premium for WAV support")
		}
	case 2, 3: // Premium/Pro tiers - MP3 and WAV
		if ext != ".mp3" && ext != ".wav" {
			return fmt.Errorf("Only MP3 and WAV files are supported")
		}
	default:
		if ext != ".mp3" {
			return fmt.Errorf("Only MP3 files are supported")
		}
	}

	// Check filename length
	if len(fileHeader.Filename) > 255 {
		return fmt.Errorf("Filename too long (max 255 characters)")
	}

	return nil
}

func generateID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random fails
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

func getFileExtension(filename string) string {
	return strings.TrimPrefix(filepath.Ext(filename), ".")
}

func getProgressFromStatus(status string) int {
	switch status {
	case "queued":
		return 1
	case "processing":
		return 15
	case "completed":
		return 100
	case "failed":
		return 0
	default:
		return 0
	}
}

func (h *UploadHandler) isCustomLUFS(lufs float64) bool {
	presets := []float64{-14.0, -16.0, -7.0, -5.0, -23.0}

	for _, preset := range presets {
		if lufs == preset {
			return false
		}
	}

	return true
}

// Helper function to get content type from file format
func getContentTypeFromFormat(format string) string {
	switch strings.ToLower(format) {
	case "wav":
		return "audio/wav"
	case "mp3":
		return "audio/mpeg"
	case "flac":
		return "audio/flac"
	default:
		return "audio/mpeg"
	}
}
