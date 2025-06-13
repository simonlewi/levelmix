package storage

import "time"

type AudioFile struct {
	ID               string
	UserID           string
	OriginalFilename string
	FileSize         int64
	Format           string
	Status           string
	LUFSTarget       float64
	CreatedAt        time.Time
}

type ProcessingJob struct {
	ID           string
	AudioFileID  string
	Status       string
	ErrorMessage string
	CreatedAt    time.Time
}
