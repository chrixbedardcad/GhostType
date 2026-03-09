//go:build darwin

package sound

import (
	"os"
	"os/exec"
)

func playWAV(data []byte) {
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

	cmd := exec.Command("afplay", tmpPath)
	if err := cmd.Start(); err != nil {
		os.Remove(tmpPath)
		return
	}
	go func() {
		cmd.Wait()
		os.Remove(tmpPath)
	}()
}
