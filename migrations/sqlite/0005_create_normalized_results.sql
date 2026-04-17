CREATE TABLE IF NOT EXISTS normalized_results (
	id TEXT PRIMARY KEY,
	job_id TEXT NOT NULL REFERENCES ingestion_jobs(id) ON DELETE CASCADE,
	attempt_number INTEGER NOT NULL,
	provider TEXT NOT NULL,
	pipeline TEXT NOT NULL,
	parser_version TEXT NOT NULL,
	normalization_version TEXT NOT NULL,
	payload_hash TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	metadata_json BLOB,
	payload_json BLOB NOT NULL,
	UNIQUE (job_id)
);
-- migrate:split
CREATE INDEX IF NOT EXISTS idx_normalized_results_pipeline_created_at
	ON normalized_results (pipeline, created_at DESC);
-- migrate:split
CREATE INDEX IF NOT EXISTS idx_normalized_results_payload_hash
	ON normalized_results (payload_hash);
