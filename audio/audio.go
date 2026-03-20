// Package audio provides cross-platform microphone recording.
// Used by the voice input pipeline to capture audio for speech-to-text.
package audio

import (
	"encoding/binary"
)

// CommandRecorder is the shared recorder type used on all platforms.
// Platform-specific files implement Available() and Record() methods.
type CommandRecorder struct {
	cmd     interface{} // *exec.Cmd on Unix, unused on Windows
	tmpFile string
}

// NewRecorder creates a new audio recorder.
func NewRecorder() *CommandRecorder {
	return &CommandRecorder{}
}

// EncodeWAV wraps raw 16-bit PCM samples in a WAV container.
// sampleRate: typically 16000 for STT.
// channels: 1 for mono.
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
	binary.LittleEndian.PutUint32(buf[16:20], 16) // PCM chunk size
	binary.LittleEndian.PutUint16(buf[20:22], 1)  // PCM format
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
