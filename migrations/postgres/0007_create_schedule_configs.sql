CREATE TABLE IF NOT EXISTS schedule_configs_v2 (
	name TEXT PRIMARY KEY,
	cron_expression TEXT NOT NULL,
	enabled BOOLEAN NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);
