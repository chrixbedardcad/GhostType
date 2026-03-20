// Package audio provides cross-platform microphone recording using miniaudio.
// Used by the voice input pipeline to capture audio for speech-to-text.
//
// Uses malgo (github.com/gen2brain/malgo) — Go bindings for miniaudio.
// Zero external dependencies: no sox, ffmpeg, arecord, or system libraries.
// Works on Windows, macOS, and Linux out of the box.
package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/gen2brain/malgo"
)

// Recorder captures audio from the system microphone using miniaudio.
type Recorder struct {
	mu        sync.Mutex
	recording bool
	ctx       *malgo.AllocatedContext
}

// NewRecorder creates a new audio recorder.
func NewRecorder() *Recorder {
	return &Recorder{}
}

// Available returns true if a microphone is accessible.
func (r *Recorder) Available() bool {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		slog.Debug("[audio] malgo context init failed", "error", err)
		return false
	}
	defer func() { _ = ctx.Uninit() }()

	// Try to enumerate capture devices.
	devices, err := ctx.Devices(malgo.Capture)
	if err != nil {
		slog.Debug("[audio] no capture devices", "error", err)
		return false
	}

	available := len(devices) > 0
	if available {
		slog.Debug("[audio] capture devices found", "count", len(devices), "default", devices[0].Name())
	}
	return available
}

// Record captures audio from the default microphone until ctx is cancelled
// or stop is closed. Returns WAV data (16kHz, mono, 16-bit).
func (r *Recorder) Record(ctx context.Context, stop <-chan struct{}) ([]byte, time.Duration, error) {
	r.mu.Lock()
	if r.recording {
		r.mu.Unlock()
		return nil, 0, fmt.Errorf("already recording")
	}
	r.recording = true
	r.mu.Unlock()
	defer func() { r.mu.Lock(); r.recording = false; r.mu.Unlock() }()

	// Initialize miniaudio context.
	mctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("audio init: %w", err)
	}
	defer func() {
		_ = mctx.Uninit()
		mctx.Free()
	}()

	// Configure capture: 16kHz, mono, 16-bit signed integer.
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = 1
	deviceConfig.SampleRate = 16000
	deviceConfig.PeriodSizeInMilliseconds = 100 // 100ms buffer periods

	var pcm bytes.Buffer
	var pcmMu sync.Mutex
	start := time.Now()

	// Callback receives audio data from the microphone.
	onRecvFrames := func(outputSamples, inputSamples []byte, frameCount uint32) {
		pcmMu.Lock()
		pcm.Write(inputSamples)
		pcmMu.Unlock()
	}

	callbacks := malgo.DeviceCallbacks{
		Data: onRecvFrames,
	}

	device, err := malgo.InitDevice(mctx.Context, deviceConfig, callbacks)
	if err != nil {
		return nil, 0, fmt.Errorf("mic init: %w", err)
	}
	defer device.Uninit()

	if err := device.Start(); err != nil {
		return nil, 0, fmt.Errorf("mic start: %w", err)
	}
	defer device.Stop()

	slog.Info("[audio] Recording started (malgo/miniaudio)", "sampleRate", 16000, "channels", 1, "format", "s16")
	fmt.Println("[audio] Recording started — speak now...")

	// Wait for stop signal, context cancellation, or max duration (120s).
	maxTimer := time.NewTimer(120 * time.Second)
	defer maxTimer.Stop()

	select {
	case <-stop:
		slog.Info("[audio] Recording stopped by user")
		fmt.Println("[audio] Recording stopped")
	case <-ctx.Done():
		slog.Info("[audio] Recording stopped by context")
	case <-maxTimer.C:
		slog.Info("[audio] Recording reached max duration (120s)")
	}

	duration := time.Since(start)

	pcmMu.Lock()
	rawPCM := pcm.Bytes()
	pcmMu.Unlock()

	if len(rawPCM) < 100 {
		return nil, duration, fmt.Errorf("recording too short (%d bytes, %.1fs)", len(rawPCM), duration.Seconds())
	}

	// Encode as WAV.
	wav := EncodeWAV(rawPCM, 16000, 1)

	slog.Info("[audio] Recording complete",
		"duration", duration,
		"pcm_bytes", len(rawPCM),
		"wav_bytes", len(wav),
		"samples", len(rawPCM)/2,
		"seconds", float64(len(rawPCM)/2)/16000.0,
	)
	fmt.Printf("[audio] Recorded %.1fs (%d bytes)\n", duration.Seconds(), len(wav))

	return wav, duration, nil
}

// EncodeWAV wraps raw 16-bit PCM samples in a WAV container.
func EncodeWAV(pcm []byte, sampleRate, channels int) []byte {
	bitsPerSample := 16
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	dataSize := len(pcm)
	fileSize := 36 + dataSize

	buf := make([]byte, 44+dataSize)
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(fileSize))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(buf[22:24], uint16(channels))
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(buf[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(buf[34:36], uint16(bitsPerSample))
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(dataSize))
	copy(buf[44:], pcm)

	return buf
}

// PCMToFloat32 converts 16-bit signed PCM bytes to float32 samples [-1.0, 1.0].
// Used by whisper.cpp which expects float32 input.
func PCMToFloat32(pcm []byte) []float32 {
	nSamples := len(pcm) / 2
	floats := make([]float32, nSamples)
	for i := 0; i < nSamples; i++ {
		sample := int16(binary.LittleEndian.Uint16(pcm[i*2 : i*2+2]))
		floats[i] = float32(sample) / float32(math.MaxInt16)
	}
	return floats
}
