package llm

import "github.com/chrixbedardcad/GhostSpell/llm/ghostai"

// newLocalClient creates the best available local AI client.
// With -tags ghostai: uses the embedded Ghost-AI engine (CGo + llama.cpp).
// Without: falls back to the subprocess-based LocalClient (llama-server).
func newLocalClient(def LLMProviderDefCompat) (Client, error) {
	if ghostai.Available() {
		return newGhostAIFromDef(def)
	}
	return newLocalFromDef(def)
}
