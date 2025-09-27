package audio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
}

func NewProcessor(audioStorage storage.AudioStorage, metadataStorage storage.MetadataStorage, redisURL string) *Processor {
	var redisClient *redis.Client
	if redisURL != "" {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     redisURL,
			Password: os.Getenv("REDIS_PASSWORD"),
		})
	}

	return &Processor{
		audioStorage:    audioStorage,
		metadataStorage: metadataStorage,
		redisClient:     redisClient,
	}
}

func (p *Processor) updateProgress(ctx context.Context, jobID string, progress int, status string) {
	if p.redisClient != nil {
		key := fmt.Sprintf("progress:%s", jobID)
		data := map[string]interface{}{
			"progress": progress,
			"status":   status,
		}
		p.redisClient.HMSet(ctx, key, data)
		p.redisClient.Expire(ctx, key, 30*time.Minute)
	}
}

func (p *Processor) HandleAudioProcess(ctx context.Context, t *asynq.Task) error {
	var task ProcessTask
	if err := json.Unmarshal(t.Payload(), &task); err != nil {
		return fmt.Errorf("failed to unmarshal task: %w", err)
	}

	// Handle backward compatibility
	if task.FastMode && task.ProcessingMode == "" {
		task.ProcessingMode = ModeFast
	}
	if task.ProcessingMode == "" {
		task.ProcessingMode = ModePrecise // Default to precise
	}

	if err := p.validateTask(task); err != nil {
		return fmt.Errorf("task validation failed: %w", err)
	}

	job, err := p.metadataStorage.GetJob(ctx, task.JobID)
	if err != nil {
		return fmt.Errorf("failed to retrieve job %s for processing: %w", task.JobID, err)
	}

	log.Printf("Processing job %s in %s mode", task.JobID, task.ProcessingMode)

	p.updateProgress(ctx, job.ID, 10, "processing")

	now := time.Now()
	job.Status = "processing"
	job.StartedAt = &now
	if err := p.metadataStorage.UpdateJob(ctx, job); err != nil {
		log.Printf("Failed to update job %s status to processing: %v", job.ID, err)
	}

	audioFile, err := p.metadataStorage.GetAudioFile(ctx, task.FileID)
	if err != nil {
		errMsg := err.Error()
		job.Status = "failed"
		job.ErrorMessage = &errMsg
		p.metadataStorage.UpdateJob(ctx, job)
		return fmt.Errorf("failed to get audio file info: %w", err)
	}

	// Download file (both modes need local file)
	inputFile, err := p.downloadFileForProcessing(ctx, task.FileID, audioFile.Format)
	if err != nil {
		errMsg := err.Error()
		job.Status = "failed"
		job.ErrorMessage = &errMsg
		p.metadataStorage.UpdateJob(ctx, job)
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer os.Remove(inputFile)

	// Determine output format and options
	outputFormat := p.determineOutputFormat(task.IsPremium, audioFile.Format)
	outputFile := p.getOutputFilePath(task.FileID, task.JobID, outputFormat)
	defer os.Remove(outputFile)
	outputOptions := p.getOutputOptions(task.IsPremium, audioFile.Format)

	// Process using unified function that handles both modes
	log.Printf("Processing job %s in %s mode", task.JobID, task.ProcessingMode)

	// Update progress based on mode
	if task.ProcessingMode == ModeFast {
		p.updateProgress(ctx, job.ID, 30, "fast_analyzing")
	} else {
		p.updateProgress(ctx, job.ID, 30, "analyzing")
	}

	// Use the unified processing function from fast-analyzer.go
	err = ProcessAudioWithMode(inputFile, outputFile, task.TargetLUFS, outputOptions, task.ProcessingMode)

	p.updateProgress(ctx, job.ID, 70, "normalizing")

	if err != nil {
		errMsg := err.Error()
		job.Status = "failed"
		job.ErrorMessage = &errMsg
		p.metadataStorage.UpdateJob(ctx, job)
		return fmt.Errorf("audio processing failed: %w", err)
	}

	p.updateProgress(ctx, job.ID, 85, "uploading")

	// Upload processed file (same for both modes)
	if err := p.uploadProcessedFile(ctx, task.FileID, outputFile, outputFormat); err != nil {
		errMsg := err.Error()
		job.Status = "failed"
		job.ErrorMessage = &errMsg
		p.metadataStorage.UpdateJob(ctx, job)
		return fmt.Errorf("failed to upload processed file: %w", err)
	}

	// Mark completed (same for both modes)
	job.OutputFormat = outputFormat
	completedNow := time.Now()
	job.Status = "completed"
	job.CompletedAt = &completedNow

	if err := p.metadataStorage.UpdateJob(ctx, job); err != nil {
		log.Printf("Failed to update job %s to completed: %v", job.ID, err)
	}

	p.updateProgress(ctx, job.ID, 100, "completed")

	if err := p.metadataStorage.UpdateStatus(ctx, task.FileID, "completed"); err != nil {
		log.Printf("Failed to update file status: %v", err)
	}

	// Update user stats (same for both modes)
	if task.UserID != "" {
		stats, err := p.metadataStorage.GetUserStats(ctx, task.UserID)
		if err != nil {
			stats = &storage.UserUploadStats{
				UserID:                     task.UserID,
				UploadsThisWeek:            0,
				WeekResetAt:                time.Now(),
				TotalUploads:               0,
				TotalProcessingTimeSeconds: 0,
			}
		}

		if job.StartedAt != nil && job.CompletedAt != nil {
			duration := job.CompletedAt.Sub(*job.StartedAt).Seconds()
			stats.TotalProcessingTimeSeconds += int(duration)
		}
	}

	if job.StartedAt != nil {
		duration := completedNow.Sub(*job.StartedAt).Seconds()
		log.Printf("Job %s: completed in %.1fs", task.JobID, duration)
	}

	return nil
}

// determineOutputFormat decides the output format based on user tier and input format
func (p *Processor) determineOutputFormat(isPremium bool, inputFormat string) string {
	// Free users always get MP3 output regardless of input
	if !isPremium {
		return "mp3"
	}

	// Premium users get the same format as input (wav stays wav, mp3 stays mp3)
	return strings.ToLower(inputFormat)
}

func (p *Processor) validateTask(task ProcessTask) error {
	if task.JobID == "" {
		return fmt.Errorf("job ID is required")
	}
	if task.FileID == "" {
		return fmt.Errorf("file ID is required")
	}
	// Note: UserID can be empty for anonymous uploads
	if task.TargetLUFS < -50 || task.TargetLUFS > 0 {
		return fmt.Errorf("target LUFS must be between -50 and 0, got %f", task.TargetLUFS)
	}
	return nil
}

func (p *Processor) downloadFileForProcessing(ctx context.Context, fileID, format string) (string, error) {
	// Create temp file using system temp directory
	ext := ".mp3"
	switch format {
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

	// Use the S3Storage method to get the correct key with extension
	var uploadKey string
	if s3Storage, ok := p.audioStorage.(*ee_storage.S3Storage); ok {
		uploadKey = s3Storage.GetUploadKey(fileID, format)

		// Try optimized multipart download
		err := s3Storage.DownloadToFile(ctx, uploadKey, tempFileName)
		if err == nil {
			return tempFileName, nil
		}
	}

	// Fallback to stream download
	if s3Storage, ok := p.audioStorage.(*ee_storage.S3Storage); ok {
		uploadKey = s3Storage.GetUploadKey(fileID, format)
	} else {
		uploadKey = "uploads/" + fileID
	}

	log.Printf("Using stream download for: %s", uploadKey)
	reader, err := p.audioStorage.Download(ctx, uploadKey)
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

	_, err = io.Copy(outFile, reader)
	if err != nil {
		os.Remove(tempFileName)
		return "", fmt.Errorf("failed to copy file content: %w", err)
	}

	return tempFileName, nil
}

func (p *Processor) uploadProcessedFile(ctx context.Context, fileID, filePath, outputFormat string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open processed file: %w", err)
	}
	defer file.Close()

	// Cast to the concrete S3Storage type if available
	if s3Storage, ok := p.audioStorage.(*ee_storage.S3Storage); ok {
		return s3Storage.UploadProcessed(ctx, fileID, file, outputFormat)
	}

	// Fallback to upload to S3 in processed/ folder
	processedKey := "processed/" + fileID
	return p.audioStorage.Upload(ctx, processedKey, file, outputFormat)
}

func (p *Processor) getOutputOptions(isPremium bool, inputFormat string) OutputOptions {
	// Determine output format based on input and user tier
	outputFormat := p.determineOutputFormat(isPremium, inputFormat)

	switch strings.ToLower(outputFormat) {
	case "wav":
		return OutputOptions{
			Codec: "pcm_s16le", // High quality uncompressed WAV
			ExtraOptions: []string{
				"-ar", "44100", // 44.1kHz sample rate
				"-ac", "2", // Stereo
			},
		}

	case "mp3":
		if isPremium {
			return OutputOptions{
				Codec:   "libmp3lame",
				Bitrate: "320k",
				ExtraOptions: []string{
					"-q:a", "0", // Highest quality VBR
				},
			}
		}
		return OutputOptions{
			Codec:   "libmp3lame",
			Bitrate: "320k", // Standard quality for free users
		}

	default:
		// Default to MP3 for unknown formats
		return OutputOptions{
			Codec:   "libmp3lame",
			Bitrate: "320k",
		}
	}
}

func (p *Processor) getOutputFilePath(fileID, jobID, outputFormat string) string {
	// Determine output extension based on the output format
	outputExt := ".mp3" // Default to MP3

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

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr, Password: os.Getenv("REDIS_PASSWORD")},
		asynq.Config{
			Concurrency: maxConcurrency,
			Queues: map[string]int{
				QueueFast:     maxConcurrency * 5 / 10, // 50% for fast processing
				QueuePremium:  maxConcurrency * 3 / 10, // 30% for premium
				QueueStandard: maxConcurrency * 2 / 10, // 20% for standard
			},
			StrictPriority: true, // Fast -> Premium -> Standard
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeAudioProcess, processor.HandleAudioProcess)

	return srv, mux
}

func StartWorker(srv *asynq.Server, mux *asynq.ServeMux) error {
	return srv.Run(mux)
}
