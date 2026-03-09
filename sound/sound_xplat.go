//go:build !windows

package sound

import (
	_ "embed"
	"sync"
)

//go:embed start.wav
var startWAV []byte

//go:embed working.wav
var workingWAV []byte

//go:embed success.wav
var successWAV []byte

//go:embed error.wav
var errorWAV []byte

//go:embed toggle.wav
var toggleWAV []byte

var (
	enabled bool
	mu      sync.Mutex
)

// Init enables or disables sound playback.
func Init(soundEnabled bool) {
	mu.Lock()
	enabled = soundEnabled
	mu.Unlock()
}

func play(data []byte) {
	mu.Lock()
	e := enabled
	mu.Unlock()
	if !e || len(data) == 0 {
		return
	}
	go playWAV(data)
}

func PlayStart()   { play(startWAV) }
func PlaySuccess() { play(successWAV) }
func PlayError()   { play(errorWAV) }
func PlayToggle()  { play(toggleWAV) }
func PlayWorking() { play(workingWAV) }

func SetEnabled(v bool) {
	mu.Lock()
	enabled = v
	mu.Unlock()
}
