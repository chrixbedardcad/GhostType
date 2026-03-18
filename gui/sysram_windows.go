//go:build windows

package gui

import (
	"os/exec"
	"strconv"
	"strings"
)

func getSystemRAMGB() int {
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		"(Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory").Output()
	if err != nil {
		return 8 // safe default
	}
	bytes, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if bytes <= 0 {
		return 8
	}
	return int(bytes / 1024 / 1024 / 1024)
}
