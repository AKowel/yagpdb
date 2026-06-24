package kewelstatus

import (
	"fmt"
	"strings"
)

func FormatBytes(bytes float64) string {
	if bytes <= 0 {
		return "—"
	}
	if bytes >= 1e9 {
		return fmt.Sprintf("%.1f GB", bytes/1e9)
	}
	if bytes >= 1e6 {
		return fmt.Sprintf("%.0f MB", bytes/1e6)
	}
	return fmt.Sprintf("%.0f KB", bytes/1e3)
}

func FormatSpeed(bps float64) string {
	if bps <= 0 {
		return ""
	}
	if bps >= 1e6 {
		return fmt.Sprintf("%.1f MB/s", bps/1e6)
	}
	return fmt.Sprintf("%.0f KB/s", bps/1e3)
}

func FormatEta(secs float64) string {
	if secs < 0 || secs >= 8_640_000 {
		return ""
	}
	if secs < 60 {
		return fmt.Sprintf("%.0fs", secs)
	}
	if secs < 3600 {
		return fmt.Sprintf("%.0fm", secs/60)
	}
	h := int(secs / 3600)
	m := int(secs) % 3600 / 60
	if m > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dh", h)
}

func ProgressBar(pct float64, width int) string {
	filled := int(pct/100*float64(width) + 0.5)
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}
