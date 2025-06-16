package main

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/simonlewi/levelmix/core/internal/audio"
	"github.com/simonlewi/levelmix/core/internal/handlers"
	"github.com/simonlewi/levelmix/core/internal/storage"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Initialize storage
	tursoStorage, err := storage.NewTursoStorage(
		os.Getenv("TURSO_DATABASE_URL"),
		os.Getenv("TURSO_AUTH_TOKEN"),
	)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Initialize S3
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal("Failed to load AWS config:", err)
	}

	s3Client := s3.NewFromConfig(cfg)
	s3Storage := storage.NewS3Storage(s3Client, os.Getenv("S3_BUCKET_NAME"))

	// Initialize queue
	qm := audio.NewQueueManager(os.Getenv("REDIS_URL"))
	defer qm.Shutdown()

	// Initialize handlers
	uploadHandler := handlers.NewUploadHandler(s3Storage, tursoStorage, qm)
	downloadHandler := handlers.NewDownloadHandler(s3Storage, tursoStorage)

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	r := gin.Default()

	// Templates
	_, b, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(b), "../..")
	templatesPattern := filepath.Join(projectRoot, "templates", "**", "*.html")
	r.SetHTMLTemplate(template.Must(template.ParseGlob(templatesPattern)))

	// Static files
	r.Static("/static", filepath.Join(projectRoot, "static"))

	// Routes
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "home.html", gin.H{})
	})

	r.GET("/upload", func(c *gin.Context) {
		c.HTML(http.StatusOK, "upload.html", gin.H{})
	})

	r.POST("/upload", uploadHandler.HandleUpload)
	r.GET("/status/:id", uploadHandler.GetStatus)
	r.GET("/results/:id", downloadHandler.ShowResults)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on http://localhost:%s", port)
	go func() {
		if err := r.Run(":" + port); err != nil {
			log.Printf("Server error: %v", err)
			quit <- syscall.SIGTERM
		}
	}()

	<-quit
	log.Println("Shutting down server...")
}
