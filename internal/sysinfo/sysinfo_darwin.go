//go:build darwin

package sysinfo

import (
	"os/exec"
	"strings"
)

func collectPlatform(info *Info) {
	info.OSVersion = cmdOutput("sw_vers", "-productVersion")
	info.Locale = cmdOutput("defaults", "read", "-g", "AppleLocale")
	// Get current keyboard input source.
	raw := cmdOutput("defaults", "read", "com.apple.HIToolbox", "AppleSelectedInputSources")
	info.KeyboardLayout = parseInputSource(raw)
}

func cmdOutput(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func parseInputSource(raw string) string {
	// Extract "KeyboardLayout Name" or "Input Mode" from the plist output.
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "KeyboardLayout Name") || strings.Contains(line, "Input Mode") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				v := strings.TrimSpace(parts[1])
				v = strings.Trim(v, `";`)
				return v
			}
		}
	}
	if raw != "" && raw != "unknown" {
		return "(see raw plist)"
	}
	return "unknown"
}
