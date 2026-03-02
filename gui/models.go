package gui

// KnownModels returns a curated list of models for the given provider.
// Models are ordered by recommendation (newest/best first).
// Users can always enter a custom model name via the "(custom model...)" option.
func KnownModels(provider string) []string {
	switch provider {
	case "anthropic":
		return []string{
			// Latest generation (4.6)
			"claude-sonnet-4-6",
			"claude-opus-4-6",
			"claude-haiku-4-5-20251001",
			// Previous generation (still available)
			"claude-sonnet-4-5-20250929",
			"claude-opus-4-5-20251101",
			"claude-sonnet-4-20250514",
		}
	case "openai":
		return []string{
			// GPT-5.2 (flagship)
			"gpt-5.2",
			// GPT-5 family
			"gpt-5",
			"gpt-5-mini",
			"gpt-5-nano",
			// GPT-4.1 family
			"gpt-4.1",
			"gpt-4.1-mini",
			"gpt-4.1-nano",
			// Reasoning models
			"o4-mini",
			"o3-mini",
			// GPT-4o (previous gen, still widely used)
			"gpt-4o",
			"gpt-4o-mini",
		}
	case "gemini":
		return []string{
			// Latest stable
			"gemini-2.5-pro",
			"gemini-2.5-flash",
			"gemini-2.5-flash-lite",
			// Preview (next gen)
			"gemini-3.1-pro-preview",
			"gemini-3-flash-preview",
			// Previous gen (retiring June 2026)
			"gemini-2.0-flash",
		}
	case "xai":
		return []string{
			"grok-4-1-fast-reasoning",
			"grok-4-1-fast-non-reasoning",
			"grok-4-0709",
			"grok-3",
			"grok-3-mini",
		}
	case "ollama":
		return []string{
			"llama3.3",
			"llama3.2",
			"llama3",
			"qwen2.5",
			"deepseek-r1",
			"mistral",
			"gemma2",
			"phi3",
		}
	default:
		return nil
	}
}
