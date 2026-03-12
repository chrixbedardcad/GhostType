package llm

import "log/slog"

// Model tags — special model names that resolve to specific models per provider.
// Users can set "model": "cheap" in their config and it resolves to the
// cheapest available model for that provider at client creation time.

// cheapModels maps provider name → cheapest model ID.
var cheapModels = map[string]string{
	"anthropic": "claude-haiku-4-5-20251001",
	"openai":    "gpt-4o-mini",
	"gemini":    "gemini-2.5-flash-lite",
	"xai":       "grok-3-mini",
	"deepseek":  "deepseek-chat",
	"local":     "qwen3-0.6b",
}

// ResolveModelTag checks if the model string is a known tag (e.g. "cheap")
// and resolves it to the actual model name for the given provider.
// If the model is not a tag, it is returned unchanged.
func ResolveModelTag(provider, model string) string {
	switch model {
	case "cheap":
		if resolved, ok := cheapModels[provider]; ok {
			slog.Info("Resolved model tag", "tag", model, "provider", provider, "model", resolved)
			return resolved
		}
		// Unknown provider (e.g. ollama) — return unchanged.
		slog.Warn("No cheap model defined for provider, using model name as-is", "provider", provider, "model", model)
		return model
	default:
		return model
	}
}
