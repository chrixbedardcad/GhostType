package sound

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
// Zero external dependencies — works on Windows, macOS, and Linux.
type Recorder struct {
	mu        sync.Mutex
	recording bool
	levelMu   sync.Mutex
	level     float32 // current RMS audio level (0.0–1.0)
	pcm       bytes.Buffer
	pcmMu     sync.Mutex
}

// NewRecorder creates a new audio recorder.
func NewRecorder() *Recorder {
	return &Recorder{}
}

// Level returns the current RMS audio level (0.0–1.0).
// Safe to call concurrently while recording.
func (r *Recorder) Level() float32 {
	r.levelMu.Lock()
	defer r.levelMu.Unlock()
	return r.level
}

// MicAvailable returns true if a microphone is accessible.
func (r *Recorder) MicAvailable() bool {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		slog.Debug("[mic] malgo context init failed", "error", err)
		return false
	}
	defer func() { _ = ctx.Uninit() }()

	devices, err := ctx.Devices(malgo.Capture)
	if err != nil {
		slog.Debug("[mic] no capture devices", "error", err)
		return false
	}

	available := len(devices) > 0
	if available {
		slog.Debug("[mic] capture devices found", "count", len(devices), "default", devices[0].Name())
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

	mctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("audio init: %w", err)
	}
	defer func() {
		_ = mctx.Uninit()
		mctx.Free()
	}()

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = 1
	deviceConfig.SampleRate = 16000
	deviceConfig.PeriodSizeInMilliseconds = 100

	r.pcmMu.Lock()
	r.pcm.Reset()
	r.pcmMu.Unlock()
	start := time.Now()

	onRecvFrames := func(outputSamples, inputSamples []byte, frameCount uint32) {
		r.pcmMu.Lock()
		r.pcm.Write(inputSamples)
		r.pcmMu.Unlock()

		// Compute RMS level from this frame's samples.
		nSamples := len(inputSamples) / 2
		if nSamples > 0 {
			var sumSq float64
			for i := 0; i < nSamples; i++ {
				s := int16(binary.LittleEndian.Uint16(inputSamples[i*2 : i*2+2]))
				f := float64(s) / float64(math.MaxInt16)
				sumSq += f * f
			}
			rms := float32(math.Sqrt(sumSq / float64(nSamples)))
			// Clamp to 0.0–1.0.
			if rms > 1.0 {
				rms = 1.0
			}
			r.levelMu.Lock()
			r.level = rms
			r.levelMu.Unlock()
		}
	}

	device, err := malgo.InitDevice(mctx.Context, deviceConfig, malgo.DeviceCallbacks{Data: onRecvFrames})
	if err != nil {
		return nil, 0, fmt.Errorf("mic init: %w", err)
	}
	defer device.Uninit()

	if err := device.Start(); err != nil {
		return nil, 0, fmt.Errorf("mic start: %w", err)
	}
	defer device.Stop()

	slog.Info("[mic] Recording started", "sampleRate", 16000, "channels", 1)
	fmt.Println("[mic] Recording — speak now...")

	maxTimer := time.NewTimer(120 * time.Second)
	defer maxTimer.Stop()

	select {
	case <-stop:
		slog.Info("[mic] Recording stopped by user")
		fmt.Println("[mic] Stopped")
	case <-ctx.Done():
		slog.Info("[mic] Recording stopped by context")
	case <-maxTimer.C:
		slog.Info("[mic] Max duration reached (120s)")
	}

	duration := time.Since(start)

	r.pcmMu.Lock()
	rawPCM := make([]byte, r.pcm.Len())
	copy(rawPCM, r.pcm.Bytes())
	r.pcmMu.Unlock()

	if len(rawPCM) < 100 {
		return nil, duration, fmt.Errorf("recording too short (%d bytes, %.1fs)", len(rawPCM), duration.Seconds())
	}

	wav := EncodeWAV(rawPCM, 16000, 1)

	slog.Info("[mic] Recording complete", "duration", duration, "pcm_bytes", len(rawPCM), "wav_bytes", len(wav))
	fmt.Printf("[mic] Recorded %.1fs (%d bytes)\n", duration.Seconds(), len(wav))

	return wav, duration, nil
}

// SnapshotWAV returns a WAV-encoded copy of the audio recorded so far.
// Safe to call concurrently while Record() is running.
func (r *Recorder) SnapshotWAV() []byte {
	r.pcmMu.Lock()
	raw := make([]byte, r.pcm.Len())
	copy(raw, r.pcm.Bytes())
	r.pcmMu.Unlock()
	if len(raw) < 100 {
		return nil
	}
	return EncodeWAV(raw, 16000, 1)
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
	binary.LittleEndian.PutUint16(buf[20:22], 1)
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

// PCMToFloat32 converts 16-bit signed PCM to float32 samples [-1.0, 1.0].
func PCMToFloat32(pcm []byte) []float32 {
	nSamples := len(pcm) / 2
	floats := make([]float32, nSamples)
	for i := 0; i < nSamples; i++ {
		sample := int16(binary.LittleEndian.Uint16(pcm[i*2 : i*2+2]))
		floats[i] = float32(sample) / float32(math.MaxInt16)
	}
	return floats
}
