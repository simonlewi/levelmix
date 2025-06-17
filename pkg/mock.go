package storage

import (
	"context"
	"fmt"
	"io"
	"time"
)

// MockAudioStorage is a mock implementation for testing
type MockAudioStorage struct {
	files map[string][]byte
}

func NewMockAudioStorage() *MockAudioStorage {
	return &MockAudioStorage{
		files: make(map[string][]byte),
	}
}

func (m *MockAudioStorage) Upload(ctx context.Context, fileID string, reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	m.files[fileID] = data
	return nil
}

func (m *MockAudioStorage) GetPresignedURL(ctx context.Context, key string, duration time.Duration) (string, error) {
	if _, exists := m.files[key]; !exists {
		return "", fmt.Errorf("file not found")
	}
	return fmt.Sprintf("https://mock-url.com/%s", key), nil
}

// MockMetadataStorage is a mock implementation for testing
type MockMetadataStorage struct {
	audioFiles map[string]*AudioFile
	jobs       map[string]*ProcessingJob
}

func NewMockMetadataStorage() *MockMetadataStorage {
	return &MockMetadataStorage{
		audioFiles: make(map[string]*AudioFile),
		jobs:       make(map[string]*ProcessingJob),
	}
}

func (m *MockMetadataStorage) CreateAudioFile(ctx context.Context, file *AudioFile) error {
	m.audioFiles[file.ID] = file
	return nil
}

func (m *MockMetadataStorage) GetAudioFile(ctx context.Context, fileID string) (*AudioFile, error) {
	file, exists := m.audioFiles[fileID]
	if !exists {
		return nil, fmt.Errorf("file not found")
	}
	return file, nil
}

func (m *MockMetadataStorage) UpdateStatus(ctx context.Context, fileID string, status string) error {
	file, exists := m.audioFiles[fileID]
	if !exists {
		return fmt.Errorf("file not found")
	}
	file.Status = status
	return nil
}

func (m *MockMetadataStorage) CreateJob(ctx context.Context, job *ProcessingJob) error {
	m.jobs[job.ID] = job
	return nil
}

func (m *MockMetadataStorage) UpdateJobStatus(ctx context.Context, jobID, status string, errorMsg *string) error {
	job, exists := m.jobs[jobID]
	if !exists {
		return fmt.Errorf("job not found")
	}
	job.Status = status
	if errorMsg != nil {
		job.ErrorMessage = *errorMsg
	}
	return nil
}

func (m *MockMetadataStorage) GetJobByFileID(ctx context.Context, fileID string) (*ProcessingJob, error) {
	for _, job := range m.jobs {
		if job.AudioFileID == fileID {
			return job, nil
		}
	}
	return nil, fmt.Errorf("job not found")
}
