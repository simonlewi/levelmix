package audio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	ee_storage "github.com/simonlewi/levelmix/ee/storage"
	"github.com/simonlewi/levelmix/pkg/storage"
)

type Processor struct {
	audioStorage    storage.AudioStorage
	metadataStorage storage.MetadataStorage
	redisClient     *redis.Client
	mu              sync.Mutex
}

func NewProcessor(audioStorage storage.AudioStorage, metadataStorage storage.MetadataStorage, redisURL string) *Processor {
	var redisClient *redis.Client
	if redisURL != "" {
		redisClient = redis.NewClient(&redis.Options{
			Addr:         redisURL,
			Password:     os.Getenv("REDIS_PASSWORD"),
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
			PoolSize:     10,
			MinIdleConns: 5,
			MaxRetries:   3,
		})

		// Test Redis connection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := redisClient.Ping(ctx).Err(); err != nil {
			log.Printf("[WARN] Redis unavailable, progress updates disabled: %v", err)
			redisClient = nil
		}
	}

	// Adjust FFmpeg concurrency based on CPU cores
	cpuCount := runtime.NumCPU()
	ffmpegConcurrency := cpuCount
	if ffmpegConcurrency > 4 {
		ffmpegConcurrency = 4 // Cap at 4 to prevent resource exhaustion
	}
	if ffmpegConcurrency < 2 {
		ffmpegConcurrency = 2
	}
	SetFFmpegConcurrency(ffmpegConcurrency)

	log.Printf("[INFO] Processor initialized (CPUs: %d, FFmpeg concurrency: %d)", cpuCount, ffmpegConcurrency)

	return &Processor{
		audioStorage:    audioStorage,
		metadataStorage: metadataStorage,
		redisClient:     redisClient,
	}
}

func (p *Processor) updateProgress(ctx context.Context, jobID string, progress int, status string) {
	if p.redisClient == nil {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	key := fmt.Sprintf("progress:%s", jobID)
	data := map[string]interface{}{
		"progress":   progress,
		"status":     status,
		"updated_at": time.Now().Unix(),
	}

	pipe := p.redisClient.Pipeline()
	pipe.HMSet(ctx, key, data)
	pipe.Expire(ctx, key, 30*time.Minute)

	if _, err := pipe.Exec(ctx); err != nil {
		// Only log if debug mode - progress updates failing isn't critical
		if debugMode {
			log.Printf("[DEBUG] Progress update failed for job %s: %v", jobID, err)
		}
	}
}

func (p *Processor) HandleAudioProcess(ctx context.Context, t *asynq.Task) (err error) {
	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic recovered: %v", r)
			log.Printf("[ERROR] Panic in audio processing: %v", r)
		}
	}()

	// Set overall timeout for the entire job
	jobTimeout := 30 * time.Minute
	ctx, cancel := context.WithTimeout(ctx, jobTimeout)
	defer cancel()

	var task ProcessTask
	if err := json.Unmarshal(t.Payload(), &task); err != nil {
		return fmt.Errorf("failed to unmarshal task: %w", err)
	}

	startTime := time.Now()
	log.Printf("[INFO] Job %s started (mode: %s, target: %.1f LUFS)",
		task.JobID, task.ProcessingMode, task.TargetLUFS)

	// Handle backward compatibility
	if task.FastMode && task.ProcessingMode == "" {
		task.ProcessingMode = ModeFast
	}
	if task.ProcessingMode == "" {
		task.ProcessingMode = ModePrecise
	}

	if err := p.validateTask(task); err != nil {
		return fmt.Errorf("task validation failed: %w", err)
	}

	job, err := p.metadataStorage.GetJob(ctx, task.JobID)
	if err != nil {
		return fmt.Errorf("failed to retrieve job %s: %w", task.JobID, err)
	}

	// Track cleanup operations
	var cleanupFiles []string
	defer func() {
		for _, file := range cleanupFiles {
			if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
				if debugMode {
					log.Printf("[DEBUG] Cleanup failed for %s: %v", file, err)
				}
			}
		}
	}()

	p.updateProgress(ctx, task.FileID, 10, "processing")

	now := time.Now()
	job.Status = "processing"
	job.StartedAt = &now
	if err := p.metadataStorage.UpdateJob(ctx, job); err != nil {
		// Non-critical, continue processing
		if debugMode {
			log.Printf("[DEBUG] Failed to update job status: %v", err)
		}
	}

	// Get audio file info
	audioFile, err := p.getAudioFileWithTimeout(ctx, task.FileID)
	if err != nil {
		return p.failJob(ctx, job, task.FileID, fmt.Errorf("failed to get audio file info: %w", err))
	}

	// Download file
	p.updateProgress(ctx, task.FileID, 20, "downloading")
	inputFile, err := p.downloadFileForProcessing(ctx, task.FileID, audioFile.Format)
	if err != nil {
		return p.failJob(ctx, job, task.FileID, fmt.Errorf("failed to download file: %w", err))
	}
	cleanupFiles = append(cleanupFiles, inputFile)

	// Verify downloaded file
	if info, err := os.Stat(inputFile); err != nil || info.Size() == 0 {
		return p.failJob(ctx, job, task.FileID, fmt.Errorf("downloaded file is invalid or empty"))
	}

	if debugMode {
		if info, _ := os.Stat(inputFile); info != nil {
			log.Printf("[DEBUG] Input file ready: %s (%.2f MB)", inputFile, float64(info.Size())/(1024*1024))
		}
	}

	// Determine output format and options
	outputFormat := p.determineOutputFormat(task.IsPremium, audioFile.Format)
	outputFile := p.getOutputFilePath(task.FileID, task.JobID, outputFormat)
	cleanupFiles = append(cleanupFiles, outputFile)
	outputOptions := p.getOutputOptions(task.IsPremium, audioFile.Format)

	// Update progress based on mode
	var statusMsg string
	switch task.ProcessingMode {
	case ModeFast:
		statusMsg = "fast_analyzing"
	case ModePrecise:
		statusMsg = "precise_analyzing"
	default:
		statusMsg = "analyzing"
	}
	p.updateProgress(ctx, task.FileID, 30, statusMsg)

	// Process with timeout monitoring
	processingCtx, processingCancel := context.WithTimeout(ctx, 20*time.Minute)
	defer processingCancel()

	processDone := make(chan error, 1)
	go func() {
		processDone <- ProcessAudioWithMode(inputFile, outputFile, task.TargetLUFS, outputOptions, task.ProcessingMode)
	}()

	// Update progress during normalization phase
	progressTicker := time.NewTicker(2 * time.Second)
	defer progressTicker.Stop()

	currentProgress := 30
	for {
		select {
		case err := <-processDone:
			if err != nil {
				return p.failJob(ctx, job, task.FileID, fmt.Errorf("audio processing failed: %w", err))
			}
			// Processing complete, break out of loop
			goto ProcessingComplete
		case <-processingCtx.Done():
			return p.failJob(ctx, job, task.FileID, fmt.Errorf("processing timed out after 20 minutes"))
		case <-progressTicker.C:
			// Incrementally update progress from 30% to 65% during processing
			if currentProgress < 65 {
				currentProgress += 3
				p.updateProgress(ctx, task.FileID, currentProgress, "normalizing")
			}
		}
	}

ProcessingComplete:
	p.updateProgress(ctx, task.FileID, 70, "normalizing")

	// Verify output file
	if info, err := os.Stat(outputFile); err != nil || info.Size() == 0 {
		return p.failJob(ctx, job, task.FileID, fmt.Errorf("processed file is invalid or empty"))
	}

	p.updateProgress(ctx, task.FileID, 85, "uploading")

	// Upload with timeout
	uploadCtx, uploadCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer uploadCancel()

	if err := p.uploadProcessedFile(uploadCtx, task.FileID, outputFile, outputFormat); err != nil {
		return p.failJob(ctx, job, task.FileID, fmt.Errorf("failed to upload processed file: %w", err))
	}

	// Mark as completed
	job.OutputFormat = outputFormat
	completedNow := time.Now()
	job.Status = "completed"
	job.CompletedAt = &completedNow

	if err := p.metadataStorage.UpdateJob(ctx, job); err != nil {
		// Non-critical error
		if debugMode {
			log.Printf("[DEBUG] Failed to update job to completed: %v", err)
		}
	}

	p.updateProgress(ctx, task.FileID, 100, "completed")

	if err := p.metadataStorage.UpdateStatus(ctx, task.FileID, "completed"); err != nil {
		if debugMode {
			log.Printf("[DEBUG] Failed to update file status: %v", err)
		}
	}

	// Update user stats if applicable
	if task.UserID != "" {
		p.updateUserStats(ctx, task.UserID, job)
	}

	// Log success with timing
	duration := time.Since(startTime)
	log.Printf("[INFO] Job %s completed in %.1fs", task.JobID, duration.Seconds())

	return nil
}

func (p *Processor) failJob(ctx context.Context, job *storage.ProcessingJob, fileID string, err error) error {
	log.Printf("[ERROR] Job %s failed: %v", job.ID, err)

	errMsg := err.Error()
	job.Status = "failed"
	job.ErrorMessage = &errMsg

	updateCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if updateErr := p.metadataStorage.UpdateJob(updateCtx, job); updateErr != nil {
		if debugMode {
			log.Printf("[DEBUG] Failed to update job status to failed: %v", updateErr)
		}
	}

	p.updateProgress(updateCtx, fileID, 0, "failed")

	return err
}

func (p *Processor) getAudioFileWithTimeout(ctx context.Context, fileID string) (*storage.AudioFile, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return p.metadataStorage.GetAudioFile(ctx, fileID)
}

func (p *Processor) updateUserStats(ctx context.Context, userID string, job *storage.ProcessingJob) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stats, err := p.metadataStorage.GetUserStats(ctx, userID)
	if err != nil {
		stats = &storage.UserUploadStats{
			UserID:                     userID,
			UploadsThisWeek:            0,
			WeekResetAt:                time.Now(),
			TotalUploads:               0,
			TotalProcessingTimeSeconds: 0,
		}
	}

	if job.StartedAt != nil && job.CompletedAt != nil {
		duration := job.CompletedAt.Sub(*job.StartedAt).Seconds()
		stats.TotalProcessingTimeSeconds += int(duration)

		if err := p.metadataStorage.UpdateUserStats(ctx, stats); err != nil {
			if debugMode {
				log.Printf("[DEBUG] Failed to update user stats: %v", err)
			}
		}
	}
}

func (p *Processor) determineOutputFormat(isPremium bool, inputFormat string) string {
	if !isPremium {
		return "mp3"
	}
	return strings.ToLower(inputFormat)
}

func (p *Processor) validateTask(task ProcessTask) error {
	if task.JobID == "" {
		return fmt.Errorf("job ID is required")
	}
	if task.FileID == "" {
		return fmt.Errorf("file ID is required")
	}
	if task.TargetLUFS < -50 || task.TargetLUFS > 0 {
		return fmt.Errorf("target LUFS must be between -50 and 0, got %f", task.TargetLUFS)
	}
	return nil
}

func (p *Processor) downloadFileForProcessing(ctx context.Context, fileID, format string) (string, error) {
	// Check disk space first
	if err := checkDiskSpace(); err != nil {
		return "", fmt.Errorf("disk space check failed: %w", err)
	}

	ext := ".mp3"
	switch strings.ToLower(format) {
	case "wav":
		ext = ".wav"
	case "flac":
		ext = ".flac"
	}

	tempFile, err := os.CreateTemp("", "levelmix_input_*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tempFileName := tempFile.Name()
	tempFile.Close()

	// Set download timeout
	downloadCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// Use the S3Storage method to get the correct key with extension
	var uploadKey string
	if s3Storage, ok := p.audioStorage.(*ee_storage.S3Storage); ok {
		uploadKey = s3Storage.GetUploadKey(fileID, format)

		// Try optimized multipart download with timeout
		err := s3Storage.DownloadToFile(downloadCtx, uploadKey, tempFileName)
		if err == nil {
			// Verify the downloaded file
			if info, err := os.Stat(tempFileName); err == nil && info.Size() > 0 {
				if debugMode {
					log.Printf("[DEBUG] Downloaded %s (%.2f MB)", uploadKey, float64(info.Size())/(1024*1024))
				}
				return tempFileName, nil
			}
		}
		// Only log fallback in debug mode
		if debugMode {
			log.Printf("[DEBUG] Multipart download failed, using stream: %v", err)
		}
	}

	// Fallback to stream download
	if s3Storage, ok := p.audioStorage.(*ee_storage.S3Storage); ok {
		uploadKey = s3Storage.GetUploadKey(fileID, format)
	} else {
		uploadKey = "uploads/" + fileID
	}

	reader, err := p.audioStorage.Download(downloadCtx, uploadKey)
	if err != nil {
		os.Remove(tempFileName)
		return "", fmt.Errorf("failed to download from S3: %w", err)
	}
	defer reader.Close()

	outFile, err := os.Create(tempFileName)
	if err != nil {
		os.Remove(tempFileName)
		return "", fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	written, err := io.Copy(outFile, reader)
	if err != nil {
		os.Remove(tempFileName)
		return "", fmt.Errorf("failed to copy file content: %w", err)
	}

	if written == 0 {
		os.Remove(tempFileName)
		return "", fmt.Errorf("downloaded file is empty")
	}

	return tempFileName, nil
}

func (p *Processor) uploadProcessedFile(ctx context.Context, fileID, filePath, outputFormat string) error {
	// Verify file exists and has content before upload
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("processed file not found: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("processed file is empty")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open processed file: %w", err)
	}
	defer file.Close()

	// Set upload timeout
	uploadCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// Cast to the concrete S3Storage type if available
	if s3Storage, ok := p.audioStorage.(*ee_storage.S3Storage); ok {
		err := s3Storage.UploadProcessed(uploadCtx, fileID, file, outputFormat)
		if err != nil {
			return fmt.Errorf("S3 upload failed: %w", err)
		}
		return nil
	}

	// Fallback to generic upload
	processedKey := "processed/" + fileID
	err = p.audioStorage.Upload(uploadCtx, processedKey, file, outputFormat)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	return nil
}

func (p *Processor) getOutputOptions(isPremium bool, inputFormat string) OutputOptions {
	outputFormat := p.determineOutputFormat(isPremium, inputFormat)

	switch strings.ToLower(outputFormat) {
	case "wav":
		return OutputOptions{
			Codec: "pcm_s16le",
			ExtraOptions: []string{
				"-ar", "44100",
				"-ac", "2",
			},
		}

	case "mp3":
		if isPremium {
			return OutputOptions{
				Codec:   "libmp3lame",
				Bitrate: "320k",
				ExtraOptions: []string{
					"-q:a", "0",
				},
			}
		}
		return OutputOptions{
			Codec:   "libmp3lame",
			Bitrate: "320k",
		}

	default:
		return OutputOptions{
			Codec:   "libmp3lame",
			Bitrate: "320k",
		}
	}
}

func (p *Processor) getOutputFilePath(fileID, jobID, outputFormat string) string {
	outputExt := ".mp3"

	switch strings.ToLower(outputFormat) {
	case "wav":
		outputExt = ".wav"
	case "mp3":
		outputExt = ".mp3"
	}

	return filepath.Join(os.TempDir(), fmt.Sprintf("levelmix_output_%s_%s%s", fileID, jobID, outputExt))
}

func NewWorker(redisAddr string, processor *Processor) (*asynq.Server, *asynq.ServeMux) {
	maxConcurrency := runtime.NumCPU() * 2

	// Cap concurrency to prevent resource exhaustion
	if maxConcurrency > 16 {
		maxConcurrency = 16
	}
	if maxConcurrency < 2 {
		maxConcurrency = 2
	}

	log.Printf("[INFO] Worker initialized (concurrency: %d, queues: fast=%d, premium=%d, standard=%d)",
		maxConcurrency,
		maxConcurrency*5/10,
		maxConcurrency*3/10,
		maxConcurrency*2/10)

	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:         redisAddr,
			Password:     os.Getenv("REDIS_PASSWORD"),
			DialTimeout:  10 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		},
		asynq.Config{
			Concurrency: maxConcurrency,
			Queues: map[string]int{
				QueueFast:     maxConcurrency * 5 / 10, // 50% for fast processing
				QueuePremium:  maxConcurrency * 3 / 10, // 30% for premium
				QueueStandard: maxConcurrency * 2 / 10, // 20% for standard
			},
			StrictPriority: true, // Fast -> Premium -> Standard

			// Only log actual errors, not retries
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				retried, _ := asynq.GetRetryCount(ctx)
				maxRetry, _ := asynq.GetMaxRetry(ctx)

				// Only log if it's the final failure
				if retried >= maxRetry {
					log.Printf("[ERROR] Task permanently failed - Type: %s, Error: %v",
						task.Type(), err)
				}
			}),

			// Exponential backoff for retries
			RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
				return time.Duration(n) * 30 * time.Second
			},
		},
	)

	mux := asynq.NewServeMux()

	// Add panic recovery wrapper
	mux.HandleFunc(TypeAudioProcess, func(ctx context.Context, t *asynq.Task) error {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[ERROR] Panic recovered in worker: %v", r)
			}
		}()
		return processor.HandleAudioProcess(ctx, t)
	})

	return srv, mux
}

func StartWorker(srv *asynq.Server, mux *asynq.ServeMux) error {
	log.Println("[INFO] Starting audio processing worker...")

	// Handle graceful shutdown
	go func() {
		sigterm := make(chan os.Signal, 1)
		signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)
		<-sigterm
		log.Println("[INFO] Shutdown signal received, stopping worker gracefully...")
		srv.Shutdown()
	}()

	if err := srv.Run(mux); err != nil {
		log.Printf("[ERROR] Worker failed to run: %v", err)
		return err
	}

	return nil
}
