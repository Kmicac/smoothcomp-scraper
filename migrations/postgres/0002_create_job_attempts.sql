CREATE TABLE IF NOT EXISTS job_attempts (
	id TEXT PRIMARY KEY,
	job_id TEXT NOT NULL REFERENCES ingestion_jobs(id) ON DELETE CASCADE,
	attempt_number INTEGER NOT NULL,
	worker_id TEXT NOT NULL,
	state TEXT NOT NULL CHECK (state IN ('running', 'succeeded', 'failed')),
	error_json BYTEA,
	error_category TEXT,
	error_code TEXT,
	error_message TEXT,
	error_retryable BOOLEAN NOT NULL DEFAULT FALSE,
	started_at TIMESTAMPTZ NOT NULL,
	finished_at TIMESTAMPTZ,
	last_heartbeat_at TIMESTAMPTZ,
	lease_until TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL,
	UNIQUE (job_id, attempt_number)
);
-- migrate:split
CREATE INDEX IF NOT EXISTS idx_job_attempts_job_started_at
	ON job_attempts (job_id, started_at DESC);
-- migrate:split
CREATE INDEX IF NOT EXISTS idx_job_attempts_worker_id
	ON job_attempts (worker_id, started_at DESC);
