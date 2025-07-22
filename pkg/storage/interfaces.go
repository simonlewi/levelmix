// pkg/storage/interfaces.go
package storage

import (
	"context"
	"io"
	"time"
)

// AudioStorage handles file storage operations
type AudioStorage interface {
	Upload(ctx context.Context, key string, reader io.Reader, format string) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	GetPresignedURL(ctx context.Context, key string, duration time.Duration, format string) (string, error)
	GetUploadKey(fileID string, format string) string
	GetProcessedKey(fileID string, format string) string
}

// MetadataStorage handles database operations
type MetadataStorage interface {
	// Audio file operations
	CreateAudioFile(ctx context.Context, file *AudioFile) error
	GetAudioFile(ctx context.Context, fileID string) (*AudioFile, error)
	UpdateStatus(ctx context.Context, fileID string, status string) error
	DeleteAudioFile(ctx context.Context, fileID string) error

	// Job operations
	CreateJob(ctx context.Context, job *ProcessingJob) error
	GetJob(ctx context.Context, jobID string) (*ProcessingJob, error)
	GetJobByFileID(ctx context.Context, fileID string) (*ProcessingJob, error)
	UpdateJobStatus(ctx context.Context, jobID, status string, errorMsg *string) error
	UpdateJob(ctx context.Context, job *ProcessingJob) error

	// User operations
	CreateUser(ctx context.Context, user *User) error
	GetUser(ctx context.Context, userID string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	UpdateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, userID string) error

	// User stats operations
	CreateUserStats(ctx context.Context, stats *UserUploadStats) error
	GetUserStats(ctx context.Context, userID string) (*UserUploadStats, error)
	UpdateUserStats(ctx context.Context, stats *UserUploadStats) error

	// User jobs
	GetUserJobs(ctx context.Context, userID string, limit, offset int) ([]*ProcessingJob, error)
}

// StorageFactory creates storage instances
type StorageFactory interface {
	CreateAudioStorage() (AudioStorage, error)
	CreateMetadataStorage() (MetadataStorage, error)
}
