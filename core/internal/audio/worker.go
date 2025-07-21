package audio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time" // Import time package

	"github.com/hibiken/asynq"
	ee_storage "github.com/simonlewi/levelmix/ee/storage" // Assuming this is your S3 storage
	"github.com/simonlewi/levelmix/pkg/storage"
)

type Processor struct {
	audioStorage    storage.AudioStorage
	metadataStorage storage.MetadataStorage
}

func NewProcessor(audioStorage storage.AudioStorage, metadataStorage storage.MetadataStorage) *Processor {
	return &Processor{
		audioStorage:    audioStorage,
		metadataStorage: metadataStorage,
	}
}

func (p *Processor) HandleAudioProcess(ctx context.Context, t *asynq.Task) error {
	var task ProcessTask
	if err := json.Unmarshal(t.Payload(), &task); err != nil {
		return fmt.Errorf("failed to unmarshal task: %w", err)
	}

	if err := p.validateTask(task); err != nil {
		return fmt.Errorf("task validation failed: %w", err)
	}

	// Fetch the job to update its start time
	job, err := p.metadataStorage.GetJob(ctx, task.JobID)
	if err != nil {
		// Log and fail if we can't get the job, as we need it to track times
		return fmt.Errorf("failed to retrieve job %s for processing: %w", task.JobID, err)
	}

	// Update job status to processing and set start time
	now := time.Now()
	job.Status = "processing"
	job.StartedAt = &now // Use StartedAt as per your struct
	if err := p.metadataStorage.UpdateJob(ctx, job); err != nil {
		log.Printf("Failed to update job %s status to processing with start time: %v", job.ID, err)
		// Decide if you want to fail the task or just log and proceed.
		// For now, we'll log and proceed, but this might lead to inaccurate processing times if update fails.
	}

	// Get audio file info to determine format
	audioFile, err := p.metadataStorage.GetAudioFile(ctx, task.FileID)
	if err != nil {
		errMsg := err.Error()
		// Update job status to failed before returning
		job.Status = "failed"
		job.ErrorMessage = &errMsg
		p.metadataStorage.UpdateJob(ctx, job) // Try to update job with error
		return fmt.Errorf("failed to get audio file info for file %s: %w", task.FileID, err)
	}

	// Download file from S3 to local temp
	inputFile, err := p.downloadFileForProcessing(ctx, task.FileID, audioFile.Format)
	if err != nil {
		errMsg := err.Error()
		job.Status = "failed"
		job.ErrorMessage = &errMsg
		p.metadataStorage.UpdateJob(ctx, job)
		return fmt.Errorf("failed to download file %s: %w", task.FileID, err)
	}
	defer os.Remove(inputFile) // Clean up temp file

	// Analyze loudness
	loudnessInfo, err := AnalyzeLoudness(inputFile)
	if err != nil {
		errMsg := err.Error()
		job.Status = "failed"
		job.ErrorMessage = &errMsg
		p.metadataStorage.UpdateJob(ctx, job)
		return fmt.Errorf("loudness analysis failed for file %s: %w", task.FileID, err)
	}

	// Process audio
	outputFile := p.getOutputFilePath(task.FileID, task.JobID, audioFile.Format)
	defer os.Remove(outputFile) // Clean up temp file

	outputOptions := p.getOutputOptions(task.IsPremium, audioFile.Format)
	if err := NormalizeLoudness(inputFile, outputFile, task.TargetLUFS, loudnessInfo, outputOptions); err != nil {
		errMsg := err.Error()
		job.Status = "failed"
		job.ErrorMessage = &errMsg
		p.metadataStorage.UpdateJob(ctx, job)
		return fmt.Errorf("loudness normalization failed for file %s: %w", task.FileID, err)
	}

	// Upload processed file back to S3
	if err := p.uploadProcessedFile(ctx, task.FileID, outputFile); err != nil {
		errMsg := err.Error()
		job.Status = "failed"
		job.ErrorMessage = &errMsg
		p.metadataStorage.UpdateJob(ctx, job)
		return fmt.Errorf("failed to upload processed file %s: %w", task.FileID, err)
	}

	// Update job status to completed and set completion time
	completedNow := time.Now()
	job.Status = "completed"
	job.CompletedAt = &completedNow // Use CompletedAt as per your struct
	if err := p.metadataStorage.UpdateJob(ctx, job); err != nil {
		log.Printf("Failed to update job %s status to completed with end time: %v", job.ID, err)
		// Log and proceed, as the core task is done, but stats might be off
	}

	// Update audio file status (this was already there)
	if err := p.metadataStorage.UpdateStatus(ctx, task.FileID, "completed"); err != nil {
		log.Printf("Failed to update file %s status to completed: %v", task.FileID, err)
		// Log and proceed
	}

	// --- START: Logic to update TotalProcessingTimeSeconds ---
	if task.UserID != "" { // Only update stats for authenticated users
		stats, err := p.metadataStorage.GetUserStats(ctx, task.UserID)
		if err != nil {
			log.Printf("Could not retrieve user stats for user %s to update processing time: %v", task.UserID, err)
			// If stats don't exist, create a default one to avoid nil pointer dereference
			stats = &storage.UserUploadStats{
				UserID:                     task.UserID,
				UploadsThisWeek:            0,
				WeekResetAt:                time.Now(),
				TotalUploads:               0,
				TotalProcessingTimeSeconds: 0,
			}
			if err := p.metadataStorage.CreateUserStats(ctx, stats); err != nil {
				log.Printf("Failed to create default user stats for user %s: %v", task.UserID, err)
			}
		}

		// Calculate duration and add to total using your struct's StartedAt and CompletedAt
		if job.StartedAt != nil && job.CompletedAt != nil { // Check for nil pointers
			duration := job.CompletedAt.Sub(*job.StartedAt).Seconds()
			stats.TotalProcessingTimeSeconds += int(duration) // Convert to int seconds
		} else {
			log.Printf("Warning: StartedAt or CompletedAt is nil for job %s. Cannot calculate accurate duration.", job.ID)
		}

		if err := p.metadataStorage.UpdateUserStats(ctx, stats); err != nil {
			log.Printf("Failed to update user stats with processing time for user %s: %v", task.UserID, err)
		}
	}
	// --- END: Logic to update TotalProcessingTimeSeconds ---

	return nil
}

// Helper functions (unchanged)

func (p *Processor) validateTask(task ProcessTask) error {
	if task.JobID == "" {
		return fmt.Errorf("job ID is required")
	}
	if task.FileID == "" {
		return fmt.Errorf("file ID is required")
	}
	if task.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if task.TargetLUFS < -50 || task.TargetLUFS > 0 {
		return fmt.Errorf("target LUFS must be between -50 and 0, got %f", task.TargetLUFS)
	}
	return nil
}

func (p *Processor) downloadFileForProcessing(ctx context.Context, fileID, format string) (string, error) {
	// Create temp file with appropriate extension
	ext := ".mp3"
	if format == "wav" {
		ext = ".wav"
	}

	tempFile, err := os.CreateTemp("", "levelmix_input_*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tempFileName := tempFile.Name()
	tempFile.Close()

	uploadKey := "uploads/" + fileID

	log.Printf("Downloading file from S3: %s", uploadKey)
	reader, err := p.audioStorage.Download(ctx, uploadKey)
	if err != nil {
		os.Remove(tempFileName) // Clean up on error
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

	log.Printf("Successfully downloaded file to: %s", tempFileName)

	return tempFileName, nil
}

func (p *Processor) uploadProcessedFile(ctx context.Context, fileID, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open processed file: %w", err)
	}
	defer file.Close()

	// Cast to the concrete S3Storage type if available
	if s3Storage, ok := p.audioStorage.(*ee_storage.S3Storage); ok {
		return s3Storage.UploadProcessed(ctx, fileID, file)
	}

	// Fallback to upload to S3 in processed/ folder
	processedKey := "processed/" + fileID
	return p.audioStorage.Upload(ctx, processedKey, file)
}

func (p *Processor) getOutputOptions(isPremium bool, inputFormat string) OutputOptions {
	// Determine output format based on input and user tier
	outputFormat := inputFormat

	// Free users always get MP3 output regardless of input
	if !isPremium {
		outputFormat = "mp3"
	}

	switch strings.ToLower(outputFormat) {
	case "wav":
		if isPremium {
			return OutputOptions{
				Codec: "pcm_s16le", // High quality uncompressed WAV
				ExtraOptions: []string{
					"-ar", "44100", // 44.1kHz sample rate
					"-ac", "2", // Stereo
				},
			}
		}
		fallthrough // Fall through to MP3 for free users

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

func (p *Processor) getOutputFilePath(fileID, jobID, inputFormat string) string {
	// Determine output extension based on input format and processing rules
	outputExt := ".mp3" // Default to MP3

	switch strings.ToLower(inputFormat) {
	case "wav":
		outputExt = ".wav"
	case "mp3":
		outputExt = ".mp3"
	}

	return filepath.Join(os.TempDir(), fmt.Sprintf("levelmix_output_%s_%s%s", fileID, jobID, outputExt))
}

func NewWorker(redisAddr string, processor *Processor) (*asynq.Server, *asynq.ServeMux) {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: 10, // Number of concurrent workers
			Queues: map[string]int{
				QueuePremium:  6,
				QueueStandard: 3,
			},
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeAudioProcess, processor.HandleAudioProcess)

	return srv, mux
}

func StartWorker(srv *asynq.Server, mux *asynq.ServeMux) error {
	return srv.Run(mux)
}
