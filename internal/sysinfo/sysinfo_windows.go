//go:build windows

package sysinfo

import (
	"os/exec"
	"strings"
)

func collectPlatform(info *Info) {
	out, err := exec.Command("cmd", "/c", "ver").Output()
	if err == nil {
		info.OSVersion = strings.TrimSpace(string(out))
	} else {
		info.OSVersion = "unknown"
	}
	info.Locale = "unknown"
	info.KeyboardLayout = "unknown"
}
