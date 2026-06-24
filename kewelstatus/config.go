package kewelstatus

import (
	"database/sql"
	"encoding/json"

	"github.com/botlabs-gg/yagpdb/v2/common"
)

type NotificationConfig struct {
	Enabled bool          `json:"enabled"`
	Channel int64         `json:"channel"`
	Embed   EmbedTemplate `json:"embed"`
}

type Notifications struct {
	DownloadComplete NotificationConfig `json:"downloadComplete"`
	DownloadWarning  NotificationConfig `json:"downloadWarning"`
	NewDownload      NotificationConfig `json:"newDownload"`
	ServiceDown      NotificationConfig `json:"serviceDown"`
	ServiceUp        NotificationConfig `json:"serviceUp"`
}

func DefaultNotifications() Notifications {
	return Notifications{
		DownloadComplete: NotificationConfig{
			Enabled: true,
			Embed: EmbedTemplate{
				Color: "#22c55e", Title: "✅ {title} downloaded", Description: "{subtitle}",
				FooterText: "Kewel Media", Timestamp: true,
			},
		},
		DownloadWarning: NotificationConfig{
			Enabled: true,
			Embed: EmbedTemplate{
				Color: "#f59e0b", Title: "⚠️ {title}", Description: "{messages}",
				FooterText: "Kewel Media", Timestamp: true,
			},
		},
		NewDownload: NotificationConfig{
			Enabled: false,
			Embed: EmbedTemplate{
				Color: "#6366f1", Title: "⬇️ Now downloading: {title}", Description: "{subtitle}",
				FooterText: "Kewel Media", Timestamp: true,
			},
		},
		ServiceDown: NotificationConfig{
			Enabled: true,
			Embed: EmbedTemplate{
				Color: "#ef4444", Title: "🔴 {service} is offline", Description: "{error}",
				FooterText: "Kewel Media", Timestamp: true,
			},
		},
		ServiceUp: NotificationConfig{
			Enabled: true,
			Embed: EmbedTemplate{
				Color: "#22c55e", Title: "🟢 {service} is back online",
				FooterText: "Kewel Media", Timestamp: true,
			},
		},
	}
}

type Config struct {
	GuildID            int64
	StatusChannel      int64
	NotifyChannel      int64
	UpdateIntervalSecs int
	RadarrURL          string
	RadarrAPIKey       string
	SonarrURL          string
	SonarrAPIKey       string
	QueueAPIURL        string
	Notifications      Notifications
	StatusMessageID    int64
	DrivesPresence     bool
}

const configColumns = `guild_id, status_channel, notify_channel, update_interval_secs,
	radarr_url, radarr_api_key, sonarr_url, sonarr_api_key, queue_api_url,
	notifications, status_message_id, drives_presence`

func scanConfig(scanner interface {
	Scan(dest ...interface{}) error
}) (*Config, error) {
	cfg := &Config{}
	var notifRaw []byte
	err := scanner.Scan(&cfg.GuildID, &cfg.StatusChannel, &cfg.NotifyChannel, &cfg.UpdateIntervalSecs,
		&cfg.RadarrURL, &cfg.RadarrAPIKey, &cfg.SonarrURL, &cfg.SonarrAPIKey, &cfg.QueueAPIURL,
		&notifRaw, &cfg.StatusMessageID, &cfg.DrivesPresence)
	if err != nil {
		return nil, err
	}

	cfg.Notifications = DefaultNotifications()
	if len(notifRaw) > 2 {
		if err := json.Unmarshal(notifRaw, &cfg.Notifications); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

// FetchConfig returns the stored config for a guild, or nil if none exists yet.
func FetchConfig(guildID int64) (*Config, error) {
	row := common.PQ.QueryRow(`SELECT `+configColumns+` FROM kewelstatus_configs WHERE guild_id = $1`, guildID)
	cfg, err := scanConfig(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// GetOrDefaultConfig returns the stored config, or an unsaved default config if none exists.
func GetOrDefaultConfig(guildID int64) (*Config, error) {
	cfg, err := FetchConfig(guildID)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = &Config{GuildID: guildID, UpdateIntervalSecs: 60, Notifications: DefaultNotifications()}
	}
	return cfg, nil
}

// AllConfigs returns every guild's stored config, for the background worker to sweep.
func AllConfigs() ([]*Config, error) {
	rows, err := common.PQ.Query(`SELECT ` + configColumns + ` FROM kewelstatus_configs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Config
	for rows.Next() {
		cfg, err := scanConfig(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, cfg)
	}
	return result, rows.Err()
}

func (c *Config) Save() error {
	notifRaw, err := json.Marshal(c.Notifications)
	if err != nil {
		return err
	}

	_, err = common.PQ.Exec(`INSERT INTO kewelstatus_configs
		(guild_id, status_channel, notify_channel, update_interval_secs,
		 radarr_url, radarr_api_key, sonarr_url, sonarr_api_key, queue_api_url,
		 notifications, status_message_id, drives_presence, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11, $12, now())
		ON CONFLICT (guild_id) DO UPDATE SET
			status_channel = $2, notify_channel = $3, update_interval_secs = $4,
			radarr_url = $5, radarr_api_key = $6, sonarr_url = $7, sonarr_api_key = $8, queue_api_url = $9,
			notifications = $10::jsonb, status_message_id = $11, drives_presence = $12, updated_at = now()`,
		c.GuildID, c.StatusChannel, c.NotifyChannel, c.UpdateIntervalSecs,
		c.RadarrURL, c.RadarrAPIKey, c.SonarrURL, c.SonarrAPIKey, c.QueueAPIURL,
		string(notifRaw), c.StatusMessageID, c.DrivesPresence)
	return err
}

// SaveStatusMessageID does a lightweight update of just the message id, so a stale
// in-memory config copy from a slow poll cycle doesn't clobber concurrent dashboard edits.
func (c *Config) SaveStatusMessageID() error {
	_, err := common.PQ.Exec(`UPDATE kewelstatus_configs SET status_message_id = $1, updated_at = now() WHERE guild_id = $2`,
		c.StatusMessageID, c.GuildID)
	return err
}
