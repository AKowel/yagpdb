package kewelstatus

import (
	_ "embed"
	"encoding/json"
	"net/http"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/cplogs"
	"github.com/botlabs-gg/yagpdb/v2/web"
	"goji.io/pat"
)

//go:embed assets/kewelstatus_settings.html
var SettingsPageHTML string

var panelLogKey = cplogs.RegisterActionFormat(&cplogs.ActionFormat{Key: "kewelstatus_settings", FormatString: "Updated Kewel Status settings"})

type SettingsForm struct {
	StatusChannel      int64  `schema:"status_channel" valid:"channel,true"`
	NotifyChannel      int64  `schema:"notify_channel" valid:"channel,true"`
	UpdateIntervalSecs int    `schema:"update_interval_secs"`
	RadarrURL          string `schema:"radarr_url"`
	RadarrAPIKey       string `schema:"radarr_api_key"`
	SonarrURL          string `schema:"sonarr_url"`
	SonarrAPIKey       string `schema:"sonarr_api_key"`
	QueueAPIURL        string `schema:"queue_api_url"`
	DrivesPresence     bool   `schema:"drives_presence"`
	NotificationsJSON  string `schema:"notifications_json"`
}

func (p *Plugin) InitWeb() {
	web.AddHTMLTemplate("kewelstatus/assets/kewelstatus_settings.html", SettingsPageHTML)
	web.AddSidebarItem(web.SidebarCategoryFeeds, &web.SidebarItem{
		Name: "Kewel Status",
		URL:  "kewelstatus/settings",
		Icon: "fas fa-download",
	})

	getHandler := web.RenderHandler(HandleSettingsGet, "cp_kewelstatus_settings")
	postHandler := web.ControllerPostHandler(HandleSettingsPost, getHandler, SettingsForm{})

	web.CPMux.Handle(pat.Get("/kewelstatus/settings"), getHandler)
	web.CPMux.Handle(pat.Get("/kewelstatus/settings/"), getHandler)
	web.CPMux.Handle(pat.Post("/kewelstatus/settings"), postHandler)
	web.CPMux.Handle(pat.Post("/kewelstatus/settings/"), postHandler)
}

func configToForm(cfg *Config) *SettingsForm {
	notifRaw, _ := json.MarshalIndent(cfg.Notifications, "", "  ")
	return &SettingsForm{
		StatusChannel:      cfg.StatusChannel,
		NotifyChannel:      cfg.NotifyChannel,
		UpdateIntervalSecs: cfg.UpdateIntervalSecs,
		RadarrURL:          cfg.RadarrURL,
		RadarrAPIKey:       cfg.RadarrAPIKey,
		SonarrURL:          cfg.SonarrURL,
		SonarrAPIKey:       cfg.SonarrAPIKey,
		QueueAPIURL:        cfg.QueueAPIURL,
		DrivesPresence:     cfg.DrivesPresence,
		NotificationsJSON:  string(notifRaw),
	}
}

func HandleSettingsGet(w http.ResponseWriter, r *http.Request) interface{} {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	if formConfig, ok := ctx.Value(common.ContextKeyParsedForm).(*SettingsForm); ok {
		templateData["Settings"] = formConfig
		return templateData
	}

	cfg, err := GetOrDefaultConfig(activeGuild.ID)
	if err != nil {
		web.CtxLogger(ctx).WithError(err).Error("kewelstatus: failed retrieving config")
	}
	templateData["Settings"] = configToForm(cfg)
	return templateData
}

func HandleSettingsPost(w http.ResponseWriter, r *http.Request) (web.TemplateData, error) {
	ctx := r.Context()
	activeGuild, templateData := web.GetBaseCPContextData(ctx)

	form := ctx.Value(common.ContextKeyParsedForm).(*SettingsForm)

	cfg, err := GetOrDefaultConfig(activeGuild.ID)
	if err != nil {
		return templateData, err
	}

	if form.NotificationsJSON != "" {
		var notifs Notifications
		if err := json.Unmarshal([]byte(form.NotificationsJSON), &notifs); err != nil {
			templateData.AddAlerts(web.ErrorAlert("Invalid notifications JSON: " + err.Error()))
			return templateData, nil
		}
		cfg.Notifications = notifs
	}

	cfg.GuildID = activeGuild.ID
	cfg.StatusChannel = form.StatusChannel
	cfg.NotifyChannel = form.NotifyChannel
	cfg.UpdateIntervalSecs = form.UpdateIntervalSecs
	cfg.RadarrURL = form.RadarrURL
	cfg.RadarrAPIKey = form.RadarrAPIKey
	cfg.SonarrURL = form.SonarrURL
	cfg.SonarrAPIKey = form.SonarrAPIKey
	cfg.QueueAPIURL = form.QueueAPIURL
	cfg.DrivesPresence = form.DrivesPresence

	if err := cfg.Save(); err != nil {
		return templateData, err
	}

	go cplogs.RetryAddEntry(web.NewLogEntryFromContext(r.Context(), panelLogKey))

	return templateData, nil
}
