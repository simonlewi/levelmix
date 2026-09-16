//go:build ee

package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/simonlewi/levelmix-enterprise/cleanup"
	"github.com/simonlewi/levelmix-enterprise/storage"
)

func run() {
	_, b, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(b), "../../..")

	// Load environment variables. .env.dev wins when present so local
	// development targets the dev database instead of prod; the Docker image
	// carries neither file and falls through to the system environment.
	envPath := filepath.Join(projectRoot, ".env.dev")
	if _, err := os.Stat(envPath); err != nil {
		envPath = filepath.Join(projectRoot, ".env")
	}
	if _, err := os.Stat(envPath); err == nil {
		if err := godotenv.Load(envPath); err != nil {
			log.Printf("Error loading %s to cleanup: %v", filepath.Base(envPath), err)
		} else {
			log.Printf("%s loaded successfully to cleanup", filepath.Base(envPath))
		}
	} else {
		log.Println("No .env file found, using environment variables from system/Docker")
	}

	log.Println("Starting cleanup job...")

	ctx := context.Background()

	// Get retention period from environment (default: 30 days)
	retentionDays := 30
	if days := os.Getenv("RETENTION_DAYS"); days != "" {
		if parsed, err := strconv.Atoi(days); err == nil {
			retentionDays = parsed
		}
	}

	// Initialize storage factory
	factory := storage.NewFactory()

	// Get S3 storage
	audioStorage, err := factory.CreateAudioStorage()
	if err != nil {
		log.Fatalf("Failed to create audio storage: %v", err)
	}

	// Get metadata storage
	metadataStorage, err := factory.CreateMetadataStorage()
	if err != nil {
		log.Fatalf("Failed to create metadata storage: %v", err)
	}

	// Configure S3 lifecycle rules (AWS handles cleanup automatically)
	if s3Storage, ok := audioStorage.(*storage.S3Storage); ok {
		cleaner := cleanup.NewS3Cleaner(s3Storage.GetClient(), s3Storage.GetBucket())

		log.Printf("Configuring S3 lifecycle rules for %d day retention", retentionDays)
		if err := cleaner.CleanupWithLifecycle(ctx, retentionDays); err != nil {
			log.Printf("Failed to configure lifecycle: %v", err)
		} else {
			log.Printf("S3 lifecycle rules configured successfully")
		}
	}

	// Run consent cleanup (2 years retention per cookie policy)
	consentCleaner := cleanup.NewConsentCleaner(metadataStorage)
	if err := consentCleaner.CleanupOldConsents(ctx, 2); err != nil {
		log.Printf("Consent cleanup failed: %v", err)
	}

	log.Println("Cleanup job completed")
}
