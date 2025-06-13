package storage

import (
	"context"
	"io"
	"time"
)

// AudioStorage defines the interface for audio file storage operations
type AudioStorage interface {
	Upload(ctx context.Context, fileID string, reader io.Reader) error
	GetPresignedURL(ctx context.Context, key string, duration time.Duration) (string, error)
}

// MetadataStorage defines the interface for metadata operations
type MetadataStorage interface {
	CreateAudioFile(ctx context.Context, file *AudioFile) error
	GetAudioFile(ctx context.Context, fileID string) (*AudioFile, error)
	UpdateStatus(ctx context.Context, fileID string, status string) error
}
