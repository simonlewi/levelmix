# LevelMix - Architecture Documentation

## Project Overview
LevelMix is a web-based SaaS application that normalizes DJ mixes to specified LUFS target levels. The application allows users to upload audio files, process them to achieve consistent loudness levels, and download the normalized results.

## Core Problem
DJs and music producers often need to normalize their mixes to specific loudness standards for various platforms and venues. LevelMix solves this by providing an easy-to-use service that automates this technical audio processing task.

## Target Users
- DJs
- Music producers
- Podcast creators
- Electronic music enthusiasts
- Audio engineers

## Business Model
Freemium SaaS with tiered pricing:
- **Tier 1 (Free)**: 1 upload per month, MP3 format only
- **Tier 2 (Premium)**: 4 uploads per month, MP3 + WAV support, background processing, queue priority
- **Tier 3 (Pro)**: Unlimited uploads, multiple audio formats (MP3, WAV, FLAC, etc.), batch processing, background processing, queue priority

## MVP Timeline
- Development period: Approximately 1 month
- Development capacity: 2 hours on weekdays, 6-8 hours on weekends
- MVP features: Basic processing pipeline, user creation, job queuing, MP3 processing up to 300MB

## Technical Architecture

### Frontend
- HTML + HTMX for dynamic content without heavy JavaScript
- PicoCSS for initial styling
- TailwindCSS for specific styling choices (to be implemented later)
- Progressive enhancement approach

### Backend
- Language: Go
- Framework: Gin router with built-in html/template
- Primary functions: File handling, audio processing, authentication, job management

### Data Storage
- Database: Turso (SQLite-based) for metadata, user information, and job tracking
- File Storage: AWS S3 for audio files (original uploads and processed results)
- Pre-signed URLs for secure access to audio files

### Processing Pipeline
1. Upload → Validation → Analysis → Load Balancing → Normalization → Storage → Download
2. FFmpeg for audio processing
3. Job queuing system for handling multiple normalization requests

### Authentication
- OAuth2 with social login providers (primary method)
- Email/password authentication (to be added later)

### Job Queue Options
- Asynq (Redis-based, simple implementation)
- Machinery (more features, multiple backend options)

### API Endpoints
- File upload/download
- Process control
- Status updates (percentage-based progress tracking)
- User management
- Authentication

## Deployment Recommendations
- API Service: GCP Cloud Run
- Storage: AWS S3
- Database: Turso
- Queue: Asynq + Redis

## Feature Details

### Audio Processing
- Initial format support: MP3 only
- File size limit: 300MB (approx. 2 hours of audio)
- LUFS target options:
  - Standard presets (-14 for Streaming, -23 for Radio)
  - Manual entry
  - EDM-specific higher range presets (-7, -5 LUFS)

### User Experience
- No account required for basic usage (free tier)
- Audio preview before download
- Processing status tracking

### Premium Features (Future Implementation)
- Multiple audio format support (.wav, .flac, etc.)
- Increased monthly upload limits
- Priority in processing queue
- User dashboard with processing history
- Music recognition API integration for track listing with timestamps (ACRCloud)
- API access for third-party integration
- Background processing
- Batch processing

### Analytics
- Usage tracking
- Performance metrics
- User behavior analysis

### Error Handling
- Notification system for failed processing jobs
- Validation at upload time for obvious errors

### Scalability Considerations
- Horizontal scaling for processing workers
- Cloud storage for indefinite scaling of file storage
- Connection to external APIs for premium features

### Security Considerations
- Secure file handling
- Authentication best practices
- Protection against common web vulnerabilities
- Rate limiting