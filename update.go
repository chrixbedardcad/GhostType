package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/chrixbedardcad/GhostSpell/internal/version"
)

// checkAndNotifyUpdate checks GitHub for a newer version and calls setUpdate
// if one is available.
func checkAndNotifyUpdate(setUpdate func(string)) {
	slog.Info("[update] checking for updates...", "current", version.Version)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://api.github.com/repos/chrixbedardcad/GhostSpell/releases/latest", nil)
	if err != nil {
		slog.Warn("[update] failed to create request", "error", err)
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		slog.Warn("[update] HTTP request failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		slog.Warn("[update] GitHub API returned non-200", "status", resp.StatusCode)
		return
	}

	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		slog.Warn("[update] failed to parse response", "error", err)
		return
	}
	latest := rel.TagName
	if len(latest) > 0 && latest[0] == 'v' {
		latest = latest[1:]
	}

	slog.Info("[update] version check complete", "current", version.Version, "latest", latest)
	if latest != version.Version && latest != "" {
		slog.Info("[update] update available!", "current", version.Version, "latest", latest)
		setUpdate(latest)
	} else {
		slog.Info("[update] already up to date")
	}
}
