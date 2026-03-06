//go:build linux

package sysinfo

import (
	"os"
	"os/exec"
	"strings"
)

func collectPlatform(info *Info) {
	info.OSVersion = readOSRelease()
	info.Locale = os.Getenv("LANG")
	if info.Locale == "" {
		info.Locale = "unknown"
	}
	info.KeyboardLayout = xkbLayout()
}

func readOSRelease() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			v := strings.TrimPrefix(line, "PRETTY_NAME=")
			return strings.Trim(v, `"`)
		}
	}
	return "unknown"
}

func xkbLayout() string {
	out, err := exec.Command("setxkbmap", "-query").Output()
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "layout:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "layout:"))
		}
	}
	return "unknown"
}
