//go:build linux

package sound

import "os/exec"

func playWAV(data []byte) {
	// Try paplay (PulseAudio) first, then aplay (ALSA).
	for _, player := range []string{"paplay", "aplay"} {
		if path, err := exec.LookPath(player); err == nil {
			playWAVWithCommand(data, path)
			return
		}
	}
}

func findPlayer() string {
	for _, player := range []string{"paplay", "aplay"} {
		if path, err := exec.LookPath(player); err == nil {
			return path
		}
	}
	return ""
}
