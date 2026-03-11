//go:build windows

package sound

import (
	_ "embed"
	"sync"
	"syscall"
	"time"
	"unsafe"
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

const (
	sndMemory    = 0x0004
	sndAsync     = 0x0001
	sndNoDefault = 0x0002
	sndSync      = 0x0000
)

var (
	winmm         = syscall.NewLazyDLL("winmm.dll")
	procPlaySound = winmm.NewProc("PlaySoundW")
)

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
	procPlaySound.Call(
		uintptr(unsafe.Pointer(&data[0])),
		0,
		uintptr(sndMemory|sndAsync|sndNoDefault),
	)
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
	if workingStop != nil {
		close(workingStop)
	}
	stop := make(chan struct{})
	workingStop = stop
	workingMu.Unlock()

	go func() {
		for {
			// Play synchronously so we know when it ends.
			procPlaySound.Call(
				uintptr(unsafe.Pointer(&workingWAV[0])),
				0,
				uintptr(sndMemory|sndSync|sndNoDefault),
			)
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
	// Stop any currently playing sound — but only if the loop was actually
	// running, so we don't kill a success/error sound that started after us.
	if wasRunning {
		procPlaySound.Call(0, 0, uintptr(sndMemory|sndAsync|sndNoDefault))
	}
}

func SetEnabled(v bool) {
	mu.Lock()
	enabled = v
	mu.Unlock()
}
