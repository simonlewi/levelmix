package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
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

	originalName := audioFile.OriginalFilename
	nameWithoutExt := strings.TrimSuffix(originalName, filepath.Ext(originalName))
	downloadFilename := nameWithoutExt + "_normalized.mp3"

	// Get file from S3
	reader, err := h.storage.Download(c.Request.Context(), "processed/"+fileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve file"})
		return
	}
	defer reader.Close()

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", downloadFilename))
	c.Header("Content-Type", "audio/mpeg")

	c.DataFromReader(http.StatusOK, -1, "audio/mpeg", reader, nil)
}
