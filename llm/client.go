package llm

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/chrixbedardcad/GhostSpell/config"
)

// Request represents a request to an LLM provider.
type Request struct {
	Prompt    string
	Text      string
	MaxTokens int
}

// Response represents a response from an LLM provider.
type Response struct {
	Text     string
	Provider string
	Model    string
}

// Client is the interface all LLM providers must implement.
type Client interface {
	// Send sends a prompt with user text to the LLM and returns the response.
	Send(ctx context.Context, req Request) (*Response, error)

	// Provider returns the name of the provider.
	Provider() string

	// Close releases resources (HTTP connections) held by the client.
	Close()
}

// RefreshOpenAIKeyFunc is a hook for refreshing an OpenAI OAuth token.
// Set by the gui package at startup to avoid circular imports.
// Returns a fresh API key from the refresh token.
var RefreshOpenAIKeyFunc func(refreshToken string) (apiKey string, err error)

// NewClientFromDef creates an LLM client from a provider definition.
// Model tags like "cheap" are resolved to actual model names before
// creating the client.
func NewClientFromDef(def config.LLMProviderDef) (Client, error) {
	def.Model = ResolveModelTag(def.Provider, def.Model)

	// Auto-refresh OpenAI OAuth tokens: if API key is empty but a refresh
	// token exists, exchange it for a fresh API key before creating the client.
	if (def.Provider == "openai" || def.Provider == "chatgpt") && def.APIKey == "" && def.RefreshToken != "" {
		if RefreshOpenAIKeyFunc != nil {
			slog.Info("[llm] OpenAI API key empty, refreshing from OAuth token")
			key, err := RefreshOpenAIKeyFunc(def.RefreshToken)
			if err != nil {
				slog.Error("[llm] OAuth token refresh failed", "error", err)
				return nil, fmt.Errorf("OpenAI OAuth token refresh failed: %w", err)
			}
			def.APIKey = key
			slog.Info("[llm] OpenAI API key refreshed successfully")
		} else {
			return nil, fmt.Errorf("OpenAI API key is empty and no OAuth refresh function is configured")
		}
	}

	switch def.Provider {
	case "anthropic":
		return newAnthropicFromDef(def), nil
	case "openai", "chatgpt":
		return newOpenAIFromDef(def), nil
	case "gemini":
		return newGeminiFromDef(def), nil
	case "xai":
		return newXAIFromDef(def), nil
	case "deepseek":
		return newDeepSeekFromDef(def), nil
	case "ollama":
		return newOllamaFromDef(def), nil
	case "lmstudio":
		return newLMStudioFromDef(def), nil
	case "local":
		return newGhostAIFromDef(LLMProviderDefCompat{
			Model:     def.Model,
			MaxTokens: def.MaxTokens,
			TimeoutMs: def.TimeoutMs,
			KeepAlive: def.KeepAlive,
		})
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", def.Provider)
	}
}
