package llm

import (
	"context"
	"fmt"

	"github.com/chrixbedardcad/GhostType/config"
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
}

// NewClient creates an LLM client based on the config.
func NewClient(cfg *config.Config) (Client, error) {
	switch cfg.LLMProvider {
	case "anthropic":
		return NewAnthropicClient(cfg), nil
	case "openai":
		return NewOpenAIClient(cfg), nil
	case "gemini":
		return NewClientFromDef(config.LLMProviderDef{
			Provider:    "gemini",
			APIKey:      cfg.APIKey,
			Model:       cfg.Model,
			APIEndpoint: cfg.APIEndpoint,
			MaxTokens:   cfg.MaxTokens,
			TimeoutMs:   cfg.TimeoutMs,
		})
	case "xai":
		return NewClientFromDef(config.LLMProviderDef{
			Provider:    "xai",
			APIKey:      cfg.APIKey,
			Model:       cfg.Model,
			APIEndpoint: cfg.APIEndpoint,
			MaxTokens:   cfg.MaxTokens,
			TimeoutMs:   cfg.TimeoutMs,
		})
	case "ollama":
		return NewOllamaClient(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.LLMProvider)
	}
}

// NewClientFromDef creates an LLM client from a provider definition.
// Model tags like "cheap" are resolved to actual model names before
// creating the client.
func NewClientFromDef(def config.LLMProviderDef) (Client, error) {
	def.Model = ResolveModelTag(def.Provider, def.Model)

	switch def.Provider {
	case "anthropic":
		return newAnthropicFromDef(def), nil
	case "openai":
		return newOpenAIFromDef(def), nil
	case "gemini":
		return newGeminiFromDef(def), nil
	case "xai":
		return newXAIFromDef(def), nil
	case "ollama":
		return newOllamaFromDef(def), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", def.Provider)
	}
}
