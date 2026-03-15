package gui

// ModelInfo describes a model with an optional recommendation tag.
type ModelInfo struct {
	Name string `json:"name"`
	Tag  string `json:"tag,omitempty"` // e.g. "recommended", "best", "free", "fast", "cheap"
}

// KnownModels returns a curated list of models for the given provider.
// Models are ordered by recommendation (best first).
func KnownModels(provider string) []ModelInfo {
	switch provider {
	case "anthropic":
		return []ModelInfo{
			{Name: "claude-sonnet-4-6", Tag: "recommended"},
			{Name: "claude-opus-4-6", Tag: "best"},
			{Name: "claude-haiku-4-5-20251001", Tag: "cheap"},
			{Name: "claude-sonnet-4-5-20250929"},
			{Name: "claude-opus-4-5-20251101"},
			{Name: "claude-opus-4-1-20250805"},
			{Name: "claude-sonnet-4-20250514"},
			{Name: "claude-opus-4-20250514"},
		}
	case "openai", "chatgpt":
		return []ModelInfo{
			{Name: "gpt-5-mini", Tag: "recommended"},
			{Name: "gpt-5.4", Tag: "best"},
			{Name: "gpt-5.4-pro"},
			{Name: "gpt-5.3-codex"},
			{Name: "gpt-5.2"},
			{Name: "gpt-5"},
			{Name: "gpt-5-nano", Tag: "fast"},
			{Name: "gpt-4.1-mini"},
			{Name: "gpt-4.1"},
			{Name: "gpt-4.1-nano"},
			{Name: "o3"},
			{Name: "gpt-4o"},
			{Name: "gpt-4o-mini", Tag: "cheap"},
		}
	case "gemini":
		return []ModelInfo{
			{Name: "gemini-2.5-flash", Tag: "recommended"},
			{Name: "gemini-2.5-pro", Tag: "best"},
			{Name: "gemini-2.5-flash-lite", Tag: "cheap"},
			{Name: "gemini-3.1-pro-preview"},
			{Name: "gemini-3-flash-preview"},
			{Name: "gemini-3.1-flash-lite-preview"},
			{Name: "gemini-2.0-flash"},
		}
	case "xai":
		return []ModelInfo{
			{Name: "grok-3-mini", Tag: "recommended"},
			{Name: "grok-4-0709", Tag: "best"},
			{Name: "grok-4-1-fast-reasoning"},
			{Name: "grok-4-1-fast-non-reasoning", Tag: "fast"},
			{Name: "grok-4-fast-reasoning"},
			{Name: "grok-4-fast-non-reasoning"},
			{Name: "grok-code-fast-1"},
			{Name: "grok-3"},
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
	default:
		return nil
	}
}
