package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/simonlewi/levelmix/core/internal/audio"
	ee_storage "github.com/simonlewi/levelmix/ee/storage"
)

func main() {
	_, b, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(b), "../../..")

	// Load environment variables
	log.Printf("Project root: %s", projectRoot)

	if err := godotenv.Load(".env.production", filepath.Join(projectRoot, ".env.production"), ".env"); err != nil {
		log.Printf("Error loading .env file to worker: %v", err)
	} else {
		log.Println(".env file loaded successfully to worker")
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

	processor := audio.NewProcessor(audioStorage, metadataStorage)

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
