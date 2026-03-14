package llm

import (
	"github.com/chrixbedardcad/GhostSpell/config"
)

const defaultDeepSeekEndpoint = "https://api.deepseek.com/chat/completions"

// newDeepSeekFromDef creates a DeepSeek client from a provider definition.
// DeepSeek uses an OpenAI-compatible API, so we reuse OpenAIClient with a
// different default endpoint and provider name.
func newDeepSeekFromDef(def config.LLMProviderDef) *OpenAIClient {
	endpoint := def.APIEndpoint
	if endpoint == "" {
		endpoint = defaultDeepSeekEndpoint
	}
	maxTokens := def.MaxTokens
	if maxTokens == 0 {
		maxTokens = 2048
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
		providerName: "deepseek",
		httpClient:   newPooledHTTPClient(timeoutMs),
	}
}
