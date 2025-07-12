# LevelMix

A web-based SaaS application that normalizes longer audio files to specified LUFS target levels, making audio content consistent and professional.

## Description

LevelMix is a powerful yet simple audio normalization service designed for content creators who need to ensure consistent loudness levels across their audio content. Built with Go and HTMX, it provides a fast, efficient way to process audio files without the need for complex software or technical expertise.

**Key Features:**
- 🎵 **Audio Normalization**: Automatically normalize audio files to industry-standard LUFS levels
- 📊 **Multiple Presets**: Choose from streaming (-14 LUFS), broadcast (-23 LUFS), or EDM club-ready (-7 LUFS) presets
- 🚀 **Fast Processing**: Efficient FFmpeg-based processing pipeline with real-time progress tracking
- 💾 **Secure Storage**: AWS S3 integration for reliable file storage and delivery
- 🎯 **User-Friendly**: Clean, responsive interface built with HTMX and TailwindCSS
- 📱 **Multi-Tier Service**: Freemium model with options for different user needs

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

## Quick Start

### Prerequisites
- Go 1.21 or higher
- FFmpeg installed on your system
- Redis server (for job queue)
- AWS S3 bucket (for file storage)
- Turso database account

### Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/yourusername/levelmix.git
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

4. **Run database migrations**
   ```bash
   go run cmd/migrate/main.go
   ```

5. **Start the application**
   ```bash
   go run cmd/server/main.go
   ```

6. **Visit the application**
   Open your browser to `http://localhost:8080`

### Environment Variables

```env
# Database
DATABASE_URL=your_turso_database_url
DATABASE_TOKEN=your_turso_token

# AWS S3
AWS_ACCESS_KEY_ID=your_aws_access_key
AWS_SECRET_ACCESS_KEY=your_aws_secret_key
AWS_REGION=your_aws_region
AWS_BUCKET_NAME=your_s3_bucket

# Redis
REDIS_URL=redis://localhost:6379

# Authentication
JWT_SECRET=your_jwt_secret
GOOGLE_CLIENT_ID=your_google_oauth_client_id
GOOGLE_CLIENT_SECRET=your_google_oauth_client_secret

# Application
PORT=8080
ENVIRONMENT=development
```

## Usage

### Web Interface

1. **Upload Audio File**
   - Visit the LevelMix homepage
   - Drag and drop your MP3 file (up to 300MB)
   - Or click to select file from your computer

2. **Choose Target Level**
   - Select from preset LUFS targets:
     - **Streaming** (-14 LUFS): Perfect for Spotify, Apple Music, etc.
     - **Broadcast** (-23 LUFS): EBU R128 standard for TV/radio
     - **Club Ready** (-7 LUFS): High-energy EDM for club systems
     - **Max Impact** (-5 LUFS): Very loud EDM masters

3. **Process & Download**
   - Monitor real-time processing progress
   - Preview your normalized audio
   - Download the processed file

### API Usage

LevelMix also provides a REST API for programmatic access:

```bash
# Upload and process a file
curl -X POST \
  -F "file=@your-audio.mp3" \
  -F "target_lufs=-14.0" \
  http://localhost:8080/api/v1/files/upload

# Check processing status
curl http://localhost:8080/api/v1/jobs/{job_id}/status

# Download processed file
curl http://localhost:8080/api/v1/jobs/{job_id}/download
```

### Subscription Tiers

- **Free Tier**: 1 upload per month, MP3 format only
- **Premium Tier**: 4 uploads per month, MP3 + WAV support, priority processing
- **Professional Tier**: Unlimited uploads, multiple formats, batch processing, API access

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
   
   # Install pre-commit hooks
   pre-commit install
   
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
├── LICENSE
├── build/              # Compiled binaries
│   └── levelmix
├── build.sh           # Build script
├── config/            # Configuration files
├── core/              # Core (Community Edition) functionality
│   ├── cmd/           # Core command-line applications
│   ├── internal/      # Core internal packages
│   ├── static/        # Core static assets
│   ├── templates/     # Core HTML templates
│   └── tmp/           # Temporary files
├── deployments/       # Deployment configurations
│   ├── docker/        # Docker configurations
│   └── k8s/          # Kubernetes configurations
├── docs/              # Documentation
│   ├── ce/           # Community Edition docs
│   └── ee/           # Enterprise Edition docs
├── ee/                # Enterprise Edition functionality
│   ├── cmd/           # Enterprise command-line applications
│   ├── internal/      # Enterprise internal packages
│   ├── static/        # Enterprise static assets
│   ├── storage/       # Enterprise storage configurations
│   └── templates/     # Enterprise HTML templates
├── migrations/        # Database migrations
│   ├── ce/           # Community Edition migrations
│   └── ee/           # Enterprise Edition migrations
├── pkg/               # Public packages
│   └── storage/       # Storage utilities
├── tests/             # Test files
│   ├── ce/           # Community Edition tests
│   └── ee/           # Enterprise Edition tests
├── go.mod             # Go module definition
├── go.sum             # Go module checksums
├── package.json       # Node.js dependencies (for TailwindCSS)
├── postcss.config.mjs # PostCSS configuration
└── tmp/               # Temporary build files
```

### Areas for Contribution

- 🐛 **Bug fixes** and performance improvements
- 📚 **Documentation** enhancements
- 🎨 **UI/UX** improvements
- 🔧 **New audio formats** support
- 🚀 **Performance optimizations**
- 🧪 **Testing** coverage expansion
- 🔒 **Security** enhancements

### Code Style

- Follow standard Go formatting (`go fmt`)
- Use meaningful variable and function names
- Add comments for complex logic
- Keep functions small and focused
- Write tests for new functionality

### Getting Help

- 📋 **Issues**: Report bugs or request features via GitHub Issues
- 💬 **Discussions**: Join community discussions for questions and ideas
- 📧 **Contact**: Reach out to maintainers for major contributions

---

**Made with ❤️ for content creators everywhere**

*LevelMix - Making professional audio normalization accessible to everyone*