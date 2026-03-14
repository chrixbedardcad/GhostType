package llm

import (
	"github.com/chrixbedardcad/GhostSpell/config"
)

const defaultXAIEndpoint = "https://api.x.ai/v1/chat/completions"

// newXAIFromDef creates an xAI (Grok) client from a provider definition.
// xAI uses an OpenAI-compatible API, so we reuse OpenAIClient with a
// different default endpoint and provider name.
func newXAIFromDef(def config.LLMProviderDef) *OpenAIClient {
	endpoint := def.APIEndpoint
	if endpoint == "" {
		endpoint = defaultXAIEndpoint
	}
	maxTokens := def.MaxTokens
	if maxTokens == 0 {
		maxTokens = 256
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
		providerName: "xai",
		httpClient:   newPooledHTTPClient(timeoutMs),
	}
}
