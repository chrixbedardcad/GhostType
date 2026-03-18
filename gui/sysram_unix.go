//go:build darwin || linux

package gui

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

func getSystemRAMGB() int {
	var out []byte
	var err error

	switch runtime.GOOS {
	case "darwin":
		out, err = exec.Command("sysctl", "-n", "hw.memsize").Output()
	case "linux":
		// /proc/meminfo: "MemTotal:    16384000 kB"
		out, err = exec.Command("sh", "-c", "awk '/MemTotal/{print $2}' /proc/meminfo").Output()
		if err == nil {
			kb, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
			return int(kb / 1024 / 1024)
		}
	}

	if err != nil {
		return 8 // safe default
	}

	bytes, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if bytes <= 0 {
		return 8
	}
	return int(bytes / 1024 / 1024 / 1024)
}
