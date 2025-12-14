package storage

import (
	"strings"
	"time"
)

// AudioFile represents an audio file in the system
type AudioFile struct {
	ID               string
	UserID           *string
	OriginalFilename string
	FileSize         int64
	Format           string
	Status           string
	LUFSTarget       float64
	DurationSeconds  *int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ProcessingJob represents a background processing job
type ProcessingJob struct {
	ID           string
	AudioFileID  string
	UserID       string
	Status       string
	TargetLUFS   *float64
	ErrorMessage *string
	OutputS3Key  string
	OutputFormat string
	StartedAt    *time.Time
	CompletedAt  *time.Time
	CreatedAt    time.Time
}

type User struct {
	ID                    string
	Email                 string
	Name                  *string
	PasswordHash          *string // Nullable for OAuth users
	CreatedAt             time.Time
	UpdatedAt             time.Time
	LastLoginAt           *time.Time
	AuthProvider          string // 'email', 'google', 'apple'
	AuthProviderID        *string
	SubscriptionTier      int // 1=free, 2=premium, 3=professional
	SubscriptionExpiresAt *time.Time
}

// User Helper Methods

// GetDisplayName returns the user's name if set, otherwise their email.
func (u *User) GetDisplayName() string {
	if u.Name != nil && *u.Name != "" {
		return *u.Name
	}
	return u.Email
}

// HasName returns true if the user has a name set (not nil and not empty).
func (u *User) HasName() bool {
	return u.Name != nil && *u.Name != ""
}

// GetFirstName extracts the first word from the user's name for casual greetings.
func (u *User) GetFirstName() string {
	if !u.HasName() {
		return ""
	}

	// Split name by whitespace
	parts := strings.Fields(*u.Name)
	if len(parts) > 0 {
		return parts[0]
	}

	// Fallback to full name if no spaces (single-name users)
	return *u.Name
}

// GetGreeting returns a friendly greeting string for the user.
func (u *User) GetGreeting() string {
	firstName := u.GetFirstName()
	if firstName != "" {
		return "Hi " + firstName
	}
	return "Hi there"
}

type UserUploadStats struct {
	UserID                     string
	TotalUploads               int
	TotalProcessingTimeSeconds int
	LastUploadAt               *time.Time

	// OLD FIELDS - keep temporarily during migration
	UploadsThisWeek int
	WeekResetAt     time.Time

	// NEW FIELDS - Primary tracking for usage limits
	ProcessingTimeThisMonth int
	MonthResetAt            time.Time
}

// Job status constants
const (
	StatusUploaded   = "uploaded"
	StatusQueued     = "queued"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

type CookieConsentRecord struct {
	ID             string
	UserID         *string // Nullable for anonymous users
	Essential      bool
	Analytics      bool
	Functional     bool
	ConsentVersion string
	UserAgent      string
	IPAddress      string
	CreatedAt      time.Time
}
