package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/simonlewi/levelmix/core/internal/audio"
	"github.com/simonlewi/levelmix/core/internal/storage"
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
		c.HTML(http.StatusBadRequest, "upload_error.html", gin.H{
			"error": "No file uploaded",
		})
		return
	}

	// Validate file
	if err := h.validateFile(fileHeader); err != nil {
		c.HTML(http.StatusBadRequest, "upload_error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	// Generate unique file ID
	fileID := generateID()

	// Get target LUFS
	targetLUFS := audio.DefaultLUFS
	if lufsStr := c.PostForm("target_lufs"); lufsStr != "" {
		if parsed, err := strconv.ParseFloat(lufsStr, 64); err == nil {
			targetLUFS = parsed
		}
	}

	// Open uploaded file
	file, err := fileHeader.Open()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "upload_error.html", gin.H{
			"error": "Failed to process file",
		})
		return
	}
	defer file.Close()

	// Upload to S3
	if err := h.storage.Upload(c.Request.Context(), "uploads/"+fileID, file); err != nil {
		c.HTML(http.StatusInternalServerError, "upload_error.html", gin.H{
			"error": "Failed to store file",
		})
		return
	}

	// Store metadata
	audioFile := &storage.AudioFile{
		ID:               fileID,
		UserID:           "anonymous", // For now, will add auth later
		OriginalFilename: fileHeader.Filename,
		FileSize:         fileHeader.Size,
		Format:           getFileExtension(fileHeader.Filename),
		Status:           "uploaded",
		LUFSTarget:       targetLUFS,
		CreatedAt:        time.Now(),
	}

	if err := h.metadata.CreateAudioFile(c.Request.Context(), audioFile); err != nil {
		c.HTML(http.StatusInternalServerError, "upload_error.html", gin.H{
			"error": "Failed to save file metadata",
		})
		return
	}

	// Queue processing job
	jobID := generateID()
	task := audio.ProcessTask{
		JobID:      jobID,
		FileID:     fileID,
		TargetLUFS: targetLUFS,
		UserID:     "anonymous",
		IsPremium:  false,
	}

	if err := h.queue.EnqueueProcessing(c.Request.Context(), task); err != nil {
		c.HTML(http.StatusInternalServerError, "upload_error.html", gin.H{
			"error": "Failed to queue processing",
		})
		return
	}

	// Return processing page
	c.HTML(http.StatusOK, "processing.html", gin.H{
		"fileID": fileID,
		"jobID":  jobID,
	})
}

func (h *UploadHandler) GetStatus(c *gin.Context) {
	fileID := c.Param("id")

	// Get file info
	audioFile, err := h.metadata.GetAudioFile(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Get job info
	job, err := h.metadata.(*storage.TursoStorage).GetJobByFileID(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":   audioFile.Status,
			"progress": 0,
		})
		return
	}

	progress := getProgressFromStatus(job.Status)

	c.JSON(http.StatusOK, gin.H{
		"status":   job.Status,
		"progress": progress,
		"fileID":   fileID,
	})
}

func (h *UploadHandler) validateFile(fileHeader *http.Header) error {
	// Check file size (300MB limit)
	maxSize := int64(300 * 1024 * 1024)
	if fileHeader.Size > maxSize {
		return fmt.Errorf("file too large (max 300MB)")
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext != ".mp3" {
		return fmt.Errorf("only MP3 files are supported")
	}

	return nil
}

func generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
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
