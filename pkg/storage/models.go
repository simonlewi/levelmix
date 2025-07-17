package storage

import "time"

// AudioFile represents an audio file in the system
type AudioFile struct {
	ID               string
	UserID           string
	OriginalFilename string
	FileSize         int64
	Format           string
	Status           string
	LUFSTarget       float64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ProcessingJob represents a background processing job
type ProcessingJob struct {
	ID           string
	AudioFileID  string
	Status       string
	ErrorMessage string
	OutputS3Key  string
	StartedAt    *time.Time
	CompletedAt  *time.Time
	CreatedAt    time.Time
}

type User struct {
	ID                   string
	Email                string
	PasswordHash         *string // Nullable for OAuth users
	CreatedAt            time.Time
	UpdatedAt            time.Time
	LastLoginAt          *time.Time
	AuthProvider         string // 'email', 'google', 'apple'
	AuthProviderID       *string
	SubscriptionTier     int // 1=free, 2=premium, 3=professional
	SubscriptionExpiresAt *time.Time
}

type UserUploadStats struct {
	ID                        string // Add ID field for updates
	UserID                    string
	TotalUploads              int
	TotalProcessingTimeSeconds int
	UploadsThisMonth          int
	LastUploadAt              *time.Time
	MonthResetAt              time.Time
}

// Job status constants
const (
	StatusUploaded   = "uploaded"
	StatusQueued     = "queued"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)
