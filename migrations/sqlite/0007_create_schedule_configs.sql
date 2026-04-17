CREATE TABLE IF NOT EXISTS schedule_configs_v2 (
	name TEXT PRIMARY KEY,
	cron_expression TEXT NOT NULL,
	enabled INTEGER NOT NULL,
	updated_at DATETIME NOT NULL
);
