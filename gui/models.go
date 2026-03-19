package gui

// ModelInfo describes a model with an optional recommendation tag.
type ModelInfo struct {
	Name   string `json:"name"`
	Tag    string `json:"tag,omitempty"`    // e.g. "recommended", "best", "free", "fast", "cheap"
	Vision bool   `json:"vision,omitempty"` // true if the model supports image/vision input
}

// KnownModels returns a curated list of models for the given provider.
// Models are ordered by recommendation (best first).
func KnownModels(provider string) []ModelInfo {
	switch provider {
	case "anthropic":
		return []ModelInfo{
			{Name: "claude-sonnet-4-6", Tag: "recommended", Vision: true},
			{Name: "claude-opus-4-6", Tag: "best", Vision: true},
			{Name: "claude-haiku-4-5-20251001", Tag: "cheap", Vision: true},
			{Name: "claude-sonnet-4-5-20250929", Vision: true},
			{Name: "claude-opus-4-5-20251101", Vision: true},
			{Name: "claude-opus-4-1-20250805", Vision: true},
			{Name: "claude-sonnet-4-20250514", Vision: true},
			{Name: "claude-opus-4-20250514", Vision: true},
		}
	case "openai", "chatgpt":
		return []ModelInfo{
			{Name: "gpt-5-mini", Tag: "recommended", Vision: true},
			{Name: "gpt-5.4", Tag: "best", Vision: true},
			{Name: "gpt-5.4-pro", Vision: true},
			{Name: "gpt-5.3-codex"},
			{Name: "gpt-5.2", Vision: true},
			{Name: "gpt-5", Vision: true},
			{Name: "gpt-5-nano", Tag: "fast"},
			{Name: "gpt-4.1-mini", Vision: true},
			{Name: "gpt-4.1", Vision: true},
			{Name: "gpt-4.1-nano"},
			{Name: "o3", Vision: true},
			{Name: "gpt-4o", Vision: true},
			{Name: "gpt-4o-mini", Tag: "cheap", Vision: true},
		}
	case "gemini":
		return []ModelInfo{
			{Name: "gemini-2.5-flash", Tag: "recommended", Vision: true},
			{Name: "gemini-2.5-pro", Tag: "best", Vision: true},
			{Name: "gemini-2.5-flash-lite", Tag: "cheap", Vision: true},
			{Name: "gemini-3.1-pro-preview", Vision: true},
			{Name: "gemini-3-flash-preview", Vision: true},
			{Name: "gemini-3.1-flash-lite-preview", Vision: true},
			{Name: "gemini-2.0-flash", Vision: true},
		}
	case "xai":
		return []ModelInfo{
			{Name: "grok-3-mini", Tag: "recommended"},
			{Name: "grok-4-0709", Tag: "best", Vision: true},
			{Name: "grok-4-1-fast-reasoning", Vision: true},
			{Name: "grok-4-1-fast-non-reasoning", Tag: "fast"},
			{Name: "grok-4-fast-reasoning", Vision: true},
			{Name: "grok-4-fast-non-reasoning"},
			{Name: "grok-code-fast-1"},
			{Name: "grok-3", Vision: true},
		}
	case "deepseek":
		return []ModelInfo{
			{Name: "deepseek-chat", Tag: "recommended"},
			{Name: "deepseek-reasoner", Tag: "best"},
		}
	case "local":
		return []ModelInfo{
			{Name: "qwen3.5-2b", Tag: "recommended"},
			{Name: "qwen3.5-0.8b", Tag: "fast"},
			{Name: "qwen3.5-4b", Tag: "best"},
			{Name: "nemotron-nano-4b"},
		}
	case "ollama":
		return []ModelInfo{
			{Name: "qwen3.5:4b", Tag: "recommended"},
			{Name: "llama3.3", Tag: "best"},
			{Name: "qwen3.5:9b"},
			{Name: "qwen3.5:2b", Tag: "fast"},
			{Name: "qwen3.5:0.8b"},
			{Name: "mistral"},
			{Name: "llama3.2"},
			{Name: "llama3"},
			{Name: "qwen2.5"},
			{Name: "deepseek-r1"},
			{Name: "gemma2"},
			{Name: "phi3"},
		}
	case "lmstudio":
		// LM Studio serves whatever model is loaded. Use a placeholder —
		// the actual model name doesn't matter for LM Studio's API.
		return []ModelInfo{
			{Name: "default", Tag: "recommended"},
		}
	default:
		return nil
	}
}
