package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/simonlewi/levelmix/core/internal/audio"
	ee_storage "github.com/simonlewi/levelmix/ee/storage"
)

// cleanupTempFiles removes any orphaned temp files from previous runs
// Only deletes files older than 2 hours to avoid interfering with active jobs
func cleanupTempFiles() {
	tempDir := "/tmp/levelmix"

	// Ensure the directory exists
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		log.Printf("[WARN] Failed to create temp directory for cleanup: %v", err)
		return
	}

	files, err := os.ReadDir(tempDir)
	if err != nil {
		log.Printf("[WARN] Failed to read temp directory for cleanup: %v", err)
		return
	}

	// Only delete files older than 2 hours (orphaned from crashes/restarts)
	// Active jobs complete within 60 minutes, so 2 hours is safe
	cutoffTime := time.Now().Add(-2 * time.Hour)
	cleaned := 0

	for _, file := range files {
		if strings.HasPrefix(file.Name(), "levelmix_") {
			filePath := filepath.Join(tempDir, file.Name())

			// Get file info to check age
			info, err := os.Stat(filePath)
			if err != nil {
				continue // Skip if we can't stat it
			}

			// Only delete if file is older than cutoff time
			if info.ModTime().Before(cutoffTime) {
				if err := os.Remove(filePath); err != nil {
					log.Printf("[WARN] Failed to remove old temp file %s: %v", filePath, err)
				} else {
					cleaned++
				}
			}
		}
	}

	if cleaned > 0 {
		log.Printf("[INFO] Cleaned up %d orphaned temp files (>2 hours old)", cleaned)
	}
}

func main() {
	_, b, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(b), "../../..")

	// Load environment variables
	envPath := filepath.Join(projectRoot, ".env")
	if _, err := os.Stat(envPath); err == nil {
		if err := godotenv.Load(envPath); err != nil {
			log.Printf("Error loading .env file to worker: %v", err)
		} else {
			log.Println(".env file loaded successfully to worker")
		}
	} else {
		log.Println("No .env file found, using environment variables from system/Docker")
	}

	// Clean up any orphaned temp files from previous runs
	cleanupTempFiles()

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

	processor := audio.NewProcessor(audioStorage, metadataStorage, os.Getenv("REDIS_URL"))

	// Start worker
	srv, mux := audio.NewWorker(os.Getenv("REDIS_URL"), processor)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Worker starting...")
	go func() {
		if err := audio.StartWorker(srv, mux); err != nil {
			log.Printf("Worker error: %v", err)
			quit <- syscall.SIGTERM
		}
	}()

	<-quit
	log.Println("Shutting down worker...")
	srv.Shutdown()
}
