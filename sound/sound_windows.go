//go:build windows

package sound

import (
	_ "embed"
	"sync"
	"syscall"
	"unsafe"
)

//go:embed working.wav
var workingWAV []byte

//go:embed success.wav
var successWAV []byte

//go:embed error.wav
var errorWAV []byte

const (
	sndMemory    = 0x0004
	sndAsync     = 0x0001
	sndNoDefault = 0x0002
)

var (
	winmm         = syscall.NewLazyDLL("winmm.dll")
	procPlaySound = winmm.NewProc("PlaySoundW")
)

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
	procPlaySound.Call(
		uintptr(unsafe.Pointer(&data[0])),
		0,
		uintptr(sndMemory|sndAsync|sndNoDefault),
	)
}

func PlayStart()   {}
func PlaySuccess() { play(successWAV) }
func PlayError()   { play(errorWAV) }
func PlayToggle()  {}
func PlayWorking() { play(workingWAV) }

func SetEnabled(v bool) {
	mu.Lock()
	enabled = v
	mu.Unlock()
}
