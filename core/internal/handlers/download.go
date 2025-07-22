package handlers

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	ee_storage "github.com/simonlewi/levelmix/ee/storage"
	"github.com/simonlewi/levelmix/pkg/storage"
)

type DownloadHandler struct {
	storage  storage.AudioStorage
	metadata storage.MetadataStorage
}

func NewDownloadHandler(s storage.AudioStorage, m storage.MetadataStorage) *DownloadHandler {
	return &DownloadHandler{
		storage:  s,
		metadata: m,
	}
}

func (h *DownloadHandler) ShowResults(c *gin.Context) {
	fileID := c.Param("id")

	// Get file info
	audioFile, err := h.metadata.GetAudioFile(c.Request.Context(), fileID)
	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"error": "File not found",
		})
		return
	}

	// Check if processing is complete
	if audioFile.Status != "completed" {
		c.Redirect(http.StatusFound, "/processing/"+fileID)
		return
	}

	c.HTML(http.StatusOK, "results.html", gin.H{
		"CurrentPage": "results",
		"fileID":      fileID,
		"fileName":    audioFile.OriginalFilename,
		"targetLUFS":  audioFile.LUFSTarget,
	})
}

func (h *DownloadHandler) HandleDownload(c *gin.Context) {
	fileID := c.Param("id")

	audioFile, err := h.metadata.GetAudioFile(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	if audioFile.Status != "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File not ready"})
		return
	}

	// Get the job to determine the output format
	job, err := h.metadata.GetJobByFileID(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve job information"})
		return
	}

	// Determine the output format
	// Use the output_format field from the job
	outputFormat := audioFile.Format // Default to original format
	if job.OutputFormat != "" {
		outputFormat = job.OutputFormat
	} else if job.OutputS3Key != "" {
		// Backward compatibility: if OutputFormat is not set but OutputS3Key is
		outputFormat = job.OutputS3Key
	}

	// For free users, output is always MP3
	if audioFile.UserID == nil {
		outputFormat = "mp3"
	} else if audioFile.UserID != nil {
		// Check user tier to determine if they should get MP3 or original format
		user, err := h.metadata.GetUser(c.Request.Context(), *audioFile.UserID)
		if err == nil && user.SubscriptionTier == 1 {
			outputFormat = "mp3"
		}
	}

	// Build the correct filename with extension
	originalName := audioFile.OriginalFilename
	nameWithoutExt := strings.TrimSuffix(originalName, filepath.Ext(originalName))
	downloadFilename := fmt.Sprintf("%s_normalized.%s", nameWithoutExt, outputFormat)

	// Get file from S3 with the correct key
	var reader io.ReadCloser
	if s3Storage, ok := h.storage.(*ee_storage.S3Storage); ok {
		// Use the S3Storage method to get the correct key
		processedKey := s3Storage.GetProcessedKey(fileID, outputFormat)
		reader, err = h.storage.Download(c.Request.Context(), processedKey)
	} else {
		// Fallback for non-S3 storage
		reader, err = h.storage.Download(c.Request.Context(), "processed/"+fileID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve file"})
		return
	}
	defer reader.Close()

	// Set appropriate content type based on format
	contentType := "audio/mpeg" // Default to MP3
	switch outputFormat {
	case "wav":
		contentType = "audio/wav"
	case "flac":
		contentType = "audio/flac"
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", downloadFilename))
	c.Header("Content-Type", contentType)

	c.DataFromReader(http.StatusOK, -1, contentType, reader, nil)
}
