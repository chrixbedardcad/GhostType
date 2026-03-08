package mode

import (
	"context"
	"fmt"
	"testing"

	"github.com/chrixbedardcad/GhostType/config"
	"github.com/chrixbedardcad/GhostType/llm"
)

// mockClient is a test double for llm.Client.
type mockClient struct {
	response *llm.Response
	err      error
	lastReq  llm.Request
}

func (m *mockClient) Send(_ context.Context, req llm.Request) (*llm.Response, error) {
	m.lastReq = req
	return m.response, m.err
}

func (m *mockClient) Provider() string {
	return "mock"
}

func newTestConfig() *config.Config {
	return &config.Config{
		LLMProvider: "anthropic",
		APIKey:      "test-key",
		Model:       "claude-sonnet-4-5-20250929",
		Prompts: []config.PromptEntry{
			{Name: "Correct", Prompt: "Fix spelling and grammar. Return ONLY corrected text."},
			{Name: "Polish", Prompt: "Improve the text. Return ONLY the polished text."},
			{Name: "Translate", Prompt: "Translateglish. Return ONLY the translation."},
		},
		ActivePrompt: 0,
		MaxTokens:    256,
	}
}

func TestRouter_ProcessCorrect(t *testing.T) {
	cfg := newTestConfig()
	mock := &mockClient{
		response: &llm.Response{Text: "Hello, how are you?", Provider: "mock", Model: "test"},
	}
	router := NewRouter(cfg, mock)

	result, err := router.Process(context.Background(), 0, "Helo, how are yu?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "Hello, how are you?" {
		t.Errorf("expected 'Hello, how are you?', got '%s'", result)
	}

	if mock.lastReq.Prompt != cfg.Prompts[0].Prompt {
		t.Errorf("expected correct prompt, got '%s'", mock.lastReq.Prompt)
	}
}

func TestRouter_ProcessPolish(t *testing.T) {
	cfg := newTestConfig()
	mock := &mockClient{
		response: &llm.Response{Text: "Polished text here", Provider: "mock", Model: "test"},
	}
	router := NewRouter(cfg, mock)

	result, err := router.Process(context.Background(), 1, "Some rough text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "Polished text here" {
		t.Errorf("expected polished text, got '%s'", result)
	}

	if mock.lastReq.Prompt != cfg.Prompts[1].Prompt {
		t.Errorf("expected polish prompt, got '%s'", mock.lastReq.Prompt)
	}
}

func TestRouter_ProcessTranslate(t *testing.T) {
	cfg := newTestConfig()
	mock := &mockClient{
		response: &llm.Response{Text: "Translated text", Provider: "mock", Model: "test"},
	}
	router := NewRouter(cfg, mock)

	result, err := router.Process(context.Background(), 2, "Bonjour")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "Translated text" {
		t.Errorf("expected translated text, got '%s'", result)
	}

	if mock.lastReq.Prompt != cfg.Prompts[2].Prompt {
		t.Errorf("expected translate prompt, got '%s'", mock.lastReq.Prompt)
	}
}

func TestRouter_ProcessEmptyText(t *testing.T) {
	cfg := newTestConfig()
	mock := &mockClient{}
	router := NewRouter(cfg, mock)

	_, err := router.Process(context.Background(), 0, "")
	if err == nil {
		t.Fatal("expected error for empty text")
	}

	_, err = router.Process(context.Background(), 0, "   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only text")
	}
}

func TestRouter_ProcessInvalidIndex(t *testing.T) {
	cfg := newTestConfig()
	mock := &mockClient{
		response: &llm.Response{Text: "result", Provider: "mock", Model: "test"},
	}
	router := NewRouter(cfg, mock)

	_, err := router.Process(context.Background(), -1, "test text")
	if err == nil {
		t.Fatal("expected error for negative index")
	}

	_, err = router.Process(context.Background(), 99, "test text")
	if err == nil {
		t.Fatal("expected error for out-of-range index")
	}
}

func TestRouter_ProcessLLMError(t *testing.T) {
	cfg := newTestConfig()
	mock := &mockClient{
		err: fmt.Errorf("API timeout"),
	}
	router := NewRouter(cfg, mock)

	_, err := router.Process(context.Background(), 0, "test text")
	if err == nil {
		t.Fatal("expected error when LLM fails")
	}
}

func TestRouter_CyclePrompt(t *testing.T) {
	cfg := newTestConfig()
	mock := &mockClient{}
	router := NewRouter(cfg, mock)

	// Default is first prompt ("Correct")
	if router.CurrentPromptName() != "Correct" {
		t.Errorf("expected default prompt 'Correct', got '%s'", router.CurrentPromptName())
	}

	// Cycle to "Polish"
	idx, name := router.CyclePrompt()
	if name != "Polish" || idx != 1 {
		t.Errorf("expected 'Polish' at index 1, got '%s' at %d", name, idx)
	}

	// Cycle to "Translate"
	idx, name = router.CyclePrompt()
	if name != "Translate" || idx != 2 {
		t.Errorf("expected 'Translate' at index 2, got '%s' at %d", name, idx)
	}

	// Cycle back to "Correct"
	idx, name = router.CyclePrompt()
	if name != "Correct" || idx != 0 {
		t.Errorf("expected 'Correct' at index 0, got '%s' at %d", name, idx)
	}
}

func TestRouter_SetPrompt(t *testing.T) {
	cfg := newTestConfig()
	mock := &mockClient{}
	router := NewRouter(cfg, mock)

	// Default index is 0 ("Correct")
	if router.CurrentPromptIdx() != 0 {
		t.Errorf("expected default index 0, got %d", router.CurrentPromptIdx())
	}

	// Set to index 1 ("Polish")
	name := router.SetPrompt(1)
	if name != "Polish" {
		t.Errorf("expected 'Polish', got '%s'", name)
	}
	if router.CurrentPromptIdx() != 1 {
		t.Errorf("expected index 1, got %d", router.CurrentPromptIdx())
	}
	if router.CurrentPromptName() != "Polish" {
		t.Errorf("expected 'Polish', got '%s'", router.CurrentPromptName())
	}

	// Set to index 2 ("Translate")
	name = router.SetPrompt(2)
	if name != "Translate" {
		t.Errorf("expected 'Translate', got '%s'", name)
	}

	// Out-of-bounds returns empty
	name = router.SetPrompt(-1)
	if name != "" {
		t.Errorf("expected empty for negative index, got '%s'", name)
	}
	name = router.SetPrompt(99)
	if name != "" {
		t.Errorf("expected empty for out-of-range index, got '%s'", name)
	}
}

func TestRouter_PerPromptLLM(t *testing.T) {
	cfg := &config.Config{
		DefaultLLM: "claude",
		LLMProviders: map[string]config.LLMProviderDef{
			"claude": {Provider: "anthropic", APIKey: "sk-ant", Model: "claude-sonnet-4-5-20250929"},
			"gpt":    {Provider: "openai", APIKey: "sk-oai", Model: "gpt-4o"},
		},
		Prompts: []config.PromptEntry{
			{Name: "Correct", Prompt: "Fix errors.", LLM: "gpt"},
			{Name: "Polish", Prompt: "Improve text."},
		},
		ActivePrompt: 0,
		MaxTokens:    256,
	}
	mock := &mockClient{
		response: &llm.Response{Text: "result", Provider: "mock", Model: "test"},
	}
	router := NewRouter(cfg, mock)

	// First prompt ("Correct") has LLM: "gpt"
	label := router.llmLabelForPrompt(0)
	if label != "gpt" {
		t.Errorf("expected llm label 'gpt' for Correct prompt, got %q", label)
	}

	// Second prompt ("Polish") has no LLM → falls back to DefaultLLM
	label = router.llmLabelForPrompt(1)
	if label != "claude" {
		t.Errorf("expected llm label 'claude' for Polish prompt, got %q", label)
	}
}

func TestRouter_ResetClients(t *testing.T) {
	cfg := &config.Config{
		DefaultLLM: "claude",
		LLMProviders: map[string]config.LLMProviderDef{
			"claude": {Provider: "anthropic", APIKey: "sk-ant", Model: "claude-sonnet-4-5-20250929"},
		},
		Prompts: []config.PromptEntry{
			{Name: "Correct", Prompt: "Fix errors."},
		},
		MaxTokens: 256,
	}
	mock := &mockClient{
		response: &llm.Response{Text: "fixed", Provider: "mock", Model: "test"},
	}
	router := NewRouter(cfg, mock)

	// Verify the default client is set.
	if router.defaultClient == nil {
		t.Fatal("expected defaultClient to be set after NewRouter")
	}
	if len(router.clients) != 1 {
		t.Fatalf("expected 1 cached client, got %d", len(router.clients))
	}

	// Reset should clear everything.
	router.ResetClients()

	if router.defaultClient != nil {
		t.Error("expected defaultClient to be nil after ResetClients")
	}
	if len(router.clients) != 0 {
		t.Errorf("expected 0 cached clients after ResetClients, got %d", len(router.clients))
	}
}

func TestRouter_TimeoutForPrompt(t *testing.T) {
	cfg := &config.Config{
		DefaultLLM: "claude",
		LLMProviders: map[string]config.LLMProviderDef{
			"claude": {Provider: "anthropic", APIKey: "sk-ant", Model: "claude-sonnet-4-5-20250929", TimeoutMs: 15000},
			"ollama": {Provider: "ollama", Model: "mistral", TimeoutMs: 120000},
		},
		Prompts: []config.PromptEntry{
			{Name: "Correct", Prompt: "Fix errors."},
			{Name: "Local", Prompt: "Fix errors.", LLM: "ollama"},
		},
		TimeoutMs: 30000,
		MaxTokens: 256,
	}
	mock := &mockClient{}
	router := NewRouter(cfg, mock)

	// Default prompt uses claude → 15000ms
	if timeout := router.TimeoutForPrompt(0); timeout != 15000 {
		t.Errorf("expected timeout 15000 for prompt 0, got %d", timeout)
	}

	// Prompt with LLM override uses ollama → 120000ms
	if timeout := router.TimeoutForPrompt(1); timeout != 120000 {
		t.Errorf("expected timeout 120000 for prompt 1, got %d", timeout)
	}

	// Out-of-range falls back to default LLM's timeout (claude → 15000)
	if timeout := router.TimeoutForPrompt(99); timeout != 15000 {
		t.Errorf("expected default LLM timeout 15000 for invalid index, got %d", timeout)
	}
}
