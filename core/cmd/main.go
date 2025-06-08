package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/simonlewi/levelmix/core/internal/audio"
)

func main() {
	qm := audio.NewQueueManager("localhost:6379")
	defer qm.Shutdown()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	r := gin.Default()

	// Get directory of current file
	_, b, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(b), "..")

	templatesPattern := filepath.Join(projectRoot, "templates", "pages", "*.html")
	r.SetHTMLTemplate(template.Must(template.ParseGlob(templatesPattern)))

	// Serve static files
	r.Static("/static", filepath.Join(projectRoot, "static"))

	// Routes
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "home.html", gin.H{})
	})

	r.POST("process", func(c *gin.Context) {
		targetLUFS := audio.DefaultLUFS // Default target LUFS
		if lufsStr := c.PostForm("target_lufs"); lufsStr != "" {
			if parsed, err := strconv.ParseFloat(lufsStr, 64); err == nil {
				if parsed > audio.MaxLUFS {
					parsed = audio.MaxLUFS
				} else if parsed < audio.MinLUFS {
					parsed = audio.MinLUFS
				}
				targetLUFS = parsed
			}
		}
		task := audio.ProcessTask{
			JobID:      c.PostForm("job_id"),
			FileID:     c.PostForm("file_id"),
			TargetLUFS: targetLUFS,
			UserID:     c.PostForm("user_id"),
			IsPremium:  false,
		}

		if err := qm.EnqueueProcessing(c.Request.Context(), task); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "processing"})
	})

	r.GET("/upload", func(c *gin.Context) {
		c.HTML(http.StatusOK, "upload.html", gin.H{})
	})

	r.GET("/process", func(c *gin.Context) {
		c.HTML(http.StatusOK, "process.html", gin.H{})
	})

	r.GET("/results", func(c *gin.Context) {
		c.HTML(http.StatusOK, "results.html", gin.H{})
	})

	// Start server
	log.Println("Server starting on http://localhost:8080")
	go func() {
		if err := r.Run(":8080"); err != nil {
			log.Printf("Server error: %v", err)
			quit <- syscall.SIGTERM
		}
	}()

	<-quit
	log.Println("Shutting down server...")
}
