# LevelMix - Database Schema

## Overview
This document outlines the database schema for LevelMix, a web-based SaaS application for normalizing DJ mixes to specified LUFS target levels. The schema is designed for Turso DB, which is SQLite-compatible.

## Tables

### Users
```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_login_at TIMESTAMP,
    auth_provider TEXT, -- 'email', 'google', 'apple', etc.
    auth_provider_id TEXT, -- ID from OAuth provider if applicable
    subscription_tier INTEGER DEFAULT 1, -- 1=free, 2=premium, 3=premium+
    subscription_expires_at TIMESTAMP
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_auth_provider ON users(auth_provider, auth_provider_id);
```

### Processing Jobs
```sql
CREATE TABLE processing_jobs (
    id TEXT PRIMARY KEY,
    user_id TEXT,
    status TEXT NOT NULL, -- 'queued', 'processing', 'completed', 'failed'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    progress_percentage INTEGER DEFAULT 0,
    error_message TEXT,
    priority INTEGER DEFAULT 0, -- Higher number = higher priority
    original_file_path TEXT NOT NULL, -- S3 path
    processed_file_path TEXT, -- S3 path (null until completed)
    target_lufs REAL NOT NULL, -- e.g. -14.0
    
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_processing_jobs_user ON processing_jobs(user_id);
CREATE INDEX idx_processing_jobs_status ON processing_jobs(status);
CREATE INDEX idx_processing_jobs_queue ON processing_jobs(status, priority, created_at);
```

### Audio Files
```sql
CREATE TABLE audio_files (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL,
    original_filename TEXT NOT NULL,
    file_format TEXT NOT NULL, -- 'mp3', 'wav', etc.
    file_size INTEGER NOT NULL, -- in bytes
    duration_seconds INTEGER NOT NULL,
    original_lufs REAL, -- measured during analysis
    target_lufs REAL NOT NULL,
    sample_rate INTEGER, -- e.g., 44100
    bit_rate INTEGER, -- e.g., 320000 for 320kbps
    channels INTEGER, -- 1=mono, 2=stereo
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (job_id) REFERENCES processing_jobs(id) ON DELETE CASCADE
);

CREATE INDEX idx_audio_files_job ON audio_files(job_id);
```

### User Upload Stats
```sql
CREATE TABLE user_upload_stats (
    user_id TEXT PRIMARY KEY,
    total_uploads INTEGER DEFAULT 0,
    total_processing_time_seconds INTEGER DEFAULT 0,
    uploads_this_month INTEGER DEFAULT 0,
    last_upload_at TIMESTAMP,
    month_reset_at TIMESTAMP, -- When monthly counter was last reset
    
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
```

### LUFS Presets
```sql
CREATE TABLE lufs_presets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    lufs_value REAL NOT NULL,
    description TEXT,
    is_system_preset BOOLEAN DEFAULT FALSE
);

-- Default presets
INSERT INTO lufs_presets (name, lufs_value, description, is_system_preset) 
VALUES 
    ('Spotify', -14.0, 'Recommended for Spotify streaming', TRUE),
    ('YouTube', -14.0, 'Recommended for YouTube uploads', TRUE),
    ('Apple Music', -16.0, 'Recommended for Apple Music', TRUE),
    ('Broadcast Standard', -23.0, 'EBU R128 broadcast standard', TRUE),
    ('Club Ready', -7.0, 'High energy EDM club mix', TRUE),
    ('Maximum Impact', -5.0, 'Very loud EDM master', TRUE);
```

### API Keys
```sql
CREATE TABLE api_keys (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    api_key TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL, -- User-defined name for this key
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE,
    
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_api_keys_key ON api_keys(api_key);
CREATE INDEX idx_api_keys_user ON api_keys(user_id);
```

## Relationships

### One-to-Many
- User to Processing Jobs: A user can have multiple processing jobs

### One-to-One
- Processing Job to Audio File: Each processing job is associated with one audio file
- User to Upload Stats: Each user has one set of upload statistics

## Usage Tracking Logic

### Upload Limits
When a user uploads a file and creates a job:
- Increment `uploads_this_month` in user_upload_stats
- Update `last_upload_at` timestamp

Monthly reset (via cron job):
- Reset `uploads_this_month` to 0
- Update `month_reset_at` timestamp

### Upload Validation
Before accepting an upload:
- Free tier: Check if `uploads_this_month < 1`
- Premium tier: Check if `uploads_this_month < 4`
- Premium+ tier: No limit check required

## Implementation Notes

- **IDs**: Using TEXT type for IDs to accommodate UUID strings
- **Timestamps**: Using ISO8601 format (YYYY-MM-DD HH:MM:SS)
- **Soft Limits**: The initial implementation will track usage but not strictly enforce limits
- **Migrations**: As the application grows, migrations should be managed through a dedicated migration tool