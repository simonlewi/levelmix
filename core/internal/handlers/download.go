package handlers

import (
	"net/http"
	"time"

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

	// Generate presigned URL for download
	downloadURL, err := h.storage.GetPresignedURL(c.Request.Context(), "processed/"+fileID, 1*time.Hour)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to generate download link",
		})
		return
	}

	c.HTML(http.StatusOK, "results.html", gin.H{
		"fileName":    audioFile.OriginalFilename,
		"downloadURL": downloadURL,
		"targetLUFS":  audioFile.LUFSTarget,
	})
}
