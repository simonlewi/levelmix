package audio

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hibiken/asynq"
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

	if err := p.metadataStorage.UpdateJobStatus(ctx, task.JobID, "processing", nil); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Download file from S3 to local temp
	inputFile, err := p.downloadFileForProcessing(ctx, task.FileID)
	if err != nil {
		errMsg := err.Error()
		p.metadataStorage.UpdateJobStatus(ctx, task.JobID, "failed", &errMsg)
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer os.Remove(inputFile) // Clean up temp file

	// Analyze loudness
	loudnessInfo, err := AnalyzeLoudness(inputFile)
	if err != nil {
		errMsg := err.Error()
		p.metadataStorage.UpdateJobStatus(ctx, task.JobID, "failed", &errMsg)
		return fmt.Errorf("loudness analysis failed: %w", err)
	}

	// Process audio
	outputFile := p.getOutputFilePath(task.FileID, task.JobID)
	defer os.Remove(outputFile) // Clean up temp file

	outputOptions := p.getOutputOptions(task.IsPremium)
	if err := NormalizeLoudness(inputFile, outputFile, task.TargetLUFS, loudnessInfo, outputOptions); err != nil {
		errMsg := err.Error()
		p.metadataStorage.UpdateJobStatus(ctx, task.JobID, "failed", &errMsg)
		return fmt.Errorf("loudness normalization failed: %w", err)
	}

	// Upload processed file back to S3
	if err := p.uploadProcessedFile(ctx, task.FileID, outputFile); err != nil {
		errMsg := err.Error()
		p.metadataStorage.UpdateJobStatus(ctx, task.JobID, "failed", &errMsg)
		return fmt.Errorf("failed to upload processed file: %w", err)
	}

	// Update job status to completed
	if err := p.metadataStorage.UpdateJobStatus(ctx, task.JobID, "completed", nil); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Update audio file status
	if err := p.metadataStorage.UpdateStatus(ctx, task.FileID, "completed"); err != nil {
		return fmt.Errorf("failed to update file status: %w", err)
	}

	return nil
}

// Helper functions

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

func (p *Processor) downloadFileForProcessing(ctx context.Context, fileID string) (string, error) {
	// Create temp file
	tempFile, err := os.CreateTemp("", "levelmix_input_*.mp3")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	// Download from S3 (assuming S3Storage has a Download method)
	// You'll need to add this method to your AudioStorage interface or cast to S3Storage
	// For now, this is a placeholder that needs S3 integration

	return tempFile.Name(), nil
}

func (p *Processor) uploadProcessedFile(ctx context.Context, fileID, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open processed file: %w", err)
	}
	defer file.Close()

	// Upload to S3 in processed/ folder
	processedKey := "processed/" + fileID
	return p.audioStorage.Upload(ctx, processedKey, file)
}

func (p *Processor) getOutputFilePath(fileID, jobID string) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("levelmix_output_%s_%s.mp3", fileID, jobID))
}

func (p *Processor) getOutputOptions(isPremium bool) OutputOptions {
	if isPremium {
		return OutputOptions{
			Codec:   "libmp3lame", // High quality MP3
			Bitrate: "320k",       // Premium bitrate
		}
	}

	return OutputOptions{
		Codec:   "libmp3lame", // Standard MP3
		Bitrate: "192k",       // Standard bitrate
	}
}

//

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
