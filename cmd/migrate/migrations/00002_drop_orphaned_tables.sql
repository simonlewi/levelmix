-- Drops two tables that the baseline carried only because they existed in
-- production, neither of which anything reads.
--
-- processing_jobs_backup was a temporary copy taken to hold jobs across a
-- processing-pipeline update. Its rows are real but disposable, and the table
-- has no code references.
--
-- schema_migrations is the tracking table of the hand-rolled runner that goose
-- replaced in 00001; goose keeps its own state in goose_db_version. It is empty.

-- +goose Up

DROP TABLE IF EXISTS processing_jobs_backup;
DROP TABLE IF EXISTS schema_migrations;

-- +goose Down
-- Structure only: the dropped rows are not recoverable from here.

CREATE TABLE processing_jobs_backup (
    id TEXT,
    audio_file_id TEXT,
    status TEXT,
    error_message TEXT,
    output_s3_key TEXT,
    started_at NUM,
    completed_at NUM,
    created_at NUM,
    updated_at NUM,
    user_id TEXT,
    priority INT
);

CREATE TABLE schema_migrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    version TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_schema_migrations_version ON schema_migrations(version);
