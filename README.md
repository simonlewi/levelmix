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

4. **Set up the database**
   ```bash
   # Create your Turso database
   turso db create levelmix-dev
   
   # Apply the database schema
   turso db shell levelmix-dev < schema.sql
   
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
   go run core/cmd/server/main.go
   ```

7. **Start the worker (in a separate terminal)**
   ```bash
   go run core/cmd/worker/main.go
   ```

8. **Visit the application**
   Open your browser to `http://localhost:8080`

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

# Monitoring (optional but recommended)
SENTRY_DSN=your-sentry-dsn
DATADOG_API_KEY=your-datadog-api-key
```

## Usage

### Web Interface

1. **Upload Audio File**
   - Visit the LevelMix homepage
   - Drag and drop your MP3 file (up to 300MB for free users)
   - Or click to select file from your computer

2. **Choose Target Level**
   - Select from preset LUFS targets:
     - **Streaming** (-14 LUFS): Perfect for Spotify, Apple Music, etc.
     - **Podcast** (-16 LUFS): Optimized for podcast platforms
     - **Radio** (-23 LUFS): EBU R128 standard for TV/radio
     - **Club Mix** (-7 LUFS): High-energy for club systems
     - **Festival Mix** (-5 LUFS): Maximum impact EDM masters
     - **Custom LUFS** (Premium/Pro only): Set your own target level

3. **Process & Download**
   - Monitor real-time processing progress
   - Download the processed file when complete
   - Access your processing history in the dashboard (registered users)

### Subscription Tiers

- **Free Tier**: 1 upload per week, MP3 format only, up to 300MB
- **Premium Tier**: 5 uploads per week, MP3 + WAV support, priority processing, custom LUFS targets
- **Professional Tier**: 20 uploads per week, multiple formats, batch processing, priority support

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
├── core/                    # Core application
│   ├── cmd/
│   │   ├── server/         # Main web server
│   │   └── worker/         # Background audio processor
│   ├── internal/
│   │   ├── audio/          # Audio processing logic
│   │   └── handlers/       # HTTP request handlers
│   ├── static/             # Static assets (CSS, images)
│   └── templates/          # HTML templates
├── ee/                     # Enterprise Edition features
│   ├── auth/              # Authentication system
│   └── storage/           # Storage implementations
├── pkg/                   # Shared packages
│   ├── email/             # Email service
│   └── storage/           # Storage interfaces
├── deployments/           # Deployment configurations
├── scripts/               # Utility scripts
├── go.mod                 # Go module definition
├── schema.sql             # Database schema
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
   go run core/cmd/server/main.go
   ```

3. **Start the worker (separate terminal)**
   ```bash
   go run core/cmd/worker/main.go
   ```

### Testing Audio Processing

1. Create a test MP3 file or use any existing audio file
2. Upload through the web interface at `http://localhost:8080/upload`
3. Monitor the processing in the worker logs
4. Download the normalized result

### Database Management

The application uses Turso (SQLite-compatible) as its database. The schema is defined in `schema.sql`:

```bash
# Connect to your database
turso db shell your-database-name

# Run a query
SELECT * FROM users LIMIT 5;

# View tables
.tables

# View schema for a table
.schema users
```

---

**Made with ❤️ for content creators everywhere**

*LevelMix - Making professional audio normalization accessible to everyone*