ALTER TABLE normalized_results ADD COLUMN scope_key TEXT;
-- migrate:split
ALTER TABLE normalized_results ADD COLUMN contract_version TEXT;
-- migrate:split
ALTER TABLE normalized_results ADD COLUMN source_snapshot_hash TEXT;
-- migrate:split
ALTER TABLE normalized_results ADD COLUMN publication_decision TEXT;
-- migrate:split
ALTER TABLE normalized_results ADD COLUMN publication_reason TEXT;
-- migrate:split
ALTER TABLE normalized_results ADD COLUMN change_classification TEXT;
-- migrate:split
ALTER TABLE normalized_results ADD COLUMN forced_republish INTEGER NOT NULL DEFAULT 0;
-- migrate:split
ALTER TABLE published_results ADD COLUMN scope_key TEXT;
-- migrate:split
ALTER TABLE published_results ADD COLUMN correlation_id TEXT;
-- migrate:split
ALTER TABLE published_results ADD COLUMN parser_version TEXT;
-- migrate:split
ALTER TABLE published_results ADD COLUMN normalization_version TEXT;
-- migrate:split
ALTER TABLE published_results ADD COLUMN source_snapshot_hash TEXT;
-- migrate:split
ALTER TABLE published_results ADD COLUMN normalized_hash TEXT;
-- migrate:split
ALTER TABLE published_results ADD COLUMN publication_decision TEXT;
-- migrate:split
ALTER TABLE published_results ADD COLUMN publication_reason TEXT;
-- migrate:split
ALTER TABLE published_results ADD COLUMN change_classification TEXT;
-- migrate:split
ALTER TABLE published_results ADD COLUMN forced_republish INTEGER NOT NULL DEFAULT 0;
-- migrate:split
ALTER TABLE published_results ADD COLUMN supersedes_publication_id TEXT;
-- migrate:split
CREATE INDEX IF NOT EXISTS idx_normalized_results_pipeline_scope_created_at
	ON normalized_results (pipeline, scope_key, created_at DESC);
-- migrate:split
CREATE INDEX IF NOT EXISTS idx_published_results_pipeline_scope_published_at
	ON published_results (pipeline, scope_key, published_at DESC);
