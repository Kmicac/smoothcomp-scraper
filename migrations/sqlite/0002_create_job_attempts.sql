CREATE TABLE IF NOT EXISTS job_attempts (
	id TEXT PRIMARY KEY,
	job_id TEXT NOT NULL REFERENCES ingestion_jobs(id) ON DELETE CASCADE,
	attempt_number INTEGER NOT NULL,
	worker_id TEXT NOT NULL,
	state TEXT NOT NULL CHECK (state IN ('running', 'succeeded', 'failed')),
	error_json BLOB,
	error_category TEXT,
	error_code TEXT,
	error_message TEXT,
	error_retryable INTEGER NOT NULL DEFAULT 0,
	started_at DATETIME NOT NULL,
	finished_at DATETIME,
	last_heartbeat_at DATETIME,
	lease_until DATETIME,
	created_at DATETIME NOT NULL,
	UNIQUE (job_id, attempt_number)
);
-- migrate:split
CREATE INDEX IF NOT EXISTS idx_job_attempts_job_started_at
	ON job_attempts (job_id, started_at DESC);
-- migrate:split
CREATE INDEX IF NOT EXISTS idx_job_attempts_worker_id
	ON job_attempts (worker_id, started_at DESC);
