//go:build !windows

package sound

import (
	"embed"
	"io/fs"
	"math/rand/v2"
	"os"
	"os/exec"
	"strings"
	"sync"
)

//go:embed *.wav
var soundFS embed.FS

type soundGroup struct {
	variants [][]byte
}

func (sg *soundGroup) play() {
	if len(sg.variants) == 0 {
		return
	}
	wav := sg.variants[rand.IntN(len(sg.variants))]
	go playWAV(wav)
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

// playWAVWithCommand writes WAV data to a temp file and plays it with the given command.
func playWAVWithCommand(data []byte, player string) {
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
	// Clean up temp file after playback completes (in background).
	go func() {
		cmd.Wait()
		os.Remove(tmpPath)
	}()
}
