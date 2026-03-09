//go:build linux

package sound

import (
	"os"
	"os/exec"
)

func playWAV(data []byte) {
	// Try paplay (PulseAudio) first, then aplay (ALSA).
	var player string
	for _, p := range []string{"paplay", "aplay"} {
		if path, err := exec.LookPath(p); err == nil {
			player = path
			break
		}
	}
	if player == "" {
		return
	}

	f, err := os.CreateTemp("", "ghosttype-*.wav")
	if err != nil {
		return
	}
	tmpPath := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return
	}
	f.Close()

	cmd := exec.Command(player, tmpPath)
	if err := cmd.Start(); err != nil {
		os.Remove(tmpPath)
		return
	}
	go func() {
		cmd.Wait()
		os.Remove(tmpPath)
	}()
}
