package kewelstatus

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

type presenceItem struct {
	title    string
	subtitle string
	year     int
	seenAt   time.Time
}

type presenceGuildData struct {
	guildID     int64
	downloading []presenceItem
	warned      []presenceItem
	queueLen    int
}

var presenceAssets = func(image, text string) discordgo.Assets {
	return discordgo.Assets{LargeImageID: image, LargeText: text, SmallImageID: "kewel", SmallText: "Kewel Bot"}
}

// updatePresence aggregates the snapshot from every guild with drives_presence enabled
// into a single bot-wide Discord presence (a bot has exactly one presence across all
// guilds — there's no per-guild presence in Discord's protocol). Tie-broken by lowest
// guild ID for determinism when more than one guild contributes data in the same tick.
func updatePresence(data []presenceGuildData) {
	if len(data) == 0 {
		return
	}

	sort.Slice(data, func(i, j int) bool { return data[i].guildID < data[j].guildID })

	var allWarned []presenceItem
	var allDownloading []presenceItem
	totalQueue := 0
	for _, d := range data {
		allWarned = append(allWarned, d.warned...)
		allDownloading = append(allDownloading, d.downloading...)
		totalQueue += d.queueLen
	}

	var status discordgo.Status
	var activity discordgo.Activity
	var idleSince *int

	switch {
	case len(allWarned) > 0:
		status = discordgo.StatusDoNotDisturb
		details := fmt.Sprintf("%d downloads", len(allWarned))
		if len(allWarned) == 1 {
			details = allWarned[0].title
		}
		activity = discordgo.Activity{
			Name: "Kewel Media", Type: discordgo.ActivityTypeWatching,
			Details: details, State: "⚠️ Needs attention",
			Assets: presenceAssets("warning", "Warning"),
		}

	case len(allDownloading) == 1:
		item := allDownloading[0]
		status = discordgo.StatusOnline
		state := "⬇️ Downloading"
		if item.subtitle != "" {
			state = "⬇️ " + item.subtitle
		} else if item.year != 0 {
			state = fmt.Sprintf("⬇️ %d", item.year)
		}
		activity = discordgo.Activity{
			Name: "Kewel Media", Type: discordgo.ActivityTypeWatching,
			Details:    item.title,
			State:      state,
			TimeStamps: discordgo.TimeStamps{StartTimestamp: item.seenAt.UnixMilli()},
			Assets:     presenceAssets("downloading", "Downloading"),
		}

	case len(allDownloading) > 1:
		earliest := allDownloading[0].seenAt
		var titles []string
		for i, item := range allDownloading {
			if item.seenAt.Before(earliest) {
				earliest = item.seenAt
			}
			if i < 2 {
				titles = append(titles, item.title)
			}
		}
		status = discordgo.StatusOnline
		activity = discordgo.Activity{
			Name: "Kewel Media", Type: discordgo.ActivityTypeWatching,
			Details:    fmt.Sprintf("%d downloads in progress", len(allDownloading)),
			State:      strings.Join(titles, " · "),
			TimeStamps: discordgo.TimeStamps{StartTimestamp: earliest.UnixMilli()},
			Assets:     presenceAssets("downloading", "Downloading"),
		}

	case totalQueue > 0:
		status = discordgo.StatusIdle
		since := int(time.Now().UnixMilli())
		idleSince = &since
		plural := "s"
		if totalQueue == 1 {
			plural = ""
		}
		activity = discordgo.Activity{
			Name: "Kewel Media", Type: discordgo.ActivityTypeWatching,
			Details: fmt.Sprintf("%d item%s in queue", totalQueue, plural),
			State:   "Waiting to download",
			Assets:  presenceAssets("idle", "Queue idle"),
		}

	default:
		status = discordgo.StatusOnline
		activity = discordgo.Activity{
			Name: "Kewel Media", Type: discordgo.ActivityTypeWatching,
			Details: "Queue is empty", State: "All caught up ✅",
			Assets: presenceAssets("idle", "Nothing downloading"),
		}
	}

	err := common.BotSession.UpdateStatusComplex(discordgo.UpdateStatusData{
		IdleSince: idleSince,
		Activity:  &activity,
		Status:    status,
	})
	if err != nil {
		logger.WithError(err).Error("kewelstatus: failed updating presence")
	}
}
