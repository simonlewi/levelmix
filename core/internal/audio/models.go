package audio

type ProcessingMode string

const (
	ModePrecise ProcessingMode = "precise"
	ModeFast    ProcessingMode = "fast"
)

type LoudnessInfo struct {
	InputI        float64
	InputTP       float64
	InputLRA      float64
	InputThresh   float64
	InputLoudness float64
}

type ProcessTask struct {
	JobID          string         `json:"job_id"`
	FileID         string         `json:"file_id"`
	UserID         string         `json:"user_id"`
	TargetLUFS     float64        `json:"target_lufs"`
	Preset         string         `json:"preset"`
	IsPremium      bool           `json:"is_premium"`
	FastMode       bool           `json:"fast_mode"` // deprecated, used for backward compatibility
	ProcessingMode ProcessingMode `json:"processing_mode"`
}

type OutputOptions struct {
	Codec        string
	Bitrate      string
	ExtraOptions []string
}

// Preset LUFS targets
const (
	DefaultLUFS   = -5.0  // Default target
	DJMixLUFS     = -5.0  // DJ mixes - loud, punchy, club-ready
	StreamingLUFS = -14.0 // Spotify, Apple Music, YouTube
	PodcastLUFS   = -16.0 // Spoken word, podcasts
	BroadcastLUFS = -23.0 // Radio, TV (EBU R128)
)

// Safety limits
const (
	MaxLUFS = -2.0
	MinLUFS = -30.0
)

// Queue priority levels
const (
	QueueFast     = "fast"
	QueuePremium  = "premium"
	QueueStandard = "standard"
)

const (
	TypeAudioProcess = "audio:process"
)
