//go:build !windows

package sound

import (
	_ "embed"
	"sync"
	"time"
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

//go:embed cancel.wav
var cancelWAV []byte

var (
	enabled bool
	mu      sync.Mutex

	workingStop chan struct{}
	workingMu   sync.Mutex
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
func PlayCancel()  { play(cancelWAV) }
func PlayWorking() { play(workingWAV) }

// StartWorkingLoop plays working.wav in a loop until StopWorkingLoop is called.
func StartWorkingLoop() {
	mu.Lock()
	e := enabled
	mu.Unlock()
	if !e || len(workingWAV) == 0 {
		return
	}

	workingMu.Lock()
	// Stop any existing loop.
	if workingStop != nil {
		close(workingStop)
	}
	stop := make(chan struct{})
	workingStop = stop
	workingMu.Unlock()

	go func() {
		for {
			playWAVLoop(workingWAV) // blocks until sound finishes; trackable for kill
			select {
			case <-stop:
				return
			case <-time.After(100 * time.Millisecond): // small gap between loops
			}
		}
	}()
}

// StopWorkingLoop stops the looping working sound.
func StopWorkingLoop() {
	workingMu.Lock()
	wasRunning := workingStop != nil
	if wasRunning {
		close(workingStop)
		workingStop = nil
	}
	workingMu.Unlock()
	// Kill any lingering playback process — but only if the loop was actually
	// running, so we don't kill a success/error sound that started after us.
	if wasRunning {
		stopPlayback()
	}
}

func SetEnabled(v bool) {
	mu.Lock()
	enabled = v
	mu.Unlock()
}
