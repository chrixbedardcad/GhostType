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
		LLMProvider:            "anthropic",
		APIKey:                 "test-key",
		Model:                  "claude-sonnet-4-5-20250929",
		Languages:              []string{"en", "fr"},
		LanguageNames:          map[string]string{"en": "English", "fr": "French"},
		DefaultTranslateTarget: "en",
		Prompts: config.Prompts{
			Correct:   "Fix spelling and grammar. Return ONLY corrected text.",
			Translate: "Translate to {target_language}. Return ONLY the translation.",
			RewriteTemplates: []config.RewriteTemplate{
				{Name: "funny", Prompt: "Rewrite as funny. Return ONLY the text."},
				{Name: "formal", Prompt: "Rewrite as formal. Return ONLY the text."},
				{Name: "sarcastic", Prompt: "Rewrite with sarcasm. Return ONLY the text."},
			},
		},
		MaxTokens: 256,
	}
}

func TestRouter_ProcessCorrect(t *testing.T) {
	cfg := newTestConfig()
	mock := &mockClient{
		response: &llm.Response{Text: "Hello, how are you?", Provider: "mock", Model: "test"},
	}
	router := NewRouter(cfg, mock)

	result, err := router.Process(context.Background(), ModeCorrect, "Helo, how are yu?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "Hello, how are you?" {
		t.Errorf("expected 'Hello, how are you?', got '%s'", result)
	}

	if mock.lastReq.Prompt != cfg.Prompts.Correct {
		t.Errorf("expected correct prompt, got '%s'", mock.lastReq.Prompt)
	}
}

func TestRouter_ProcessTranslate(t *testing.T) {
	cfg := newTestConfig()
	mock := &mockClient{
		response: &llm.Response{Text: "Bonjour, comment allez-vous?", Provider: "mock", Model: "test"},
	}
	router := NewRouter(cfg, mock)

	// Default target is "en" (English)
	result, err := router.Process(context.Background(), ModeTranslate, "Bonjour")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "Bonjour, comment allez-vous?" {
		t.Errorf("expected translated text, got '%s'", result)
	}

	// Verify prompt was built with target language
	if mock.lastReq.Prompt != "Translate to English. Return ONLY the translation." {
		t.Errorf("expected prompt with English target, got '%s'", mock.lastReq.Prompt)
	}
}

func TestRouter_ProcessRewrite(t *testing.T) {
	cfg := newTestConfig()
	mock := &mockClient{
		response: &llm.Response{Text: "LOL, that's hilarious!", Provider: "mock", Model: "test"},
	}
	router := NewRouter(cfg, mock)

	result, err := router.Process(context.Background(), ModeRewrite, "That is funny")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "LOL, that's hilarious!" {
		t.Errorf("expected rewritten text, got '%s'", result)
	}

	// Default template is first one ("funny")
	if mock.lastReq.Prompt != "Rewrite as funny. Return ONLY the text." {
		t.Errorf("expected funny prompt, got '%s'", mock.lastReq.Prompt)
	}
}

func TestRouter_ProcessEmptyText(t *testing.T) {
	cfg := newTestConfig()
	mock := &mockClient{}
	router := NewRouter(cfg, mock)

	_, err := router.Process(context.Background(), ModeCorrect, "")
	if err == nil {
		t.Fatal("expected error for empty text")
	}

	_, err = router.Process(context.Background(), ModeCorrect, "   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only text")
	}
}

func TestRouter_ProcessLLMError(t *testing.T) {
	cfg := newTestConfig()
	mock := &mockClient{
		err: fmt.Errorf("API timeout"),
	}
	router := NewRouter(cfg, mock)

	_, err := router.Process(context.Background(), ModeCorrect, "test text")
	if err == nil {
		t.Fatal("expected error when LLM fails")
	}
}

func TestRouter_ToggleTranslateTarget(t *testing.T) {
	cfg := newTestConfig()
	mock := &mockClient{}
	router := NewRouter(cfg, mock)

	// Default is "en" (index 0)
	if router.CurrentTranslateTarget() != "en" {
		t.Errorf("expected default target 'en', got '%s'", router.CurrentTranslateTarget())
	}

	// Toggle to "fr"
	name := router.ToggleTranslateTarget()
	if name != "French" {
		t.Errorf("expected 'French', got '%s'", name)
	}
	if router.CurrentTranslateTarget() != "fr" {
		t.Errorf("expected target 'fr', got '%s'", router.CurrentTranslateTarget())
	}

	// Toggle back to "en"
	name = router.ToggleTranslateTarget()
	if name != "English" {
		t.Errorf("expected 'English', got '%s'", name)
	}
	if router.CurrentTranslateTarget() != "en" {
		t.Errorf("expected target 'en', got '%s'", router.CurrentTranslateTarget())
	}
}

func TestRouter_CycleTemplate(t *testing.T) {
	cfg := newTestConfig()
	mock := &mockClient{}
	router := NewRouter(cfg, mock)

	// Default is first template ("funny")
	if router.CurrentTemplateName() != "funny" {
		t.Errorf("expected default template 'funny', got '%s'", router.CurrentTemplateName())
	}

	// Cycle to "formal"
	name := router.CycleTemplate()
	if name != "formal" {
		t.Errorf("expected 'formal', got '%s'", name)
	}

	// Cycle to "sarcastic"
	name = router.CycleTemplate()
	if name != "sarcastic" {
		t.Errorf("expected 'sarcastic', got '%s'", name)
	}

	// Cycle back to "funny"
	name = router.CycleTemplate()
	if name != "funny" {
		t.Errorf("expected 'funny', got '%s'", name)
	}
}

func TestRouter_ModeString(t *testing.T) {
	tests := []struct {
		mode     Mode
		expected string
	}{
		{ModeCorrect, "correct"},
		{ModeTranslate, "translate"},
		{ModeRewrite, "rewrite"},
		{Mode(99), "unknown"},
	}

	for _, tc := range tests {
		if tc.mode.String() != tc.expected {
			t.Errorf("expected mode string '%s', got '%s'", tc.expected, tc.mode.String())
		}
	}
}
