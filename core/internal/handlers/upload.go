package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/simonlewi/levelmix/core/internal/audio"
	"github.com/simonlewi/levelmix/pkg/storage"
)

type UploadHandler struct {
	storage  storage.AudioStorage
	metadata storage.MetadataStorage
	queue    *audio.QueueManager
}

func NewUploadHandler(s storage.AudioStorage, m storage.MetadataStorage, q *audio.QueueManager) *UploadHandler {
	return &UploadHandler{
		storage:  s,
		metadata: m,
		queue:    q,
	}
}

func (h *UploadHandler) HandleUpload(c *gin.Context) {
	// Get uploaded file
	fileHeader, err := c.FormFile("audio_file")
	if err != nil {
		h.returnError(c, "No file uploaded")
		return
	}

	// Validate file
	if err := h.validateFile(fileHeader); err != nil {
		h.returnError(c, err.Error())
		return
	}

	// Generate unique file ID
	fileID := generateID()

	// Get target LUFS
	targetLUFS, err := h.parseTargetLUFS(c.PostForm("target_lufs"))
	if err != nil {
		h.returnError(c, "Invalid LUFS target value")
		return
	}

	var userID *string
	isPremium := false

	if user, exists := c.Get("user"); exists {
		if u, ok := user.(*storage.User); ok {
			userID = &u.ID
			isPremium = u.SubscriptionTier > 1 //Premium/Pro users
			log.Printf("Authenticated user uploading: %s (tier: %d)", *userID, u.SubscriptionTier)
		}
	} else {
		log.Printf("Anonymous upload - no personal data stored")
	}

	// Open uploaded file
	file, err := fileHeader.Open()
	if err != nil {
		h.returnError(c, "Failed to process file")
		return
	}
	defer file.Close()

	// Upload to S3
	log.Printf("Attempting to upload file: %s to bucket:", fileID)
	if err := h.storage.Upload(c.Request.Context(), fileID, file); err != nil {
		log.Printf("Storage upload failed: %v", err)
		h.returnError(c, "Failed to store file")
		return
	}

	// Store metadata
	audioFile := &storage.AudioFile{
		ID:               fileID,
		UserID:           userID,
		OriginalFilename: fileHeader.Filename,
		FileSize:         fileHeader.Size,
		Format:           getFileExtension(fileHeader.Filename),
		Status:           "uploaded",
		LUFSTarget:       targetLUFS,
		CreatedAt:        time.Now(),
	}

	if err := h.metadata.CreateAudioFile(c.Request.Context(), audioFile); err != nil {
		// Clean up uploaded file if metadata fails
		log.Printf("Database error: %v", err)
		h.storage.Delete(c.Request.Context(), fileID)
		h.returnError(c, "Failed to save file metadata")
		return
	}

	// Queue processing job
	jobID := generateID()

	taskUserID := ""
	if userID != nil {
		taskUserID = *userID
	}

	task := audio.ProcessTask{
		JobID:      jobID,
		FileID:     fileID,
		TargetLUFS: targetLUFS,
		UserID:     taskUserID,
		IsPremium:  isPremium,
	}

	if err := h.queue.EnqueueProcessing(c.Request.Context(), task); err != nil {
		// Clean up if job queueing fails
		h.cleanup(c, fileID)
		h.returnError(c, "Failed to queue processing")
		return
	}

	job := &storage.ProcessingJob{
		ID:          jobID,
		AudioFileID: fileID,
		UserID:      taskUserID,
		Status:      "queued",
		CreatedAt:   time.Now(),
	}

	if err := h.metadata.CreateJob(c.Request.Context(), job); err != nil {
		log.Printf("Failed to create job record %v", err)
		h.cleanup(c, fileID)
		h.returnError(c, "Failed to create processing job")
		return
	}

	// Return processing state HTML
	processingHTML := h.generateProcessingHTML(fileID, jobID)
	c.Data(http.StatusOK, "text/html", []byte(processingHTML))
}

func (h *UploadHandler) GetStatus(c *gin.Context) {
	log.Println("*** REAL GetStatus called ***")
	fileID := c.Param("id")
	log.Printf("GetStatus called with fileID: '%s'", fileID)

	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File ID required"})
		return
	}

	// Get file info
	log.Printf("Looking up file in database: %s", fileID)
	audioFile, err := h.metadata.GetAudioFile(c.Request.Context(), fileID)
	if err != nil {
		log.Printf("ERROR: Database lookup failed: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	log.Printf("Found file: %s, status: %s", fileID, audioFile.Status)

	// Get job info
	log.Printf("Looking up job for file: %s", fileID)
	job, err := h.metadata.GetJobByFileID(c.Request.Context(), fileID)
	if err != nil {
		log.Printf("No job found, returning file status. Error: %v", err)
		// If no job found, return file status
		c.JSON(http.StatusOK, gin.H{
			"status":   audioFile.Status,
			"progress": 0,
			"fileID":   fileID,
		})
		return
	}

	log.Printf("Found job: %s, status: %s", job.ID, job.Status)

	progress := getProgressFromStatus(job.Status)

	response := gin.H{
		"status":   job.Status,
		"progress": progress,
		"fileID":   fileID,
	}

	// Add error message if job failed
	if job.Status == "failed" && job.ErrorMessage != "" {
		response["error"] = job.ErrorMessage
	}

	c.JSON(http.StatusOK, response)
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

	parsed, err := strconv.ParseFloat(lufsStr, 64)
	if err != nil {
		return 0, err
	}

	// Validate LUFS range (typical range is -30 to 0)
	if parsed < -30 || parsed > 0 {
		return 0, fmt.Errorf("LUFS target must be between -30 and 0")
	}

	return parsed, nil
}

// cleanup removes uploaded file and metadata on error
func (h *UploadHandler) cleanup(ctx *gin.Context, fileID string) {
	// Try to delete the uploaded file (ignore errors)
	h.storage.Delete(ctx.Request.Context(), fileID)

	// Try to delete metadata (ignore errors)
	h.metadata.DeleteAudioFile(ctx.Request.Context(), fileID)
}

func (h *UploadHandler) validateFile(fileHeader *multipart.FileHeader) error {
	// Check if file is provided
	if fileHeader == nil {
		return fmt.Errorf("no file provided")
	}

	// Check file size (300MB limit)
	maxSize := int64(300 * 1024 * 1024)
	if fileHeader.Size > maxSize {
		return fmt.Errorf("file too large (max 300MB)")
	}

	// Check minimum file size (1KB to avoid empty files)
	minSize := int64(1024)
	if fileHeader.Size < minSize {
		return fmt.Errorf("file too small (min 1KB)")
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext != ".mp3" {
		return fmt.Errorf("only MP3 files are supported")
	}

	// Check filename length
	if len(fileHeader.Filename) > 255 {
		return fmt.Errorf("filename too long (max 255 characters)")
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
		return 10
	case "processing":
		return 50
	case "completed":
		return 100
	case "failed":
		return 0
	default:
		return 0
	}
}
