CREATE TABLE IF NOT EXISTS raw_snapshots (
	id TEXT PRIMARY KEY,
	job_id TEXT NOT NULL REFERENCES ingestion_jobs(id) ON DELETE CASCADE,
	attempt_number INTEGER NOT NULL,
	provider TEXT NOT NULL,
	pipeline TEXT NOT NULL,
	resource_type TEXT NOT NULL,
	resource_key TEXT NOT NULL,
	source_url TEXT,
	content_type TEXT,
	status_code INTEGER NOT NULL,
	parser_version TEXT NOT NULL,
	captured_at TIMESTAMPTZ NOT NULL,
	sha256 TEXT NOT NULL,
	body BYTEA NOT NULL,
	metadata_json BYTEA,
	UNIQUE (job_id, attempt_number, resource_type, resource_key, sha256)
);
-- migrate:split
CREATE INDEX IF NOT EXISTS idx_raw_snapshots_job_captured_at
	ON raw_snapshots (job_id, captured_at DESC);
