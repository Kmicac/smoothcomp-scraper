CREATE TABLE IF NOT EXISTS job_state_transitions (
	id TEXT PRIMARY KEY,
	job_id TEXT NOT NULL REFERENCES ingestion_jobs(id) ON DELETE CASCADE,
	attempt_number INTEGER,
	from_state TEXT,
	to_state TEXT NOT NULL,
	reason TEXT NOT NULL,
	metadata_json BLOB,
	created_at DATETIME NOT NULL
);
-- migrate:split
CREATE INDEX IF NOT EXISTS idx_job_state_transitions_job_created_at
	ON job_state_transitions (job_id, created_at DESC);
