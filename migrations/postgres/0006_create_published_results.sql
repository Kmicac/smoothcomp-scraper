CREATE TABLE IF NOT EXISTS published_results (
	id TEXT PRIMARY KEY,
	job_id TEXT NOT NULL REFERENCES ingestion_jobs(id) ON DELETE CASCADE,
	attempt_number INTEGER NOT NULL,
	provider TEXT NOT NULL,
	pipeline TEXT NOT NULL,
	contract_version TEXT NOT NULL,
	checksum TEXT NOT NULL,
	published_at TIMESTAMPTZ NOT NULL,
	metadata_json BYTEA,
	payload_json BYTEA NOT NULL,
	UNIQUE (job_id)
);
-- migrate:split
CREATE INDEX IF NOT EXISTS idx_published_results_pipeline_published_at
	ON published_results (pipeline, published_at DESC);
-- migrate:split
CREATE INDEX IF NOT EXISTS idx_published_results_checksum
	ON published_results (checksum);
