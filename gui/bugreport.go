package gui

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/url"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/chrixbedardcad/GhostSpell/internal/sysinfo"
	"github.com/chrixbedardcad/GhostSpell/internal/version"
)

// redactSecrets removes API keys and tokens from log text.
func redactSecrets(text string) string {
	// Redact common API key patterns: sk-..., key-..., bearer tokens, etc.
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9_-]{10,})`),
		regexp.MustCompile(`(?i)(key-[a-zA-Z0-9_-]{10,})`),
		regexp.MustCompile(`(?i)(api[_-]?key\s*[:=]\s*)"?([^"\s,}{]+)`),
		regexp.MustCompile(`(?i)(bearer\s+)([a-zA-Z0-9_.-]{20,})`),
		regexp.MustCompile(`(?i)(refresh[_-]?token\s*[:=]\s*)"?([^"\s,}{]+)`),
		regexp.MustCompile(`(?i)(eyJ[a-zA-Z0-9_-]{20,}\.[a-zA-Z0-9_-]{20,})`), // JWT
	}
	result := text
	for _, re := range patterns {
		result = re.ReplaceAllString(result, "[REDACTED]")
	}
	return result
}

// SubmitBugReport collects diagnostics and opens a GitHub issue with pre-filled content.
// The description parameter is the user's bug description which appears first in the issue.
func (s *SettingsService) SubmitBugReport(description string) string {
	guiLog("[GUI] JS called: SubmitBugReport")

	// Collect system info.
	sys := sysinfo.Collect()

	// Build provider list (names only, no keys).
	var providers []string
	var defaultModel string
	if s.cfgCopy != nil {
		for name := range s.cfgCopy.Providers {
			providers = append(providers, name)
		}
		defaultModel = s.cfgCopy.DefaultModel
	}

	// Collect log tail.
	logTail := ""
	if s.DebugTailFn != nil {
		if tail, err := s.DebugTailFn(); err == nil {
			logTail = redactSecrets(tail)
		}
	}

	// Generate a fingerprint from system info + description for duplicate detection.
	fingerprint := fmt.Sprintf("%x", sha256.Sum256([]byte(
		version.Version+sys.OS+sys.OSVersion+sys.Arch+description,
	)))[:12]

	// Build issue body — user description first, then diagnostics.
	var body strings.Builder

	if description != "" {
		body.WriteString("## Description\n\n")
		body.WriteString(description)
		body.WriteString("\n\n")
	}

	body.WriteString("## System Information\n\n")
	body.WriteString("| | |\n|---|---|\n")
	fmt.Fprintf(&body, "| **Version** | %s |\n", version.Version)
	fmt.Fprintf(&body, "| **OS** | %s %s (%s) |\n", sys.OS, sys.OSVersion, sys.Arch)
	fmt.Fprintf(&body, "| **Locale** | %s |\n", sys.Locale)
	fmt.Fprintf(&body, "| **Keyboard** | %s |\n", sys.KeyboardLayout)
	fmt.Fprintf(&body, "| **Providers** | %s |\n", strings.Join(providers, ", "))
	fmt.Fprintf(&body, "| **Default Model** | %s |\n", defaultModel)
	fmt.Fprintf(&body, "| **Fingerprint** | `%s` |\n", fingerprint)

	if logTail != "" {
		body.WriteString("\n## Recent Log (last ~200 lines)\n\n")
		body.WriteString("<details><summary>Click to expand</summary>\n\n```\n")
		// Limit log to ~6000 chars to stay within URL limits.
		if len(logTail) > 6000 {
			logTail = logTail[len(logTail)-6000:]
		}
		body.WriteString(logTail)
		body.WriteString("\n```\n\n</details>\n")
	}

	// Build the GitHub new issue URL.
	title := fmt.Sprintf("Bug Report — v%s %s/%s", version.Version, sys.OS, sys.Arch)
	issueURL := fmt.Sprintf(
		"https://github.com/chrixbedardcad/GhostSpell/issues/new?title=%s&body=%s&labels=bug",
		url.QueryEscape(title),
		url.QueryEscape(body.String()),
	)

	// GitHub URLs have a practical limit of ~8192 chars. If we exceed it,
	// truncate the log portion and retry.
	if len(issueURL) > 8000 {
		// Rebuild with shorter log.
		body.Reset()
		if description != "" {
			body.WriteString("## Description\n\n")
			body.WriteString(description)
			body.WriteString("\n\n")
		}
		body.WriteString("## System Information\n\n")
		body.WriteString("| | |\n|---|---|\n")
		fmt.Fprintf(&body, "| **Version** | %s |\n", version.Version)
		fmt.Fprintf(&body, "| **OS** | %s %s (%s) |\n", sys.OS, sys.OSVersion, sys.Arch)
		fmt.Fprintf(&body, "| **Locale** | %s |\n", sys.Locale)
		fmt.Fprintf(&body, "| **Keyboard** | %s |\n", sys.KeyboardLayout)
		fmt.Fprintf(&body, "| **Providers** | %s |\n", strings.Join(providers, ", "))
		fmt.Fprintf(&body, "| **Default Model** | %s |\n", defaultModel)
		fmt.Fprintf(&body, "| **Fingerprint** | `%s` |\n", fingerprint)
		body.WriteString("\n_Log was too large for URL — please paste from clipboard or attach ghostspell.log_\n")

		issueURL = fmt.Sprintf(
			"https://github.com/chrixbedardcad/GhostSpell/issues/new?title=%s&body=%s&labels=bug",
			url.QueryEscape(title),
			url.QueryEscape(body.String()),
		)
	}

	// Open browser.
	if err := openBrowser(issueURL); err != nil {
		slog.Error("[bugreport] failed to open browser", "error", err)
		return "error: failed to open browser — " + err.Error()
	}

	slog.Info("[bugreport] bug report submitted", "fingerprint", fingerprint)
	return "ok"
}

// openBrowser opens a URL in the default browser.
func openBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	case "darwin":
		cmd = exec.Command("open", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}
