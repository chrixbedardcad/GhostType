package llm

import (
	"github.com/chrixbedardcad/GhostSpell/config"
)

const defaultGeminiEndpoint = "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions"

// newGeminiFromDef creates a Google Gemini client from a provider definition.
// Gemini supports an OpenAI-compatible API, so we reuse OpenAIClient with a
// different default endpoint.
func newGeminiFromDef(def config.LLMProviderDef) *OpenAIClient {
	endpoint := def.APIEndpoint
	if endpoint == "" {
		endpoint = defaultGeminiEndpoint
	}
	maxTokens := def.MaxTokens
	if maxTokens == 0 {
		// Gemini thinking models (gemini-2.5-pro, etc.) consume tokens for
		// internal reasoning in addition to the visible output. max_tokens
		// limits the TOTAL (thinking + output). 256 is far too low — the
		// model exhausts the budget on thinking and returns empty content.
		// 8192 gives ample room for both reasoning and output.
		maxTokens = 8192
	}
	timeoutMs := def.TimeoutMs
	if timeoutMs == 0 {
		timeoutMs = 30000
	}

	return &OpenAIClient{
		apiKey:       def.APIKey,
		model:        def.Model,
		endpoint:     endpoint,
		maxTokens:    maxTokens,
		timeoutMs:    timeoutMs,
		providerName: "gemini",
		httpClient:   newPooledHTTPClient(timeoutMs),
	}
}
