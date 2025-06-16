package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

type TursoStorage struct {
	db *sql.DB
}

func NewTursoStorage(connStr, authToken string) (*TursoStorage, error) {
	db, err := sql.Open("libsql", connStr+"?authToken="+authToken)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &TursoStorage{db: db}, nil
}

func (t *TursoStorage) CreateAudioFile(ctx context.Context, file *AudioFile) error {
	query := `
		INSERT INTO audio_files (id, user_id, original_filename, s3_key, file_size, format, status, lufs_target, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	_, err := t.db.ExecContext(ctx, query,
		file.ID, file.UserID, file.OriginalFilename, file.ID, file.FileSize,
		file.Format, file.Status, file.LUFSTarget, now, now)

	return err
}

func (t *TursoStorage) GetAudioFile(ctx context.Context, fileID string) (*AudioFile, error) {
	query := `
		SELECT id, user_id, original_filename, s3_key, file_size, format, status, lufs_target, created_at
		FROM audio_files WHERE id = ?
	`

	var file AudioFile
	err := t.db.QueryRowContext(ctx, query, fileID).Scan(
		&file.ID, &file.UserID, &file.OriginalFilename, &file.ID,
		&file.FileSize, &file.Format, &file.Status, &file.LUFSTarget, &file.CreatedAt)

	if err != nil {
		return nil, err
	}

	return &file, nil
}

func (t *TursoStorage) UpdateStatus(ctx context.Context, fileID string, status string) error {
	query := `UPDATE audio_files SET status = ?, updated_at = ? WHERE id = ?`
	_, err := t.db.ExecContext(ctx, query, status, time.Now(), fileID)
	return err
}

func (t *TursoStorage) CreateJob(ctx context.Context, job *ProcessingJob) error {
	query := `
		INSERT INTO processing_jobs (id, audio_file_id, status, created_at)
		VALUES (?, ?, ?, ?)
	`
	_, err := t.db.ExecContext(ctx, query, job.ID, job.AudioFileID, job.Status, time.Now())
	return err
}

func (t *TursoStorage) UpdateJobStatus(ctx context.Context, jobID, status string, errorMsg *string) error {
	query := `UPDATE processing_jobs SET status = ?, error_message = ?, updated_at = ? WHERE id = ?`
	_, err := t.db.ExecContext(ctx, query, status, errorMsg, time.Now(), jobID)
	return err
}

func (t *TursoStorage) GetJobByFileID(ctx context.Context, fileID string) (*ProcessingJob, error) {
	query := `
		SELECT id, audio_file_id, status, error_message, output_s3_key, started_at, completed_at, created_at
		FROM processing_jobs WHERE audio_file_id = ? ORDER BY created_at DESC LIMIT 1
	`

	var job ProcessingJob
	var startedAt, completedAt sql.NullTime
	var outputKey, errorMsg sql.NullString

	err := t.db.QueryRowContext(ctx, query, fileID).Scan(
		&job.ID, &job.AudioFileID, &job.Status, &errorMsg, &outputKey,
		&startedAt, &completedAt, &job.CreatedAt)

	if err != nil {
		return nil, err
	}

	if errorMsg.Valid {
		job.ErrorMessage = errorMsg.String
	}

	return &job, nil
}
