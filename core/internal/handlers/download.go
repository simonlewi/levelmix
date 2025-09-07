package handlers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
		log.Printf("Failed to get audio file %s: %v", fileID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	if audioFile.Status != "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File not ready"})
		return
	}

	// Get job information to determine output format
	job, err := h.metadata.GetJobByFileID(c.Request.Context(), fileID)
	if err != nil {
		log.Printf("Failed to get job for file %s: %v", fileID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve processing information"})
		return
	}

	// Determine output format with proper fallback
	outputFormat := "mp3" // Default fallback
	if job.OutputFormat != "" {
		outputFormat = job.OutputFormat
	} else {
		// Determine based on user tier
		if audioFile.UserID != nil {
			user, err := h.metadata.GetUser(c.Request.Context(), *audioFile.UserID)
			if err == nil && user.SubscriptionTier > 1 {
				outputFormat = audioFile.Format // Premium users get original format
			}
		}
	}

	// Generate download filename
	originalName := audioFile.OriginalFilename
	nameWithoutExt := strings.TrimSuffix(originalName, filepath.Ext(originalName))
	downloadFilename := fmt.Sprintf("%s_normalized.%s", nameWithoutExt, outputFormat)

	// Determine content type
	var contentType string
	switch strings.ToLower(outputFormat) {
	case "wav":
		contentType = "audio/wav"
	case "flac":
		contentType = "audio/flac"
	case "mp3":
		contentType = "audio/mpeg"
	default:
		contentType = "audio/mpeg"
	}

	log.Printf("Initiating download for file %s, format: %s, filename: %s", fileID, outputFormat, downloadFilename)

	// Try presigned URL first (best performance)
	if s3Storage, ok := h.storage.(*ee_storage.S3Storage); ok {
		// Create presigned URL with download headers to force download instead of streaming
		processedKey := s3Storage.GetProcessedKey(fileID, outputFormat)

		presigner := s3.NewPresignClient(s3Storage.GetClient())

		request, err := presigner.PresignGetObject(c.Request.Context(), &s3.GetObjectInput{
			Bucket:                     aws.String(s3Storage.GetBucket()),
			Key:                        aws.String(processedKey),
			ResponseContentDisposition: aws.String(fmt.Sprintf("attachment; filename=\"%s\"", downloadFilename)),
			ResponseContentType:        aws.String(contentType),
		}, s3.WithPresignExpires(1*time.Hour))

		if err == nil {
			log.Printf("Using presigned URL with download headers for: %s", fileID)
			c.Redirect(http.StatusTemporaryRedirect, request.URL)
			return
		}
		log.Printf("Presigned URL generation failed, falling back to direct download: %v", err)
	}

	// Fallback to direct download with proper headers
	h.directDownload(c, fileID, downloadFilename, contentType, outputFormat)
}

func (h *DownloadHandler) directDownload(c *gin.Context, fileID, filename, contentType, outputFormat string) {
	log.Printf("Using direct download for file %s", fileID)

	// Get the correct S3 key
	var processedKey string
	if s3Storage, ok := h.storage.(*ee_storage.S3Storage); ok {
		processedKey = s3Storage.GetProcessedKey(fileID, outputFormat)
	} else {
		processedKey = "processed/" + fileID
	}

	// Get file reader
	reader, err := h.storage.Download(c.Request.Context(), processedKey)
	if err != nil {
		log.Printf("Failed to download file %s: %v", fileID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve file"})
		return
	}
	defer reader.Close()

	// Try to get content length from S3 metadata
	var contentLength int64 = -1
	if s3Storage, ok := h.storage.(*ee_storage.S3Storage); ok {
		// Get object info for Content-Length header
		if info, err := s3Storage.GetObjectInfo(c.Request.Context(), processedKey); err == nil {
			contentLength = info.Size
		}
	}

	// Set comprehensive headers for better Chrome support
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("Content-Type", contentType)
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	// Set Content-Length if known (crucial for Chrome progress)
	if contentLength > 0 {
		c.Header("Content-Length", strconv.FormatInt(contentLength, 10))
		log.Printf("Set Content-Length: %d for file %s", contentLength, fileID)
	}

	// Stream with proper buffering
	c.Stream(func(w io.Writer) bool {
		buffer := make([]byte, 64*1024) // 64KB buffer
		n, err := reader.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error streaming file %s: %v", fileID, err)
			}
			return false
		}

		_, writeErr := w.Write(buffer[:n])
		return writeErr == nil
	})
}
