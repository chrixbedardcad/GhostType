//go:build !windows

package sound

import (
	_ "embed"
	"math/rand"
	"sync"
	"time"
)

//go:embed start.wav
var startWAV []byte

//go:embed working.wav
var workingWAV []byte

//go:embed working2.wav
var working2WAV []byte

//go:embed working3.wav
var working3WAV []byte

//go:embed working4.wav
var working4WAV []byte

//go:embed working5.wav
var working5WAV []byte

//go:embed success.wav
var successWAV []byte

//go:embed success2.wav
var success2WAV []byte

//go:embed success3.wav
var success3WAV []byte

//go:embed success4.wav
var success4WAV []byte

//go:embed success5.wav
var success5WAV []byte

//go:embed error.wav
var errorWAV []byte

//go:embed error2.wav
var error2WAV []byte

//go:embed toggle.wav
var toggleWAV []byte

//go:embed toggle1.wav
var toggle1WAV []byte

//go:embed toggle2.wav
var toggle2WAV []byte

//go:embed toggle3.wav
var toggle3WAV []byte

//go:embed toggle4.wav
var toggle4WAV []byte

//go:embed click1.wav
var click1WAV []byte

//go:embed click2.wav
var click2WAV []byte

//go:embed click3.wav
var click3WAV []byte

//go:embed click4.wav
var click4WAV []byte

//go:embed click5.wav
var click5WAV []byte

//go:embed cancel.wav
var cancelWAV []byte

//go:embed benchmarking.wav
var benchmarkingWAV []byte

//go:embed benchmarking2.wav
var benchmarking2WAV []byte

// pickRandom returns a random element from the list.
func pickRandom(variants [][]byte) []byte {
	if len(variants) == 0 {
		return nil
	}
	return variants[rand.Intn(len(variants))]
}

var (
	enabled bool
	mu      sync.Mutex

	workingStop   chan struct{}
	workingMu     sync.Mutex
	benchStop     chan struct{}
	benchMu       sync.Mutex
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
func PlaySuccess() { play(pickRandom([][]byte{successWAV, success2WAV, success3WAV, success4WAV, success5WAV})) }
func PlayError()   { play(pickRandom([][]byte{errorWAV, error2WAV})) }
func PlayToggle()  { play(pickRandom([][]byte{toggleWAV, toggle1WAV, toggle2WAV, toggle3WAV, toggle4WAV})) }
func PlayClick()   { play(pickRandom([][]byte{click1WAV, click2WAV, click3WAV, click4WAV, click5WAV})) }
func PlayCancel()     { play(cancelWAV) }
func PlayWorking()    { play(workingWAV) }
func PlayBenchmark()  { play(pickRandom([][]byte{benchmarkingWAV, benchmarking2WAV})) }

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

	workingVariants := [][]byte{workingWAV, working2WAV, working3WAV, working4WAV, working5WAV}
	go func() {
		for {
			playWAVLoop(pickRandom(workingVariants)) // blocks until sound finishes; trackable for kill
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

// StartBenchmarkLoop plays benchmarking sounds in a loop until StopBenchmarkLoop is called.
func StartBenchmarkLoop() {
	mu.Lock()
	e := enabled
	mu.Unlock()
	if !e {
		return
	}

	benchMu.Lock()
	if benchStop != nil {
		close(benchStop)
	}
	stop := make(chan struct{})
	benchStop = stop
	benchMu.Unlock()

	variants := [][]byte{benchmarkingWAV, benchmarking2WAV}
	go func() {
		for {
			playWAVLoop(pickRandom(variants))
			select {
			case <-stop:
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
	}()
}

// StopBenchmarkLoop stops the looping benchmark sound.
func StopBenchmarkLoop() {
	benchMu.Lock()
	wasRunning := benchStop != nil
	if wasRunning {
		close(benchStop)
		benchStop = nil
	}
	benchMu.Unlock()
	if wasRunning {
		stopPlayback()
	}
}

func SetEnabled(v bool) {
	mu.Lock()
	enabled = v
	mu.Unlock()
}
