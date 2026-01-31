# LevelMix

[![CI](https://github.com/simonlewi/levelmix/actions/workflows/ci.yml/badge.svg)](https://github.com/simonlewi/levelmix/actions/workflows/ci.yml)

A web-based SaaS application that normalizes longer audio files to specified LUFS target levels, making audio content consistent and professional.

## Description

LevelMix is a powerful yet simple audio normalization service designed for content creators who need to ensure consistent loudness levels across their audio content. Built with Go, vanilla JavaScript, and TailwindCSS, it provides a fast, efficient way to process audio files without the need for complex software or technical expertise.

**Key Features:**
- 🎵 **Audio Normalization**: Automatically normalize audio files to industry-standard LUFS levels
- 📊 **Multiple Presets**: Choose from DJ mix (-5 LUFS), streaming (-14 LUFS), podcast (-16 LUFS), or broadcast (-23 LUFS) presets
- 🚀 **Fast Processing**: Efficient FFmpeg-based processing pipeline with real-time progress tracking
- 💾 **Secure Storage**: AWS S3 integration for reliable file storage and delivery
- 🎯 **User-Friendly**: Clean, responsive interface built with vanilla JavaScript and TailwindCSS
- 📱 **Multi-Tier Service**: Freemium model with time-based processing limits

## Why?

### The Problem
Content creators across various industries face a common challenge: **inconsistent audio levels**. Whether you're a:
- 🎧 **DJ** creating seamless mixes
- 🎙️ **Podcaster** ensuring consistent episode volumes
- 🎵 **Music Producer** preparing tracks for different platforms
- 🎬 **Video Editor** balancing audio across clips

You've likely encountered the tedious process of manually adjusting audio levels to meet platform requirements or maintain professional quality standards.

### The Solution
LevelMix automates this technical process, allowing creators to:
- **Save Time**: No more manual audio editing or guesswork
- **Ensure Consistency**: Meet industry standards for streaming platforms, broadcasting, and club play
- **Focus on Creativity**: Spend time on content creation, not technical adjustments
- **Professional Results**: Achieve broadcast-quality audio normalization

## Architecture

This repo is **semi-open-source**. The `core/` and `pkg/` directories contain the application logic and compile on their own. Enterprise features (auth, payments, S3 storage, cleanup) live in a separate private `ee/` directory that is **not included** in this repo.

The codebase uses [Go build tags](https://pkg.go.dev/go/build#hdr-Build_Constraints) to handle this:

- `go build ./core/...` compiles with CE stubs (prints "rebuild with `-tags ee`")
- `go build -tags ee ./core/...` compiles with full enterprise wiring

If you clone this repo and want to run it, you need to provide your own `ee/` directory implementing the storage, auth, and payment interfaces. See [Enterprise setup](#enterprise-setup) below.

## Quick Start

### Prerequisites
- Go 1.24 or higher
- FFmpeg installed on your system
- Redis server (for job queue)
- AWS S3 bucket (for file storage)
- Turso database account

### Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/simonlewi/levelmix.git
   cd levelmix
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Set up environment variables**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

4. **Set up the database**
   ```bash
   # Create your Turso database
   turso db create levelmix-dev

   # Apply the database schema (located in your ee/ directory)
   turso db shell levelmix-dev < ee/storage/sql/schema.sql

   # Get your database URL and token
   turso db show levelmix-dev
   # Update your .env file with the connection details
   ```

5. **Start Redis (if running locally)**
   ```bash
   redis-server
   ```

6. **Start the application**
   ```bash
   go run -tags ee ./core/cmd/server
   ```

7. **Start the worker (in a separate terminal)**
   ```bash
   go run -tags ee ./core/cmd/worker
   ```

8. **Visit the application**
   Open your browser to `http://localhost:8080`

### Enterprise setup

The `ee/` directory is not included in this repo. To run the full application, you need to create your own implementations of:

- `ee/storage/` — `AudioStorage` and `MetadataStorage` (see `pkg/storage/interfaces.go`)
- `ee/auth/` — Authentication middleware and handlers
- `ee/payment/` — Payment processing (optional)
- `ee/cleanup/` — S3 lifecycle and consent cleanup (optional)

Each `core/cmd/*/run_ee.go` file shows exactly which `ee/` packages are imported and how they're wired up. Use those as your reference for what to implement.

### Environment Variables

Create a `.env` file in the project root with the following configuration:

```env
# Application Settings
APP_URL=http://localhost:8080
PORT=8080
GIN_MODE=debug
SESSION_SECRET=your-very-long-random-session-secret-here

# Database (Turso)
TURSO_DB_URL=libsql://your-database.turso.io
TURSO_AUTH_TOKEN=your-turso-auth-token

# Storage (AWS S3)
AWS_REGION=us-east-1
AWS_S3_BUCKET=levelmix-audio-files
AWS_ACCESS_KEY_ID=your-aws-access-key
AWS_SECRET_ACCESS_KEY=your-aws-secret-key

# Queue (Redis)
REDIS_URL=redis://localhost:6379

# Email Service (Resend)
EMAIL_SERVICE=resend
RESEND_API_KEY=your-resend-api-key
EMAIL_FROM=your-email-address-here
EMAIL_FROM_NAME=YourName
```

## Usage

### Web Interface

1. **Upload Audio File**
   - Visit the LevelMix homepage
   - Drag and drop your audio file or click to select from your computer

2. **Choose Target Level**
   - Select from preset LUFS targets:
     - **DJ Mix** (-5 LUFS): High-energy for club systems
     - **Streaming** (-14 LUFS): Perfect for Spotify, Apple Music, etc.
     - **Podcast** (-16 LUFS): Optimized for podcast platforms
     - **Broadcast** (-23 LUFS): EBU R128 standard for TV/radio
     - **Custom LUFS** (Premium/Pro only): Set your own target level

3. **Process & Download**
   - Monitor real-time processing progress
   - Download the processed file when complete
   - Access your processing history in the dashboard (registered users)

### Subscription Tiers

- **Free**: 2 hours processing/month, standard queue, all presets, mp3 only
- **Premium**: 10 hours processing/month, fast queue, custom LUFS, WAV support
- **Professional**: 40 hours processing/month, priority processing, all formats (MP3, WAV, FLAC)

## Contributing

We welcome contributions to LevelMix! Here's how you can help:

### Development Setup

1. **Fork the repository** and create your feature branch
   ```bash
   git checkout -b feature/amazing-feature
   ```

2. **Set up your development environment**
   ```bash
   # Install development dependencies
   go mod download
   
   # Install golangci-lint for code quality
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   
   # Run tests
   go test ./...
   ```

3. **Make your changes**
   - Follow Go best practices and conventions
   - Add tests for new functionality
   - Update documentation as needed

4. **Test your changes**
   ```bash
   # Run all tests
   go test ./...
   
   # Run with race detection
   go test -race ./...
   
   # Check formatting
   go fmt ./...
   
   # Run linting
   golangci-lint run
   ```

5. **Submit a Pull Request**
   - Ensure all tests pass
   - Include a clear description of your changes
   - Reference any related issues

### Project Structure

```
levelmix/
├── core/                    # Core application (open source)
│   ├── cmd/
│   │   ├── server/         # Web server (main.go + run_ee.go/run_ce.go)
│   │   ├── worker/         # Background audio processor
│   │   └── cleanup/        # S3 lifecycle cleanup job
│   ├── internal/
│   │   ├── audio/          # Audio processing logic
│   │   └── handlers/       # HTTP request handlers
│   ├── static/             # Static assets (CSS, JS, images)
│   └── templates/          # HTML templates
├── ee/                     # Enterprise features (not included, private)
│   ├── auth/              # Authentication system
│   ├── cleanup/           # S3 and consent cleanup
│   ├── payment/           # Payment processing
│   └── storage/           # S3 + Turso implementations
├── pkg/                   # Shared packages (open source)
│   ├── email/             # Email service
│   └── storage/           # Storage interfaces and models
├── go.mod                 # Go module definition
└── README.md
```

### Areas for Contribution

- 🐛 **Bug fixes** and performance improvements
- 📚 **Documentation** enhancements
- 🎨 **UI/UX** improvements
- 🔧 **New audio formats** support (FLAC, AAC, etc.)
- 🚀 **Performance optimizations**
- 🧪 **Testing** coverage expansion
- 🔒 **Security** enhancements
- 🎛️ **Additional audio processing features**

### Code Style

- Follow standard Go formatting (`go fmt`)
- Use meaningful variable and function names
- Add comments for complex logic
- Keep functions small and focused
- Write tests for new functionality
- Follow the existing project structure

### Getting Help

- 📋 **Issues**: Report bugs or request features via GitHub Issues
- 💬 **Discussions**: Join community discussions for questions and ideas
- 📧 **Contact**: Reach out to maintainers for major contributions

## Development

### Running in Development Mode

1. **Start Redis**
   ```bash
   redis-server
   ```

2. **Start the web server**
   ```bash
   go run -tags ee ./core/cmd/server
   ```

3. **Start the worker (separate terminal)**
   ```bash
   go run -tags ee ./core/cmd/worker
   ```

### Testing Audio Processing

1. Create a test MP3 file or use any existing audio file
2. Upload through the web interface at `http://localhost:8080/upload`
3. Monitor the processing in the worker logs
4. Download the normalized result

### Database Management

The application uses Turso (SQLite-compatible) as its database. The schema is in `ee/storage/sql/schema.sql`.

```bash
# Connect to your database
turso db shell your-database-name

# Run a query
SELECT * FROM users LIMIT 5;

# View tables
.tables
```

---

**Made with ❤️ for content creators everywhere**

*LevelMix - Making professional audio normalization accessible to everyone*