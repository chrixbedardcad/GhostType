package llm

import "testing"

func TestResolveModelTag_Cheap(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"anthropic", "claude-haiku-4-5-20251001"},
		{"openai", "gpt-4o-mini"},
		{"gemini", "gemini-2.5-flash-lite"},
		{"xai", "grok-3-mini"},
	}

	for _, tt := range tests {
		got := ResolveModelTag(tt.provider, "cheap")
		if got != tt.want {
			t.Errorf("ResolveModelTag(%q, \"cheap\") = %q, want %q", tt.provider, got, tt.want)
		}
	}
}

func TestResolveModelTag_CheapUnknownProvider(t *testing.T) {
	// Unknown provider returns "cheap" unchanged.
	got := ResolveModelTag("ollama", "cheap")
	if got != "cheap" {
		t.Errorf("ResolveModelTag(\"ollama\", \"cheap\") = %q, want \"cheap\"", got)
	}
}

func TestResolveModelTag_RegularModel(t *testing.T) {
	// Non-tag model names pass through unchanged.
	got := ResolveModelTag("anthropic", "claude-sonnet-4-5-20250929")
	if got != "claude-sonnet-4-5-20250929" {
		t.Errorf("ResolveModelTag returned %q, want claude-sonnet-4-5-20250929", got)
	}
}

func TestResolveModelTag_EmptyModel(t *testing.T) {
	got := ResolveModelTag("openai", "")
	if got != "" {
		t.Errorf("ResolveModelTag returned %q, want empty", got)
	}
}
