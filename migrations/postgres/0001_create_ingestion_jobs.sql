CREATE TABLE IF NOT EXISTS ingestion_jobs (
	id TEXT PRIMARY KEY,
	provider TEXT NOT NULL,
	pipeline TEXT NOT NULL,
	trigger TEXT NOT NULL,
	state TEXT NOT NULL CHECK (state IN ('pending', 'running', 'succeeded', 'failed', 'exhausted')),
	correlation_id TEXT,
	parser_version TEXT NOT NULL,
	normalization_version TEXT NOT NULL,
	country TEXT,
	event_type TEXT,
	event_id TEXT,
	event_url TEXT,
	event_name TEXT,
	request_json BYTEA NOT NULL,
	stats_json BYTEA NOT NULL,
	error_json BYTEA,
	error_category TEXT,
	error_code TEXT,
	error_message TEXT,
	error_retryable BOOLEAN NOT NULL DEFAULT FALSE,
	attempt_count INTEGER NOT NULL DEFAULT 0,
	max_attempts INTEGER NOT NULL,
	claimed_by TEXT,
	claimed_at TIMESTAMPTZ,
	lease_until TIMESTAMPTZ,
	last_heartbeat_at TIMESTAMPTZ,
	next_retry_at TIMESTAMPTZ,
	last_transition_at TIMESTAMPTZ NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	started_at TIMESTAMPTZ,
	finished_at TIMESTAMPTZ
);
-- migrate:split
CREATE INDEX IF NOT EXISTS idx_ingestion_jobs_claim
	ON ingestion_jobs (state, next_retry_at, lease_until, created_at);
-- migrate:split
CREATE INDEX IF NOT EXISTS idx_ingestion_jobs_pipeline_created_at
	ON ingestion_jobs (pipeline, created_at DESC);
-- migrate:split
CREATE INDEX IF NOT EXISTS idx_ingestion_jobs_claimed_by
	ON ingestion_jobs (claimed_by);
