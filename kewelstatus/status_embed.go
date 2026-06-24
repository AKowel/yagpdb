package kewelstatus

import (
	"fmt"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

func itemLine(item QueueItem) string {
	pct := item.Progress * 100
	speed := FormatSpeed(item.DLSpeed)
	eta := FormatEta(item.ETA)
	dl := FormatBytes(item.Size * item.Progress)
	total := FormatBytes(item.Size)

	line := fmt.Sprintf("**%s**", item.Title)
	if item.Subtitle != "" {
		line += fmt.Sprintf(" · *%s*", item.Subtitle)
	}
	line += fmt.Sprintf("\n`%s` %.0f%%", ProgressBar(pct, 12), pct)
	if speed != "" {
		line += fmt.Sprintf(" · **%s**", speed)
	}
	if eta != "" {
		line += fmt.Sprintf(" · ETA %s", eta)
	}
	line += fmt.Sprintf(" · %s / %s", dl, total)
	if item.TrackedStatus == "warning" || item.TrackedStatus == "error" {
		line += fmt.Sprintf(" · ⚠️ %s", item.TrackedStatus)
	}
	return line
}

func statusDot(ok bool) string {
	if ok {
		return "🟢"
	}
	return "🔴"
}

func clampStr(s string, max int) string {
	r := []rune(s)
	if len(r) > max {
		return string(r[:max])
	}
	return s
}

func allQueueItems(queue *QueueData) []QueueItem {
	all := make([]QueueItem, 0, len(queue.Movies)+len(queue.Shows))
	all = append(all, queue.Movies...)
	all = append(all, queue.Shows...)
	return all
}

// BuildStatusEmbed is the Go port of embeds.ts's buildStatusEmbed — the auto-updating
// "live status" embed shared by the background worker and the /queue command.
func BuildStatusEmbed(queue *QueueData, wantedMovies []WantedMovie, wantedEpisodes []WantedEpisode) *discordgo.MessageEmbed {
	allItems := allQueueItems(queue)

	color := 0x10b981 // green, idle
	if len(allItems) > 0 {
		color = 0x5865F2 // blurple
	}

	embed := &discordgo.MessageEmbed{
		Title:     "Kewel Media",
		Color:     color,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:  "Services",
		Value: fmt.Sprintf("%s Radarr  %s Sonarr  %s qBit", statusDot(queue.RadarrReachable), statusDot(queue.SonarrReachable), statusDot(queue.QbitConnected)),
	})

	if len(allItems) == 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "⬇️ Queue",
			Value: "Nothing downloading",
		})
	} else {
		shown := allItems
		overflow := ""
		if len(allItems) > 6 {
			shown = allItems[:6]
			overflow = fmt.Sprintf("\n*…and %d more*", len(allItems)-6)
		}
		lines := make([]string, len(shown))
		for i, item := range shown {
			lines[i] = itemLine(item)
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  fmt.Sprintf("⬇️ Downloading (%d)", len(allItems)),
			Value: clampStr(strings.Join(lines, "\n\n")+overflow, 1024),
		})
	}

	var wanted []string
	movieCount := len(wantedMovies)
	if movieCount > 8 {
		movieCount = 8
	}
	for _, m := range wantedMovies[:movieCount] {
		wanted = append(wanted, fmt.Sprintf("🎬 %s (%d)", m.Title, m.Year))
	}

	var seriesOrder []string
	bySeries := map[string][]WantedEpisode{}
	for _, ep := range wantedEpisodes {
		if _, ok := bySeries[ep.SeriesTitle]; !ok {
			seriesOrder = append(seriesOrder, ep.SeriesTitle)
		}
		bySeries[ep.SeriesTitle] = append(bySeries[ep.SeriesTitle], ep)
	}

	seriesShown := 0
	for _, title := range seriesOrder {
		if seriesShown >= 8 {
			break
		}
		eps := bySeries[title]
		if len(eps) == 1 {
			e := eps[0]
			wanted = append(wanted, fmt.Sprintf("📺 %s S%02dE%02d", title, e.SeasonNumber, e.EpisodeNumber))
		} else {
			wanted = append(wanted, fmt.Sprintf("📺 %s (%d episodes)", title, len(eps)))
		}
		seriesShown++
	}

	if len(wanted) > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  fmt.Sprintf("🔍 Requested, not yet found (%d)", len(wantedMovies)+len(seriesOrder)),
			Value: clampStr(strings.Join(wanted, "\n"), 1024),
		})
	}

	return embed
}
