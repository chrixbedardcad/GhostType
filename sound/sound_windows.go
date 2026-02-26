//go:build windows

package sound

import (
	"embed"
	"io/fs"
	"math/rand/v2"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

//go:embed *.wav
var soundFS embed.FS

const (
	sndMemory    = 0x0004
	sndAsync     = 0x0001
	sndNoDefault = 0x0002
)

var (
	winmm         = syscall.NewLazyDLL("winmm.dll")
	procPlaySound = winmm.NewProc("PlaySoundW")
)

type soundGroup struct {
	variants [][]byte
}

func (sg *soundGroup) play() {
	if len(sg.variants) == 0 {
		return
	}
	wav := sg.variants[rand.IntN(len(sg.variants))]
	procPlaySound.Call(
		uintptr(unsafe.Pointer(&wav[0])),
		0,
		uintptr(sndMemory|sndAsync|sndNoDefault),
	)
}

var (
	enabled bool
	mu      sync.Mutex
	groups  map[string]*soundGroup
)

// Init loads all embedded WAV files and groups them by prefix.
func Init(soundEnabled bool) {
	enabled = soundEnabled
	groups = make(map[string]*soundGroup)

	entries, _ := fs.Glob(soundFS, "*.wav")
	for _, name := range entries {
		prefix := extractPrefix(name)
		data, _ := soundFS.ReadFile(name)
		if groups[prefix] == nil {
			groups[prefix] = &soundGroup{}
		}
		groups[prefix].variants = append(groups[prefix].variants, data)
	}
}

// extractPrefix strips .wav and trailing digits: "toggle3.wav" → "toggle".
func extractPrefix(filename string) string {
	name := strings.TrimSuffix(filename, ".wav")
	return strings.TrimRight(name, "0123456789")
}

func playGroup(name string) {
	mu.Lock()
	e := enabled
	mu.Unlock()
	if !e {
		return
	}
	if g, ok := groups[name]; ok {
		g.play()
	}
}

func PlayStart()   { playGroup("start") }
func PlaySuccess() { playGroup("sucess") } // matches filename spelling
func PlayError()   { playGroup("error") }
func PlayToggle()  { playGroup("toggle") }
func PlayWorking() { playGroup("working") }

func SetEnabled(v bool) {
	mu.Lock()
	enabled = v
	mu.Unlock()
}
