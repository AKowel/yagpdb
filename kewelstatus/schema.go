package kewelstatus

var DBSchemas = []string{`
CREATE TABLE IF NOT EXISTS kewelstatus_configs (
	guild_id BIGINT PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
	updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),

	status_channel BIGINT NOT NULL DEFAULT 0,
	notify_channel BIGINT NOT NULL DEFAULT 0,
	update_interval_secs INT NOT NULL DEFAULT 60,

	radarr_url TEXT NOT NULL DEFAULT '',
	radarr_api_key TEXT NOT NULL DEFAULT '',
	sonarr_url TEXT NOT NULL DEFAULT '',
	sonarr_api_key TEXT NOT NULL DEFAULT '',
	queue_api_url TEXT NOT NULL DEFAULT '',

	notifications JSONB NOT NULL DEFAULT '{}'::jsonb,

	status_message_id BIGINT NOT NULL DEFAULT 0,
	drives_presence BOOLEAN NOT NULL DEFAULT false
);
`, `
CREATE INDEX IF NOT EXISTS idx_kewelstatus_configs_updated_at ON kewelstatus_configs(updated_at);
`}
