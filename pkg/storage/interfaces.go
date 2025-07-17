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
	Delete(ctx context.Context, key string) error
}

// MetadataStorage defines the interface for metadata operations
type MetadataStorage interface {
	// Audio file operations
	CreateAudioFile(ctx context.Context, file *AudioFile) error
	GetAudioFile(ctx context.Context, fileID string) (*AudioFile, error)
	UpdateStatus(ctx context.Context, fileID string, status string) error
	CreateJob(ctx context.Context, job *ProcessingJob) error
	UpdateJobStatus(ctx context.Context, jobID, status string, errorMsg *string) error
	GetJobByFileID(ctx context.Context, fileID string) (*ProcessingJob, error)
	DeleteAudioFile(ctx context.Context, fileID string) error

	// User operations
	CreateUser(ctx context.Context, user *User) error
	GetUser(ctx context.Context, userID string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	UpdateUser(ctx context.Context, user *User) error
	
	CreateUserStats(ctx context.Context, stats *UserUploadStats) error
	GetUserStats(ctx context.Context, userID string) (*UserUploadStats, error)
	UpdateUserStats(ctx context.Context, stats *UserUploadStats) error
	
	GetUserJobs(ctx context.Context, userID string, limit, offset int) ([]*ProcessingJob, error)
	GetAudioFileByJobID(ctx context.Context, jobID string) (*AudioFile, error)	
}

// StorageFactory defines the interface for creating storage instances
type StorageFactory interface {
	CreateAudioStorage() (AudioStorage, error)
	CreateMetadataStorage() (MetadataStorage, error)
}


