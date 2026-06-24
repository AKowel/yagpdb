package kewelstatus

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/mqueue"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

const sweepInterval = 15 * time.Second

var stopWorker = make(chan struct{})

type prevItem struct {
	title     string
	subtitle  string
	year      int
	wasActive bool
	warnSent  bool
	messages  []string
	seenAt    time.Time
}

type serviceState struct {
	radarr bool
	sonarr bool
	qbit   bool
}

type guildWorkerState struct {
	mu           sync.Mutex
	prevItems    map[string]*prevItem
	prevServices *serviceState
	newItemsSeen map[string]bool
	firstPoll    bool
	lastPolledAt time.Time
}

var (
	statesMu sync.Mutex
	states   = map[int64]*guildWorkerState{}
)

func getState(guildID int64) *guildWorkerState {
	statesMu.Lock()
	defer statesMu.Unlock()
	s, ok := states[guildID]
	if !ok {
		s = &guildWorkerState{
			prevItems:    map[string]*prevItem{},
			newItemsSeen: map[string]bool{},
			firstPoll:    true,
		}
		states[guildID] = s
	}
	return s
}

func (p *Plugin) RunBackgroundWorker() {
	pollAll()

	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pollAll()
		case <-stopWorker:
			return
		}
	}
}

func (p *Plugin) StopBackgroundWorker(wg *sync.WaitGroup) {
	close(stopWorker)
	wg.Done()
}

func pollAll() {
	configs, err := AllConfigs()
	if err != nil {
		logger.WithError(err).Error("kewelstatus: failed fetching configs")
		return
	}

	now := time.Now()
	var presenceData []presenceGuildData

	for _, cfg := range configs {
		state := getState(cfg.GuildID)

		interval := time.Duration(cfg.UpdateIntervalSecs) * time.Second
		if interval <= 0 {
			interval = 60 * time.Second
		}
		if !state.lastPolledAt.IsZero() && now.Sub(state.lastPolledAt) < interval {
			continue
		}
		state.lastPolledAt = now

		data, err := pollGuild(cfg, state)
		if err != nil {
			logger.WithError(err).WithField("guild", cfg.GuildID).Error("kewelstatus: poll failed")
			continue
		}
		if cfg.DrivesPresence && data != nil {
			presenceData = append(presenceData, *data)
		}
	}

	updatePresence(presenceData)
}

func isActive(item QueueItem) bool {
	return item.QbitState == "downloading" || item.Status == "downloading" || item.TrackedState == "importPending"
}

func resolveChannel(override, fallback int64) int64 {
	if override != 0 {
		return override
	}
	return fallback
}

func yearStr(year int) string {
	if year == 0 {
		return ""
	}
	return strconv.Itoa(year)
}

func sendNotification(channelID, guildID int64, embed *discordgo.MessageEmbed, sourceItemID string) {
	if channelID == 0 {
		return
	}
	err := mqueue.QueueMessage(&mqueue.QueuedElement{
		GuildID:      guildID,
		ChannelID:    channelID,
		Source:       "kewelstatus",
		SourceItemID: sourceItemID,
		MessageEmbed: embed,
	})
	if err != nil {
		logger.WithError(err).Error("kewelstatus: failed queueing notification")
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func checkServiceTransition(name string, ok, wasOk bool, errPtr *string, cfg *Config, notifyFallback int64) {
	label := capitalize(name)
	if ok && !wasOk && cfg.Notifications.ServiceUp.Enabled {
		n := cfg.Notifications.ServiceUp
		sendNotification(resolveChannel(n.Channel, notifyFallback), cfg.GuildID,
			BuildEmbed(n.Embed, map[string]string{"service": label}), name)
	}
	if !ok && wasOk && cfg.Notifications.ServiceDown.Enabled {
		n := cfg.Notifications.ServiceDown
		errStr := ""
		if errPtr != nil {
			errStr = *errPtr
		}
		sendNotification(resolveChannel(n.Channel, notifyFallback), cfg.GuildID,
			BuildEmbed(n.Embed, map[string]string{"service": label, "error": errStr}), name)
	}
}

func upsertStatusEmbed(cfg *Config, embed *discordgo.MessageEmbed) {
	if cfg.StatusMessageID != 0 {
		_, err := common.BotSession.ChannelMessageEditComplex(&discordgo.MessageEdit{
			ID:      cfg.StatusMessageID,
			Channel: cfg.StatusChannel,
			Embeds:  []*discordgo.MessageEmbed{embed},
		})
		if err == nil {
			return
		}
		cfg.StatusMessageID = 0
	}

	msg, err := common.BotSession.ChannelMessageSendEmbed(cfg.StatusChannel, embed)
	if err != nil {
		logger.WithError(err).WithField("guild", cfg.GuildID).Error("kewelstatus: failed sending status embed")
		return
	}
	cfg.StatusMessageID = msg.ID
	if err := cfg.SaveStatusMessageID(); err != nil {
		logger.WithError(err).Error("kewelstatus: failed saving status message id")
	}
}

// pollGuild runs one full poll cycle for a single guild: fetch queue/wanted, fire the
// five notification types on state transitions, upsert the live status embed, and
// return a snapshot for presence aggregation. Port of liveStatus.ts's update().
func pollGuild(cfg *Config, state *guildWorkerState) (*presenceGuildData, error) {
	queue, err := FetchQueue(cfg.QueueAPIURL)
	if err != nil {
		return nil, err
	}

	var wantedMovies []WantedMovie
	var wantedEpisodes []WantedEpisode
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); wantedMovies = FetchWantedMovies(cfg.RadarrURL, cfg.RadarrAPIKey) }()
	go func() { defer wg.Done(); wantedEpisodes = FetchWantedEpisodes(cfg.SonarrURL, cfg.SonarrAPIKey) }()
	wg.Wait()

	allItems := allQueueItems(queue)

	state.mu.Lock()
	defer state.mu.Unlock()

	currMap := make(map[string]bool, len(allItems))
	for _, item := range allItems {
		currMap[item.ID] = true
	}

	currServices := &serviceState{radarr: queue.RadarrReachable, sonarr: queue.SonarrReachable, qbit: queue.QbitConnected}
	notifyFallback := cfg.NotifyChannel

	// ── Service up/down ──
	if !state.firstPoll && state.prevServices != nil {
		checkServiceTransition("radarr", currServices.radarr, state.prevServices.radarr, queue.Errors.Radarr, cfg, notifyFallback)
		checkServiceTransition("sonarr", currServices.sonarr, state.prevServices.sonarr, queue.Errors.Sonarr, cfg, notifyFallback)
		checkServiceTransition("qbit", currServices.qbit, state.prevServices.qbit, queue.Errors.Qbit, cfg, notifyFallback)
	}
	state.prevServices = currServices

	// ── New downloads ──
	if !state.firstPoll && cfg.Notifications.NewDownload.Enabled {
		n := cfg.Notifications.NewDownload
		for _, item := range allItems {
			if !state.newItemsSeen[item.ID] {
				vars := map[string]string{
					"title": item.Title, "year": yearStr(item.Year), "subtitle": item.Subtitle,
					"size": FormatBytes(item.Size),
				}
				sendNotification(resolveChannel(n.Channel, notifyFallback), cfg.GuildID, BuildEmbed(n.Embed, vars), item.ID)
			}
		}
	}
	for _, item := range allItems {
		state.newItemsSeen[item.ID] = true
	}

	// ── Download warnings (only once per item) ──
	if cfg.Notifications.DownloadWarning.Enabled {
		n := cfg.Notifications.DownloadWarning
		for _, item := range allItems {
			prev := state.prevItems[item.ID]
			hasWarning := item.TrackedStatus == "warning" || item.TrackedStatus == "error"
			if hasWarning && !state.firstPoll && (prev == nil || !prev.warnSent) {
				vars := map[string]string{
					"title": item.Title, "year": yearStr(item.Year), "subtitle": item.Subtitle,
					"trackedStatus": item.TrackedStatus, "messages": strings.Join(item.Messages, "\n"),
				}
				sendNotification(resolveChannel(n.Channel, notifyFallback), cfg.GuildID, BuildEmbed(n.Embed, vars), item.ID)
			}
		}
	}

	// ── Download complete ──
	if !state.firstPoll && cfg.Notifications.DownloadComplete.Enabled {
		n := cfg.Notifications.DownloadComplete
		for id, prev := range state.prevItems {
			if prev.wasActive && !currMap[id] {
				vars := map[string]string{"title": prev.title, "subtitle": prev.subtitle}
				sendNotification(resolveChannel(n.Channel, notifyFallback), cfg.GuildID, BuildEmbed(n.Embed, vars), id)
			}
		}
	}

	// ── Update prev item state ──
	newPrevItems := make(map[string]*prevItem, len(allItems))
	for _, item := range allItems {
		old := state.prevItems[item.ID]
		warnSent := item.TrackedStatus == "warning" || item.TrackedStatus == "error"
		seenAt := time.Now()
		if old != nil {
			warnSent = warnSent || old.warnSent
			seenAt = old.seenAt
		}
		newPrevItems[item.ID] = &prevItem{
			title: item.Title, subtitle: item.Subtitle, year: item.Year,
			wasActive: isActive(item), warnSent: warnSent, messages: item.Messages, seenAt: seenAt,
		}
	}
	state.prevItems = newPrevItems
	state.firstPoll = false

	// ── Live status embed ──
	if cfg.StatusChannel != 0 {
		embed := BuildStatusEmbed(queue, wantedMovies, wantedEpisodes)
		upsertStatusEmbed(cfg, embed)
	}

	// ── Presence snapshot ──
	data := &presenceGuildData{guildID: cfg.GuildID, queueLen: len(allItems)}
	for _, item := range allItems {
		pi := newPrevItems[item.ID]
		if item.TrackedStatus == "warning" || item.TrackedStatus == "error" {
			data.warned = append(data.warned, presenceItem{title: item.Title})
		}
		if isActive(item) {
			data.downloading = append(data.downloading, presenceItem{
				title: item.Title, subtitle: item.Subtitle, year: item.Year, seenAt: pi.seenAt,
			})
		}
	}

	return data, nil
}
