package storage

import (
	"context"
	"io"
	"time"
)

// AudioStorage defines the interface for audio file storage operations
type AudioStorage interface {
	Upload(ctx context.Context, fileID string, reader io.Reader) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	GetPresignedURL(ctx context.Context, key string, duration time.Duration) (string, error)
}

// MetadataStorage defines the interface for metadata operations
type MetadataStorage interface {
	CreateAudioFile(ctx context.Context, file *AudioFile) error
	GetAudioFile(ctx context.Context, fileID string) (*AudioFile, error)
	UpdateStatus(ctx context.Context, fileID string, status string) error
	CreateJob(ctx context.Context, job *ProcessingJob) error
	UpdateJobStatus(ctx context.Context, jobID, status string, errorMsg *string) error
	GetJobByFileID(ctx context.Context, fileID string) (*ProcessingJob, error)
}

// StorageFactory defines the interface for creating storage instances
type StorageFactory interface {
	CreateAudioStorage() (AudioStorage, error)
	CreateMetadataStorage() (MetadataStorage, error)
}
