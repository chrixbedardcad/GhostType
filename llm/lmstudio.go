package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/chrixbedardcad/GhostSpell/config"
)

// Use 127.0.0.1 instead of localhost: on Windows, Go resolves localhost
// to IPv6 [::1] first, but LM Studio only listens on IPv4.
const defaultLMStudioEndpoint = "http://127.0.0.1:1234/v1"

// newLMStudioFromDef creates an OpenAI-compatible client pointed at an LM Studio server.
// LM Studio exposes an OpenAI-compatible API, so we reuse OpenAIClient with a
// custom endpoint and providerName.
func newLMStudioFromDef(def config.LLMProviderDef) *OpenAIClient {
	endpoint := def.APIEndpoint
	if endpoint == "" {
		endpoint = defaultLMStudioEndpoint
	}
	maxTokens := def.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1024
	}
	timeoutMs := def.TimeoutMs
	if timeoutMs == 0 {
		timeoutMs = 120000
	}

	return &OpenAIClient{
		apiKey:       def.APIKey, // often empty for local LM Studio
		model:        def.Model,
		endpoint:     endpoint + "/chat/completions",
		maxTokens:    maxTokens,
		timeoutMs:    timeoutMs,
		providerName: "lmstudio",
		httpClient:   newPooledHTTPClient(timeoutMs),
	}
}

// lmStudioModelsResponse is the response from the LM Studio /v1/models endpoint.
type lmStudioModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// LMStudioStatus checks if an LM Studio server is running at the given endpoint
// and returns the list of available models.
func LMStudioStatus(endpoint string) (running bool, models []string, err error) {
	if endpoint == "" {
		endpoint = defaultLMStudioEndpoint
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(endpoint + "/models")
	if err != nil {
		return false, nil, fmt.Errorf("LM Studio not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil, fmt.Errorf("LM Studio returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, nil, fmt.Errorf("failed to read response: %w", err)
	}

	var modelsResp lmStudioModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return false, nil, fmt.Errorf("failed to parse models response: %w", err)
	}

	for _, m := range modelsResp.Data {
		models = append(models, m.ID)
	}

	return true, models, nil
}
