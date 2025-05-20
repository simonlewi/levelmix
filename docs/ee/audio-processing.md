# levelmix.io - Audio Processing Pipeline

## Overview
This document details the audio processing pipeline for LevelMix, which normalizes DJ mixes to specified LUFS target levels. The pipeline is implemented in Go with FFmpeg as the core audio processing engine.

## Pipeline Stages

### 1. Upload
- **Input**: MP3 file from user (max 300MB)
- **Process**:
  - File is uploaded directly to temporary storage
  - Server validates file format and size
  - Upon validation, file is moved to S3 storage
- **Output**: Valid file stored in S3, job created in database

### 2. Validation
- **Input**: Uploaded MP3 file
- **Process**:
  - Verify file is a valid MP3
  - Check for corruption or format issues
  - Validate duration (reject files that are too short or too long)
  - Check audio properties (sample rate, bit rate, channels)
- **Output**: Validated MP3 file or error message

### 3. Analysis
- **Input**: Validated MP3 file
- **Process**:
  - Measure existing LUFS level using FFmpeg with ebur128 filter
  - Extract audio metadata (duration, sample rate, bit rate, channels)
  - Determine gain adjustment needed to reach target LUFS
- **Output**: Analysis data stored in job record

### 4. Load Balancing
- **Input**: Analysis results and job metadata
- **Process**:
  - Distribute processing jobs across multiple worker instances
  - Consider system load, worker availability, and resource capacity
  - Route jobs based on complexity (file size, processing requirements)
  - Implement health checks to avoid routing to failing nodes
  - Apply dynamic scaling for worker pools during high demand
- **Output**: Job assigned to optimal processing node

### 5. Normalization
- **Input**: Analyzed MP3 file and target LUFS value
- **Process**:
  - Apply gain adjustment using FFmpeg's loudnorm filter
  - Maintain audio quality during processing
  - Update job progress percentage during processing
- **Output**: Normalized MP3 file at target LUFS level

### 6. Storage
- **Input**: Normalized MP3 file
- **Process**:
  - Upload processed file to S3
  - Generate pre-signed URL for download
  - Update job status to completed
- **Output**: Stored file with download URL

### 7. Download
- **Input**: Request for processed file
- **Process**:
  - Generate time-limited pre-signed URL
  - Redirect user to download URL
- **Output**: File download to user

## FFmpeg Commands

### Analysis Command
```bash
ffmpeg -i input.mp3 -af ebur128=metadata=1 -f null -
```

### Normalization Command
```bash
ffmpeg -i input.mp3 -af loudnorm=I={target_lufs}:TP=-1.0:LRA=11 -c:a libmp3lame -q:a 0 output.mp3
```

## Progress Tracking
1. Method: FFmpeg progress can be tracked by parsing its output
2. Implementation:
   - Use FFmpeg's progress output to stderr
   - Parse output to extract duration and time information
   - Calculate percentage completion
   - Update job record with current percentage
   - Broadcast updates through server-sent events

## Queue System
The processing pipeline will be managed by a job queue to handle multiple requests efficiently:
1. Job Creation: When a file is uploaded, a job is created and placed in the queue
2. Worker Assignment: Available workers pick up jobs from the queue
3. Priority Handling: Premium users' jobs are given higher priority
4. Concurrency Control: Limit concurrent jobs based on server resources
5. Failed Job Handling: Detect and notify about failed jobs

For Go implementation, Asynq with Redis backend is recommended for its simplicity and reliability.

## Load Balancing System
The load balancing system ensures optimal distribution of processing workloads:

1. **Distribution Strategies**:
   - Round-robin for even distribution across workers
   - Least connections to prioritize less busy workers
   - Resource-aware routing based on CPU/memory availability
   - Weighted distribution for heterogeneous worker pools

2. **Health Monitoring**:
   - Active health checks to detect worker failures
   - Passive monitoring of job completion rates
   - Automatic removal of failing nodes
   - Recovery detection for reintegration of repaired nodes

3. **Scaling Mechanisms**:
   - Horizontal scaling during peak demand periods
   - Auto-scaling based on queue depth
   - Worker pool segregation for different job priorities
   - Graceful scaling down during low demand

4. **Optimization Features**:
   - Job affinity for similar processing tasks
   - Predictive routing based on historical performance
   - Specialized worker pools for different file sizes/complexities
   - Geographic distribution for edge processing

## Error Handling
The pipeline includes comprehensive error handling:
1. Validation Errors: Caught early and reported to the user
2. Processing Errors: Logged and reported via notifications
3. Resource Exhaustion: Graceful handling of insufficient resources
4. Timeouts: Handle jobs that take too long
5. Load Balancer Failures: Automatic failover to secondary balancers

## Monitoring and Metrics
The pipeline tracks and records:
1. Processing Time: Total time taken to process each file
2. Success Rate: Percentage of successfully processed files
3. Queue Length: Current number of jobs waiting for processing
4. Resource Utilization: CPU, memory usage during processing
5. User-specific Metrics: Each user's processing history and statistics
6. Load Distribution: Balance of work across processing nodes
7. Node Performance: Individual worker throughput and reliability

## Future Enhancements
1. Additional Audio Formats: Support for WAV, FLAC, etc.
2. Batch Processing: Process multiple files in a single job
3. Advanced Audio Analysis: Detailed audio quality metrics
4. Custom Processing Options: Additional audio processing parameters
5. Music Recognition Integration: Track identification and timestamp generation
6. Predictive Load Balancing: AI-based job routing for optimal performance
7. Edge Processing: Distributed processing nodes for geographic optimization

## Testing Strategy
1. Unit Tests: Test individual components in isolation
2. Integration Tests: Verify pipeline stages work together correctly
3. Performance Tests: Measure processing time and resource usage
4. Load Tests: Verify system handles multiple concurrent jobs
5. Error Case Tests: Validate proper handling of various error conditions
6. Failover Tests: Verify load balancer handles node failures gracefully
7. Scaling Tests: Confirm system scales properly under varying loads