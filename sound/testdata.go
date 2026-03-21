package sound

import _ "embed"

// HumanVoiceTestWAV is a built-in test recording saying "GhostVoice 1 2 3".
// Used by the "Test with sample" button in Settings > Voice to verify
// the whisper engine works without needing a microphone.
//
//go:embed HumanVoiceTest.wav
var HumanVoiceTestWAV []byte
