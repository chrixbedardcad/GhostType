package llm

import "github.com/chrixbedardcad/GhostSpell/llm/ghostai"

// GhostAIAvailable reports whether the embedded Ghost-AI engine is compiled in.
func GhostAIAvailable() bool {
	return ghostai.Available()
}
