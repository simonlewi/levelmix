// core/internal/audio/models.go
package audio

// ProcessingMode represents different audio processing strategies
type ProcessingMode string

const (
	// ModePrecise uses full file analysis for maximum accuracy
	ModePrecise ProcessingMode = "precise"

	// ModeFast uses adaptive sampling for balanced speed and accuracy
	ModeFast ProcessingMode = "fast"
)

// LoudnessInfo contains audio loudness measurements from FFmpeg analysis
type LoudnessInfo struct {
	InputI        float64 // Integrated LUFS
	InputTP       float64 // True Peak
	InputLRA      float64 // Loudness Range
	InputThresh   float64 // Threshold
	InputLoudness float64 // Measured Loudness
}

// ProcessTask represents an audio processing job
type ProcessTask struct {
	JobID          string         `json:"job_id"`
	FileID         string         `json:"file_id"`
	UserID         string         `json:"user_id"`
	TargetLUFS     float64        `json:"target_lufs"`
	IsPremium      bool           `json:"is_premium"`
	ProcessingMode ProcessingMode `json:"processing_mode"`
	FastMode       bool           `json:"fast_mode"` // Deprecated, kept for backward compatibility
}

// OutputOptions defines encoding options for audio output
type OutputOptions struct {
	Codec        string   // e.g., "pcm_s16le", "flac", "libmp3lame", "aac"
	Bitrate      string   // e.g., "320k" for MP3
	ExtraOptions []string // Any additional FFmpeg options
}

// LUFS constants for different use cases
const (
	DefaultLUFS   = -7.0  // Default target LUFS optimized for DJ content
	MaxImpactLUFS = -5.0  // Higher output for loud content
	StreamingLUFS = -14.0 // Streaming standard
	PodcastLUFS   = -16.0 // Podcast standard
	BroadcastLUFS = -23.0 // Broadcast standard

	MaxLUFS = -2.0  // Prevent clipping
	MinLUFS = -30.0 // Prevent inaudible output
)

// Queue priority levels
const (
	QueueFast     = "fast"
	QueuePremium  = "premium"
	QueueStandard = "standard"
)

// Task types
const (
	TypeAudioProcess = "audio:process"
)
