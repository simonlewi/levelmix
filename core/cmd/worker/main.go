package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/simonlewi/levelmix/core/internal/audio"
	ee_storage "github.com/simonlewi/levelmix/ee/storage"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
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
