ALTER TABLE normalized_results
	ADD COLUMN IF NOT EXISTS scope_key TEXT,
	ADD COLUMN IF NOT EXISTS contract_version TEXT,
	ADD COLUMN IF NOT EXISTS source_snapshot_hash TEXT,
	ADD COLUMN IF NOT EXISTS publication_decision TEXT,
	ADD COLUMN IF NOT EXISTS publication_reason TEXT,
	ADD COLUMN IF NOT EXISTS change_classification TEXT,
	ADD COLUMN IF NOT EXISTS forced_republish BOOLEAN NOT NULL DEFAULT FALSE;
-- migrate:split
ALTER TABLE published_results
	ADD COLUMN IF NOT EXISTS scope_key TEXT,
	ADD COLUMN IF NOT EXISTS correlation_id TEXT,
	ADD COLUMN IF NOT EXISTS parser_version TEXT,
	ADD COLUMN IF NOT EXISTS normalization_version TEXT,
	ADD COLUMN IF NOT EXISTS source_snapshot_hash TEXT,
	ADD COLUMN IF NOT EXISTS normalized_hash TEXT,
	ADD COLUMN IF NOT EXISTS publication_decision TEXT,
	ADD COLUMN IF NOT EXISTS publication_reason TEXT,
	ADD COLUMN IF NOT EXISTS change_classification TEXT,
	ADD COLUMN IF NOT EXISTS forced_republish BOOLEAN NOT NULL DEFAULT FALSE,
	ADD COLUMN IF NOT EXISTS supersedes_publication_id TEXT;
-- migrate:split
CREATE INDEX IF NOT EXISTS idx_normalized_results_pipeline_scope_created_at
	ON normalized_results (pipeline, scope_key, created_at DESC);
-- migrate:split
CREATE INDEX IF NOT EXISTS idx_published_results_pipeline_scope_published_at
	ON published_results (pipeline, scope_key, published_at DESC);
