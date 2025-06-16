package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/joho/godotenv"

	"github.com/simonlewi/levelmix/core/internal/audio"
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

	// Initialize processor with dependencies
	processor := audio.NewProcessor(s3Storage, tursoStorage)

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
