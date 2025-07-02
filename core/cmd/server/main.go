package main

import (
	"html/template"
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
	log.Printf("Project root: %s", projectRoot)
	envPath := filepath.Join(projectRoot, ".env")
	log.Printf("Looking for .env file at: %s", envPath)

	if err := godotenv.Load(envPath); err != nil {
		log.Printf("Error loading.env file: %v", err)
	} else {
		log.Println(".env file loaded successfully")
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

	// Templates
	templatesPattern := filepath.Join(projectRoot, "core", "templates", "**", "*.html")
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

	log.Println("=== ROUTE REGISTRATION START ===")
	log.Printf("Upload handler created: %+v", uploadHandler != nil)
	log.Printf("GetStatus method exists: %+v", uploadHandler != nil)

	r.GET("/status/:id", uploadHandler.GetStatus)

	r.GET("/test/:id", func(c *gin.Context) {
		fileID := c.Param("id")
		log.Printf("Test route called with ID: %s", fileID)
		c.JSON(200, gin.H{"test": "working", "id": fileID})
	})
	log.Println("Registered /test/:id route")

	routes := r.Routes()
	log.Printf("Total routes registered: %d", len(routes))
	for _, route := range routes {
		log.Printf("Route: %s %s", route.Method, route.Path)
	}
	log.Println("=== ROUTE REGISTRATION END ===")

	r.GET("/download/:id", downloadHandler.HandleDownload)
	r.GET("/results/:id", downloadHandler.ShowResults)
	log.Println("All routes registered")

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
