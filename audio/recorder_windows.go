//go:build windows

package audio

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// Windows waveIn API for microphone recording — no external tools needed.
// Uses winmm.dll, available on all Windows versions.

var (
	winmm            = syscall.NewLazyDLL("winmm.dll")
	procWaveInOpen   = winmm.NewProc("waveInOpen")
	procWaveInStart  = winmm.NewProc("waveInStart")
	procWaveInStop   = winmm.NewProc("waveInStop")
	procWaveInReset  = winmm.NewProc("waveInReset")
	procWaveInClose  = winmm.NewProc("waveInClose")
	procWaveInPrepH  = winmm.NewProc("waveInPrepareHeader")
	procWaveInUnprepH = winmm.NewProc("waveInUnprepareHeader")
	procWaveInAddBuf = winmm.NewProc("waveInAddBuffer")
)

const (
	waveMapper       = 0xFFFFFFFF
	waveFmtPCM       = 1
	callbackNull     = 0
	whdrDone         = 0x01
	bufferSize       = 16000 * 2 // 1 second of 16kHz 16-bit mono
	maxBuffers       = 120       // 120 seconds max recording
)

type waveFormatEx struct {
	FormatTag      uint16
	Channels       uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	Size           uint16
}

type waveHdr struct {
	Data          uintptr
	BufferLength  uint32
	BytesRecorded uint32
	User          uintptr
	Flags         uint32
	Loops         uint32
	Next          uintptr
	Reserved      uintptr
}

func (r *CommandRecorder) Available() bool {
	return true // winmm.dll is always available on Windows
}

func (r *CommandRecorder) Record(ctx context.Context, stop <-chan struct{}) ([]byte, time.Duration, error) {
	slog.Info("[audio] Windows: recording via waveIn API (winmm.dll)")
	start := time.Now()

	// Set up wave format: 16kHz, mono, 16-bit.
	wfx := waveFormatEx{
		FormatTag:      waveFmtPCM,
		Channels:       1,
		SamplesPerSec:  16000,
		AvgBytesPerSec: 32000,
		BlockAlign:     2,
		BitsPerSample:  16,
		Size:           0,
	}

	var hWaveIn uintptr
	ret, _, _ := procWaveInOpen.Call(
		uintptr(unsafe.Pointer(&hWaveIn)),
		uintptr(waveMapper),
		uintptr(unsafe.Pointer(&wfx)),
		0, 0, callbackNull,
	)
	if ret != 0 {
		return nil, 0, fmt.Errorf("waveInOpen failed: error %d (no microphone?)", ret)
	}
	defer procWaveInClose.Call(hWaveIn)

	// Allocate double-buffered recording.
	var buffers [2][]byte
	var headers [2]waveHdr
	for i := range buffers {
		buffers[i] = make([]byte, bufferSize)
		headers[i] = waveHdr{
			Data:         uintptr(unsafe.Pointer(&buffers[i][0])),
			BufferLength: uint32(bufferSize),
		}
		procWaveInPrepH.Call(hWaveIn, uintptr(unsafe.Pointer(&headers[i])), unsafe.Sizeof(headers[i]))
		procWaveInAddBuf.Call(hWaveIn, uintptr(unsafe.Pointer(&headers[i])), unsafe.Sizeof(headers[i]))
	}

	// Start recording.
	ret, _, _ = procWaveInStart.Call(hWaveIn)
	if ret != 0 {
		return nil, 0, fmt.Errorf("waveInStart failed: error %d", ret)
	}

	// Collect PCM data until stopped.
	var pcm bytes.Buffer
	var mu sync.Mutex
	done := make(chan struct{})

	go func() {
		defer close(done)
		for i := 0; pcm.Len() < bufferSize*maxBuffers; i++ {
			idx := i % 2
			// Wait for buffer to be filled.
			for {
				mu.Lock()
				flags := headers[idx].Flags
				mu.Unlock()
				if flags&whdrDone != 0 {
					break
				}
				select {
				case <-stop:
					return
				case <-ctx.Done():
					return
				default:
					time.Sleep(10 * time.Millisecond)
				}
			}

			// Copy recorded data.
			recorded := headers[idx].BytesRecorded
			if recorded > 0 {
				pcm.Write(buffers[idx][:recorded])
			}

			// Reset and requeue buffer.
			mu.Lock()
			headers[idx].Flags = 0
			headers[idx].BytesRecorded = 0
			mu.Unlock()
			procWaveInAddBuf.Call(hWaveIn, uintptr(unsafe.Pointer(&headers[idx])), unsafe.Sizeof(headers[idx]))
		}
	}()

	// Wait for stop signal.
	select {
	case <-stop:
		slog.Info("[audio] Recording stopped by user")
	case <-ctx.Done():
		slog.Info("[audio] Recording stopped by context")
	case <-done:
		slog.Info("[audio] Recording reached max duration")
	}

	// Stop and clean up.
	procWaveInStop.Call(hWaveIn)
	procWaveInReset.Call(hWaveIn)
	time.Sleep(50 * time.Millisecond) // let buffers flush

	// Unprepare headers.
	for i := range headers {
		procWaveInUnprepH.Call(hWaveIn, uintptr(unsafe.Pointer(&headers[i])), unsafe.Sizeof(headers[i]))
	}

	duration := time.Since(start)
	rawPCM := pcm.Bytes()

	if len(rawPCM) < 100 {
		return nil, duration, fmt.Errorf("recording too short (%d bytes)", len(rawPCM))
	}

	// Wrap in WAV container.
	wav := EncodeWAV(rawPCM, 16000, 1)

	slog.Info("[audio] Recording complete", "duration", duration, "pcm_size", len(rawPCM), "wav_size", len(wav))
	return wav, duration, nil
}
