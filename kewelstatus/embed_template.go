package kewelstatus

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
)

type EmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type EmbedTemplate struct {
	Color        string       `json:"color"`
	AuthorName   string       `json:"authorName"`
	AuthorIcon   string       `json:"authorIcon"`
	Title        string       `json:"title"`
	URL          string       `json:"url"`
	Description  string       `json:"description"`
	Fields       []EmbedField `json:"fields"`
	ThumbnailURL string       `json:"thumbnailUrl"`
	ImageURL     string       `json:"imageUrl"`
	FooterText   string       `json:"footerText"`
	FooterIcon   string       `json:"footerIcon"`
	Timestamp    bool         `json:"timestamp"`
}

var placeholderRe = regexp.MustCompile(`\{(\w+)\}`)

// Interpolate replaces {placeholder} tokens with values from vars, same regex-based
// substitution as the original Node bot's interpolate() — no nesting/conditionals.
func Interpolate(tmpl string, vars map[string]string) string {
	return placeholderRe.ReplaceAllStringFunc(tmpl, func(match string) string {
		key := match[1 : len(match)-1]
		if v, ok := vars[key]; ok {
			return v
		}
		return ""
	})
}

func clampField(s string, max int) string {
	if s == "" {
		return "​" // zero-width space, avoids Discord rejecting empty field values
	}
	r := []rune(s)
	if len(r) > max {
		return string(r[:max])
	}
	return s
}

// BuildEmbed renders an EmbedTemplate against vars into a real Discord embed, skipping
// any field whose interpolated value comes out empty.
func BuildEmbed(tmpl EmbedTemplate, vars map[string]string) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{}

	if tmpl.Color != "" {
		colorHex := strings.TrimPrefix(tmpl.Color, "#")
		if c, err := strconv.ParseInt(colorHex, 16, 64); err == nil {
			embed.Color = int(c)
		}
	}

	if title := Interpolate(tmpl.Title, vars); title != "" {
		embed.Title = clampField(title, 256)
	}
	if url := Interpolate(tmpl.URL, vars); url != "" {
		embed.URL = url
	}
	if desc := Interpolate(tmpl.Description, vars); desc != "" {
		embed.Description = clampField(desc, 4096)
	}
	if name := Interpolate(tmpl.AuthorName, vars); name != "" {
		embed.Author = &discordgo.MessageEmbedAuthor{Name: name, IconURL: Interpolate(tmpl.AuthorIcon, vars)}
	}
	if url := Interpolate(tmpl.ThumbnailURL, vars); url != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: url}
	}
	if url := Interpolate(tmpl.ImageURL, vars); url != "" {
		embed.Image = &discordgo.MessageEmbedImage{URL: url}
	}
	if text := Interpolate(tmpl.FooterText, vars); text != "" {
		embed.Footer = &discordgo.MessageEmbedFooter{Text: text, IconURL: Interpolate(tmpl.FooterIcon, vars)}
	}
	for _, f := range tmpl.Fields {
		name := Interpolate(f.Name, vars)
		value := Interpolate(f.Value, vars)
		if name == "" && value == "" {
			continue
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   clampField(name, 256),
			Value:  clampField(value, 1024),
			Inline: f.Inline,
		})
	}
	if tmpl.Timestamp {
		embed.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	return embed
}
