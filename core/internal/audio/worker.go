package audio

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

type Processor struct {
}

func NewProcessor() *Processor {
	return &Processor{}
}

func (p *Processor) HandleAudioProcess(ctx context.Context, t *asynq.Task) error {
	var task ProcessTask
	if err := json.Unmarshal(t.Payload(), &task); err != nil {
		return fmt.Errorf("failed to unmarshal task: %w", err)
	}

	if err := p.validateTask(task); err != nil {
		return fmt.Errorf("task validation failed: %w", err)
	}

	inputFile := p.getInputFilePath(task.FileID)
	loudnessInfo, err := AnalyzeLoudness(inputFile)
	if err != nil {
		return fmt.Errorf("loudness analysis failed: %w,", err)
	}

	outputFile := p.getOutputFilePath(task.FileID, task.JobID)
	outputOptions := p.getOutputOptions(task.IsPremium)

	if err := NormalizeLoudness(inputFile, outputFile, task.TargetLUFS, loudnessInfo, outputOptions); err != nil {
		return fmt.Errorf("loudness normalization failed: %w", err)
	}

	if err := p.storeProcessedFile(task, outputFile); err != nil {
		return fmt.Errorf("file storage failed: %w", err)
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

func (p *Processor) getInputFilePath(fileID string) string {
	// Implement input file path logic based on storage system here
	return fmt.Sprintf("/tmp/uploads/%s", fileID)
}

func (p *Processor) getOutputFilePath(fileID, jobID string) string {
	// Implement output file path logic based on storage system here
	return fmt.Sprintf("/tmp/processed/%s_%s.wav", fileID, jobID)
}

func (p *Processor) getOutputOptions(isPremium bool) OutputOptions {
	if isPremium {
		return OutputOptions{
			Codec:   "pcm_s24le", // Example codec for premium users
			Bitrate: "",          // No bitrate for lossless formats
		}
	}

	return OutputOptions{
		Codec:   "pcm_s16le", // Standard codec for non-premium users
		Bitrate: "320k",      // Example bitrate for MP3
	}
}

func (p *Processor) storeProcessedFile(task ProcessTask, outputFile string) error {
	// Implement your storage logic here
	// This might involve:
	// - Moving file to final storage location
	// - Updating database with processing status
	// - Notifying user that processing is complete
	// - Cleaning up temporary files

	// Example placeholder implementation:
	// return p.moveFileToStorage(outputFile, task.JobID)
	// return p.updateJobStatus(task.JobID, "completed")

	return nil // Placeholder
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
