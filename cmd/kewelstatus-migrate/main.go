// Standalone one-off importer for the legacy kewel-discord-bot config.json
// into a kewelstatus_configs row. Run once during cutover, then delete this
// directory — it isn't wired into the main yagpdb binary.
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/kewelstatus"
	_ "github.com/lib/pq"
)

type legacyNotificationConfig struct {
	Enabled bool   `json:"enabled"`
	Channel string `json:"channel"`
	Embed   struct {
		Color        string `json:"color"`
		AuthorName   string `json:"authorName"`
		AuthorIcon   string `json:"authorIcon"`
		Title        string `json:"title"`
		URL          string `json:"url"`
		Description  string `json:"description"`
		ThumbnailURL string `json:"thumbnailUrl"`
		ImageURL     string `json:"imageUrl"`
		FooterText   string `json:"footerText"`
		FooterIcon   string `json:"footerIcon"`
		Timestamp    bool   `json:"timestamp"`
		Fields       []struct {
			Name   string `json:"name"`
			Value  string `json:"value"`
			Inline bool   `json:"inline"`
		} `json:"fields"`
	} `json:"embed"`
}

type legacyConfig struct {
	Channels struct {
		Status        string `json:"status"`
		Notifications string `json:"notifications"`
	} `json:"channels"`
	UpdateIntervalSecs int `json:"updateIntervalSecs"`
	Notifications      struct {
		DownloadComplete legacyNotificationConfig `json:"downloadComplete"`
		DownloadWarning  legacyNotificationConfig `json:"downloadWarning"`
		NewDownload      legacyNotificationConfig `json:"newDownload"`
		ServiceDown      legacyNotificationConfig `json:"serviceDown"`
		ServiceUp        legacyNotificationConfig `json:"serviceUp"`
	} `json:"notifications"`
}

func convertNotification(legacy legacyNotificationConfig, fallback kewelstatus.NotificationConfig) kewelstatus.NotificationConfig {
	out := fallback
	out.Enabled = legacy.Enabled
	if legacy.Channel != "" {
		if id, err := strconv.ParseInt(legacy.Channel, 10, 64); err == nil {
			out.Channel = id
		}
	}
	if legacy.Embed.Title != "" || legacy.Embed.Description != "" {
		out.Embed.Color = legacy.Embed.Color
		out.Embed.AuthorName = legacy.Embed.AuthorName
		out.Embed.AuthorIcon = legacy.Embed.AuthorIcon
		out.Embed.Title = legacy.Embed.Title
		out.Embed.URL = legacy.Embed.URL
		out.Embed.Description = legacy.Embed.Description
		out.Embed.ThumbnailURL = legacy.Embed.ThumbnailURL
		out.Embed.ImageURL = legacy.Embed.ImageURL
		out.Embed.FooterText = legacy.Embed.FooterText
		out.Embed.FooterIcon = legacy.Embed.FooterIcon
		out.Embed.Timestamp = legacy.Embed.Timestamp
		for _, f := range legacy.Embed.Fields {
			out.Embed.Fields = append(out.Embed.Fields, kewelstatus.EmbedField{Name: f.Name, Value: f.Value, Inline: f.Inline})
		}
	}
	return out
}

func main() {
	configJSON := flag.String("config-json", "", "path to the legacy config.json")
	guildID := flag.Int64("guild-id", 0, "destination Discord guild ID")
	radarrURL := flag.String("radarr-url", "", "Radarr base URL")
	radarrAPIKey := flag.String("radarr-api-key", "", "Radarr API key")
	sonarrURL := flag.String("sonarr-url", "", "Sonarr base URL")
	sonarrAPIKey := flag.String("sonarr-api-key", "", "Sonarr API key")
	queueAPIURL := flag.String("queue-api-url", "", "kewel-queue API base URL")
	drivesPresence := flag.Bool("drives-presence", true, "let this guild drive the bot's rich presence")
	pqHost := flag.String("pq-host", "localhost", "Postgres host")
	pqUser := flag.String("pq-user", "postgres", "Postgres user")
	pqPass := flag.String("pq-pass", "", "Postgres password")
	pqDB := flag.String("pq-db", "yagpdb", "Postgres database")
	flag.Parse()

	if *configJSON == "" || *guildID == 0 {
		fmt.Fprintln(os.Stderr, "usage: kewelstatus-migrate -config-json=... -guild-id=... [-radarr-url=... -radarr-api-key=... -sonarr-url=... -sonarr-api-key=... -queue-api-url=...]")
		os.Exit(1)
	}

	raw, err := os.ReadFile(*configJSON)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed reading config.json:", err)
		os.Exit(1)
	}

	var legacy legacyConfig
	if err := json.Unmarshal(raw, &legacy); err != nil {
		fmt.Fprintln(os.Stderr, "failed parsing config.json:", err)
		os.Exit(1)
	}

	passwordPart := ""
	if *pqPass != "" {
		passwordPart = " password='" + *pqPass + "'"
	}
	db, err := sql.Open("postgres", fmt.Sprintf("host=%s user=%s dbname=%s sslmode=disable%s", *pqHost, *pqUser, *pqDB, passwordPart))
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed connecting to postgres:", err)
		os.Exit(1)
	}
	common.PQ = db
	if err := common.PQ.Ping(); err != nil {
		fmt.Fprintln(os.Stderr, "failed pinging postgres:", err)
		os.Exit(1)
	}
	common.InitSchemas("kewelstatus", kewelstatus.DBSchemas...)

	cfg := &kewelstatus.Config{
		GuildID:            *guildID,
		UpdateIntervalSecs: legacy.UpdateIntervalSecs,
		RadarrURL:          *radarrURL,
		RadarrAPIKey:       *radarrAPIKey,
		SonarrURL:          *sonarrURL,
		SonarrAPIKey:       *sonarrAPIKey,
		QueueAPIURL:        *queueAPIURL,
		DrivesPresence:     *drivesPresence,
		Notifications:      kewelstatus.DefaultNotifications(),
	}

	if id, err := strconv.ParseInt(legacy.Channels.Status, 10, 64); err == nil {
		cfg.StatusChannel = id
	}
	if id, err := strconv.ParseInt(legacy.Channels.Notifications, 10, 64); err == nil {
		cfg.NotifyChannel = id
	}

	defaults := kewelstatus.DefaultNotifications()
	cfg.Notifications.DownloadComplete = convertNotification(legacy.Notifications.DownloadComplete, defaults.DownloadComplete)
	cfg.Notifications.DownloadWarning = convertNotification(legacy.Notifications.DownloadWarning, defaults.DownloadWarning)
	cfg.Notifications.NewDownload = convertNotification(legacy.Notifications.NewDownload, defaults.NewDownload)
	cfg.Notifications.ServiceDown = convertNotification(legacy.Notifications.ServiceDown, defaults.ServiceDown)
	cfg.Notifications.ServiceUp = convertNotification(legacy.Notifications.ServiceUp, defaults.ServiceUp)

	if err := cfg.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "failed saving config:", err)
		os.Exit(1)
	}

	fmt.Printf("Migrated guild %d: status_channel=%d notify_channel=%d drives_presence=%v\n",
		cfg.GuildID, cfg.StatusChannel, cfg.NotifyChannel, cfg.DrivesPresence)
}
