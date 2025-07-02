package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/simonlewi/levelmix/core/internal/audio"
	"github.com/simonlewi/levelmix/core/internal/handlers"
	ee_storage "github.com/simonlewi/levelmix/ee/storage"
)

func main() {
	_, b, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(b), "../../..")

	// Load environment variables
	envPath := filepath.Join(projectRoot, ".env")
	if err := godotenv.Load(envPath); err != nil {
		log.Println("No .env file found")
	}

	// Initialize storage
	factory := ee_storage.NewFactory()

	audioStorage, err := factory.CreateAudioStorage()
	if err != nil {
		log.Fatal("Failed to create audio storage:", err)
	}

	metadataStorage, err := factory.CreateMetadataStorage()
	if err != nil {
		log.Fatal("Failed to create metadata storage:", err)
	}

	// Initialize queue
	qm := audio.NewQueueManager(os.Getenv("REDIS_URL"))
	defer qm.Shutdown()

	// Initialize handlers
	uploadHandler := handlers.NewUploadHandler(audioStorage, metadataStorage, qm)
	downloadHandler := handlers.NewDownloadHandler(audioStorage, metadataStorage)

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	r := gin.Default()

	baseTemplate := filepath.Join(projectRoot, "core", "templates", "base.html")
	homeTemplate := filepath.Join(projectRoot, "core", "templates", "pages", "home.html")
	uploadTemplate := filepath.Join(projectRoot, "core", "templates", "pages", "upload.html")

	r.LoadHTMLFiles(homeTemplate, uploadTemplate, baseTemplate)

	// Static files
	r.Static("/static", filepath.Join(projectRoot, "static"))

	// Routes
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "base.html", gin.H{
			"CurrentPage": "home",
		})
	})

	r.GET("/upload", func(c *gin.Context) {
		c.HTML(http.StatusOK, "upload.html", gin.H{
			"CurrentPage": "upload",
		})
	})

	r.POST("/upload", uploadHandler.HandleUpload)
	r.GET("/status/:id", uploadHandler.GetStatus)
	r.GET("/download/:id", downloadHandler.HandleDownload)
	r.GET("/results/:id", downloadHandler.ShowResults)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on http://localhost:%s", port)
	log.Printf("DEBUG: Templates loaded successfully")

	go func() {
		if err := r.Run(":" + port); err != nil {
			log.Printf("Server error: %v", err)
			quit <- syscall.SIGTERM
		}
	}()

	<-quit
	log.Println("Shutting down server...")
}
