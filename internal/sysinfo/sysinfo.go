// Package sysinfo collects platform-specific system information for debug reports.
package sysinfo

import (
	"fmt"
	"runtime"
	"strings"
)

// Info holds collected system information.
type Info struct {
	OS              string
	OSVersion       string
	Arch            string
	Locale          string
	KeyboardLayout  string
	GoVersion       string
}

// Collect gathers system information for the current platform.
func Collect() Info {
	info := Info{
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		GoVersion: runtime.Version(),
	}
	collectPlatform(&info)
	return info
}

// String formats the system info as a human-readable block.
func (i Info) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "OS:             %s %s (%s)\n", i.OS, i.OSVersion, i.Arch)
	fmt.Fprintf(&b, "Locale:         %s\n", i.Locale)
	fmt.Fprintf(&b, "Keyboard:       %s\n", i.KeyboardLayout)
	fmt.Fprintf(&b, "Go:             %s\n", i.GoVersion)
	return b.String()
}
