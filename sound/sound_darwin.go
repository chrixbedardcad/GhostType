//go:build darwin

package sound

import "os/exec"

func playWAV(data []byte) {
	playWAVWithCommand(data, "afplay")
}

func findPlayer() string {
	if path, err := exec.LookPath("afplay"); err == nil {
		return path
	}
	return ""
}
