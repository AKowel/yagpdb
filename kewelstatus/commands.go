package kewelstatus

import (
	"time"

	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

var _ commands.CommandProvider = (*Plugin)(nil)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, cmds...)
}

var cmds = []*commands.YAGCommand{
	{
		CmdCategory:         commands.CategoryTool,
		Name:                "Queue",
		Description:         "Show the current download queue and requested items",
		SlashCommandEnabled: true,
		DefaultEnabled:      true,
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			cfg, err := GetOrDefaultConfig(parsed.GuildData.GS.ID)
			if err != nil {
				return nil, err
			}

			queue, err := FetchQueue(cfg.QueueAPIURL)
			if err != nil {
				return "❌ Failed to fetch queue: `" + err.Error() + "`", nil
			}

			wantedMovies := FetchWantedMovies(cfg.RadarrURL, cfg.RadarrAPIKey)
			wantedEpisodes := FetchWantedEpisodes(cfg.SonarrURL, cfg.SonarrAPIKey)

			return BuildStatusEmbed(queue, wantedMovies, wantedEpisodes), nil
		},
	},
	{
		CmdCategory:         commands.CategoryTool,
		Name:                "Status",
		Description:         "Show the health of all media services",
		SlashCommandEnabled: true,
		DefaultEnabled:      true,
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			cfg, err := GetOrDefaultConfig(parsed.GuildData.GS.ID)
			if err != nil {
				return nil, err
			}

			queue, err := FetchQueue(cfg.QueueAPIURL)
			if err != nil {
				return "❌ Failed to reach queue API: `" + err.Error() + "`", nil
			}

			allUp := queue.RadarrReachable && queue.SonarrReachable && queue.QbitConnected
			color := 0x10b981
			if !allUp {
				color = 0xef4444
			}

			embed := &discordgo.MessageEmbed{
				Title:     "Service Health",
				Color:     color,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Fields: []*discordgo.MessageEmbedField{
					serviceHealthField("Radarr", queue.RadarrReachable, queue.Errors.Radarr),
					serviceHealthField("Sonarr", queue.SonarrReachable, queue.Errors.Sonarr),
					serviceHealthField("qBittorrent", queue.QbitConnected, queue.Errors.Qbit),
				},
			}
			return embed, nil
		},
	},
}

func serviceHealthField(name string, ok bool, errPtr *string) *discordgo.MessageEmbedField {
	value := "🟢 Online"
	if !ok {
		errStr := ""
		if errPtr != nil {
			errStr = *errPtr
		}
		value = "🔴 Offline\n`" + errStr + "`"
	}
	return &discordgo.MessageEmbedField{Name: name, Value: value, Inline: true}
}
