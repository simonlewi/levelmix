//go:build ee

package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"

	"github.com/simonlewi/levelmix-enterprise/marketing"
	payment_handlers "github.com/simonlewi/levelmix-enterprise/payment/handlers"
	ee_storage "github.com/simonlewi/levelmix-enterprise/storage"
	"github.com/simonlewi/levelmix/core/internal/audio"
	"github.com/simonlewi/levelmix/pkg/email"
)

func run() {
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

	// Initialize email service for trial reminder handler
	var emailService email.EmailService
	if os.Getenv("EMAIL_SERVICE") == "mock" || os.Getenv("RESEND_API_KEY") == "" {
		emailService = email.NewMockEmailService()
	} else {
		var err error
		emailService, err = email.NewResendService()
		if err != nil {
			log.Printf("Failed to initialize email service in worker, falling back to mock: %v", err)
			emailService = email.NewMockEmailService()
		}
	}

	// Build a minimal handler for the trial reminder task — only emailService is used at consume time
	trialHandler := payment_handlers.NewPaymentHandlers(nil, nil, emailService, nil)

	// Start worker
	srv, mux := audio.NewWorker(os.Getenv("REDIS_URL"), processor, trialHandler)

	// Marketing → Resend contact sync (Task type marketing:sync_resend).
	// Only wired when Resend is configured (RESEND_API_KEY + RESEND_AUDIENCE_ID);
	// in dev/mock environments it's skipped entirely. When enabled, a daily
	// scheduler enqueues the sync task, which this worker consumes.
	var marketingScheduler *asynq.Scheduler
	if syncer, err := marketing.NewSyncer(metadataStorage); err != nil {
		log.Printf("Marketing sync disabled: %v", err)
	} else {
		mux.HandleFunc(marketing.TypeMarketingSync, syncer.HandleSyncTask)

		marketingScheduler = asynq.NewScheduler(
			asynq.RedisClientOpt{
				Addr:     os.Getenv("REDIS_URL"),
				Password: os.Getenv("REDIS_PASSWORD"),
			},
			nil,
		)
		syncTask := asynq.NewTask(marketing.TypeMarketingSync, nil)
		// Run daily at 03:00. Routed to the low-weight notifications queue so it
		// never competes with audio processing.
		if _, err := marketingScheduler.Register("0 3 * * *", syncTask,
			asynq.Queue(payment_handlers.QueueNotifications)); err != nil {
			log.Printf("Failed to register marketing sync schedule: %v", err)
		} else if err := marketingScheduler.Start(); err != nil {
			log.Printf("Failed to start marketing sync scheduler: %v", err)
		} else {
			log.Println("Marketing sync scheduler started (daily at 03:00)")
		}
	}

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
	if marketingScheduler != nil {
		marketingScheduler.Shutdown()
	}
	srv.Shutdown()
}

// cleanupTempFiles removes any orphaned temp files from previous runs
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

	cleaned := 0
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "levelmix_") {
			filePath := filepath.Join(tempDir, file.Name())
			if err := os.Remove(filePath); err != nil {
				log.Printf("[WARN] Failed to remove temp file %s: %v", filePath, err)
			} else {
				cleaned++
			}
		}
	}

	if cleaned > 0 {
		log.Printf("[INFO] Cleaned up %d orphaned temp files", cleaned)
	}
}
