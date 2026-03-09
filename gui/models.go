package gui

// ModelInfo describes a model with an optional recommendation tag.
type ModelInfo struct {
	Name string `json:"name"`
	Tag  string `json:"tag,omitempty"` // e.g. "recommended", "best", "free", "fast"
}

// KnownModels returns a curated list of models for the given provider.
// Models are ordered by recommendation (best first).
func KnownModels(provider string) []ModelInfo {
	switch provider {
	case "anthropic":
		return []ModelInfo{
			{Name: "cheap", Tag: "cheap"},
			{Name: "claude-sonnet-4-6", Tag: "recommended"},
			{Name: "claude-opus-4-6", Tag: "best"},
			{Name: "claude-haiku-4-5-20251001", Tag: "fast"},
			{Name: "claude-sonnet-4-5-20250929"},
			{Name: "claude-opus-4-5-20251101"},
			{Name: "claude-sonnet-4-20250514"},
		}
	case "openai":
		return []ModelInfo{
			{Name: "cheap", Tag: "cheap"},
			{Name: "gpt-4.1-mini", Tag: "recommended"},
			{Name: "gpt-5.2", Tag: "best"},
			{Name: "gpt-5"},
			{Name: "gpt-5-mini"},
			{Name: "gpt-5-nano", Tag: "fast"},
			{Name: "gpt-4.1"},
			{Name: "gpt-4.1-nano"},
			{Name: "o4-mini"},
			{Name: "o3-mini"},
			{Name: "gpt-4o"},
			{Name: "gpt-4o-mini"},
		}
	case "gemini":
		return []ModelInfo{
			{Name: "cheap", Tag: "cheap"},
			{Name: "gemini-2.5-flash", Tag: "recommended"},
			{Name: "gemini-2.5-pro", Tag: "best"},
			{Name: "gemini-2.5-flash-lite", Tag: "free"},
			{Name: "gemini-3.1-pro-preview"},
			{Name: "gemini-3-flash-preview"},
			{Name: "gemini-2.0-flash"},
		}
	case "xai":
		return []ModelInfo{
			{Name: "cheap", Tag: "cheap"},
			{Name: "grok-3-mini", Tag: "recommended"},
			{Name: "grok-4-0709", Tag: "best"},
			{Name: "grok-4-1-fast-reasoning"},
			{Name: "grok-4-1-fast-non-reasoning", Tag: "fast"},
			{Name: "grok-3"},
		}
	case "ollama":
		return []ModelInfo{
			{Name: "mistral", Tag: "recommended"},
			{Name: "llama3.3", Tag: "best"},
			{Name: "llama3.2"},
			{Name: "llama3"},
			{Name: "qwen2.5"},
			{Name: "deepseek-r1"},
			{Name: "gemma2"},
			{Name: "phi3", Tag: "fast"},
		}
	default:
		return nil
	}
}
