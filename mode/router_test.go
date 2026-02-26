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
		TranslateTargets:       []string{"en|fr"},
		ParsedTargets:          []config.TranslateTarget{{LangA: "en", LangB: "fr"}},
		DefaultTranslateTarget: "en",
		Prompts: config.Prompts{
			Correct:         "Fix spelling and grammar. Return ONLY corrected text.",
			Translate:       "Detect language and translate between {language_a} and {language_b}. Return ONLY the translation.",
			TranslateSingle: "Translate to {target_language}. Return ONLY the translation.",
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

func TestRouter_ProcessTranslatePair(t *testing.T) {
	cfg := newTestConfig()
	mock := &mockClient{
		response: &llm.Response{Text: "Bonjour, comment allez-vous?", Provider: "mock", Model: "test"},
	}
	router := NewRouter(cfg, mock)

	result, err := router.Process(context.Background(), ModeTranslate, "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "Bonjour, comment allez-vous?" {
		t.Errorf("expected translated text, got '%s'", result)
	}

	// Pair target uses Translate prompt with {language_a}/{language_b}
	expectedPrompt := "Detect language and translate between English and French. Return ONLY the translation."
	if mock.lastReq.Prompt != expectedPrompt {
		t.Errorf("expected prompt %q, got %q", expectedPrompt, mock.lastReq.Prompt)
	}
}

func TestRouter_ProcessTranslateSingle(t *testing.T) {
	cfg := &config.Config{
		Languages:              []string{"en", "fr", "es"},
		LanguageNames:          map[string]string{"en": "English", "fr": "French", "es": "Spanish"},
		TranslateTargets:       []string{"en|fr", "es"},
		ParsedTargets:          []config.TranslateTarget{{LangA: "en", LangB: "fr"}, {LangA: "es"}},
		DefaultTranslateTarget: "es",
		Prompts: config.Prompts{
			Correct:         "Fix errors.",
			Translate:       "Detect language and translate between {language_a} and {language_b}.",
			TranslateSingle: "Translate to {target_language}.",
		},
		MaxTokens: 256,
	}
	mock := &mockClient{
		response: &llm.Response{Text: "Hola", Provider: "mock", Model: "test"},
	}
	router := NewRouter(cfg, mock)

	// Default target is "es" which is at index 1 (single target)
	if router.CurrentTranslateIdx() != 1 {
		t.Fatalf("expected default target index 1, got %d", router.CurrentTranslateIdx())
	}

	_, err := router.Process(context.Background(), ModeTranslate, "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Single target uses TranslateSingle prompt with {target_language}
	expectedPrompt := "Translate to Spanish."
	if mock.lastReq.Prompt != expectedPrompt {
		t.Errorf("expected prompt %q, got %q", expectedPrompt, mock.lastReq.Prompt)
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
	cfg := &config.Config{
		Languages:              []string{"en", "fr", "es"},
		LanguageNames:          map[string]string{"en": "English", "fr": "French", "es": "Spanish"},
		TranslateTargets:       []string{"en|fr", "es"},
		ParsedTargets:          []config.TranslateTarget{{LangA: "en", LangB: "fr"}, {LangA: "es"}},
		DefaultTranslateTarget: "en",
		Prompts: config.Prompts{
			Correct:         "Fix errors.",
			Translate:       "Translate between {language_a} and {language_b}.",
			TranslateSingle: "Translate to {target_language}.",
		},
		MaxTokens: 256,
	}
	mock := &mockClient{}
	router := NewRouter(cfg, mock)

	// Default is index 0 (en|fr pair)
	if router.CurrentTranslateIdx() != 0 {
		t.Errorf("expected default target index 0, got %d", router.CurrentTranslateIdx())
	}

	// Toggle to index 1 (es single)
	label := router.ToggleTranslateTarget()
	if label != "Spanish" {
		t.Errorf("expected 'Spanish', got '%s'", label)
	}
	if router.CurrentTranslateIdx() != 1 {
		t.Errorf("expected index 1, got %d", router.CurrentTranslateIdx())
	}

	// Toggle back to index 0 (en|fr pair)
	label = router.ToggleTranslateTarget()
	if label != "English ↔ French" {
		t.Errorf("expected 'English ↔ French', got '%s'", label)
	}
	if router.CurrentTranslateIdx() != 0 {
		t.Errorf("expected index 0, got %d", router.CurrentTranslateIdx())
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

func TestRouter_PairTranslatePrompt(t *testing.T) {
	cfg := &config.Config{
		Languages:              []string{"en", "fr"},
		LanguageNames:          map[string]string{"en": "English", "fr": "French"},
		TranslateTargets:       []string{"en|fr"},
		ParsedTargets:          []config.TranslateTarget{{LangA: "en", LangB: "fr"}},
		DefaultTranslateTarget: "en",
		Prompts: config.Prompts{
			Correct:   "Fix errors.",
			Translate: "Languages are {language_a} and {language_b}. Detect and translate.",
		},
		MaxTokens: 256,
	}
	mock := &mockClient{
		response: &llm.Response{Text: "translated", Provider: "mock", Model: "test"},
	}
	router := NewRouter(cfg, mock)

	_, err := router.Process(context.Background(), ModeTranslate, "Bonjour")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Languages are English and French. Detect and translate."
	if mock.lastReq.Prompt != expected {
		t.Errorf("expected prompt %q, got %q", expected, mock.lastReq.Prompt)
	}
}

func TestRouter_SetTranslateTarget(t *testing.T) {
	cfg := &config.Config{
		Languages:              []string{"en", "fr", "es"},
		LanguageNames:          map[string]string{"en": "English", "fr": "French", "es": "Spanish"},
		TranslateTargets:       []string{"en|fr", "es"},
		ParsedTargets:          []config.TranslateTarget{{LangA: "en", LangB: "fr"}, {LangA: "es"}},
		DefaultTranslateTarget: "en",
		Prompts: config.Prompts{
			Correct:         "Fix errors.",
			Translate:       "Translate between {language_a} and {language_b}.",
			TranslateSingle: "Translate to {target_language}.",
		},
		MaxTokens: 256,
	}
	mock := &mockClient{}
	router := NewRouter(cfg, mock)

	// Default index is 0 (en|fr pair)
	if router.CurrentTranslateIdx() != 0 {
		t.Errorf("expected default index 0, got %d", router.CurrentTranslateIdx())
	}

	// Set to index 1 (es single)
	label := router.SetTranslateTarget(1)
	if label != "Spanish" {
		t.Errorf("expected 'Spanish', got '%s'", label)
	}
	if router.CurrentTranslateIdx() != 1 {
		t.Errorf("expected index 1, got %d", router.CurrentTranslateIdx())
	}

	// Set back to index 0 (pair)
	label = router.SetTranslateTarget(0)
	if label != "English ↔ French" {
		t.Errorf("expected 'English ↔ French', got '%s'", label)
	}

	// Out-of-bounds returns empty
	label = router.SetTranslateTarget(-1)
	if label != "" {
		t.Errorf("expected empty for negative index, got '%s'", label)
	}
	label = router.SetTranslateTarget(99)
	if label != "" {
		t.Errorf("expected empty for out-of-range index, got '%s'", label)
	}
}

func TestRouter_BackwardCompatEmptyTargets(t *testing.T) {
	// When TranslateTargets is empty but Languages has values,
	// applyDefaults should derive targets. Simulating that here.
	cfg := &config.Config{
		Languages:              []string{"en", "fr"},
		LanguageNames:          map[string]string{"en": "English", "fr": "French"},
		TranslateTargets:       []string{"en|fr"},
		ParsedTargets:          []config.TranslateTarget{{LangA: "en", LangB: "fr"}},
		DefaultTranslateTarget: "en",
		Prompts: config.Prompts{
			Correct:   "Fix errors.",
			Translate: "Translate between {language_a} and {language_b}.",
		},
		MaxTokens: 256,
	}
	mock := &mockClient{
		response: &llm.Response{Text: "translated", Provider: "mock", Model: "test"},
	}
	router := NewRouter(cfg, mock)

	// Should work as a pair derived from languages
	_, err := router.Process(context.Background(), ModeTranslate, "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Translate between English and French."
	if mock.lastReq.Prompt != expected {
		t.Errorf("expected prompt %q, got %q", expected, mock.lastReq.Prompt)
	}
}

func TestRouter_SetTemplate(t *testing.T) {
	cfg := newTestConfig()
	mock := &mockClient{}
	router := NewRouter(cfg, mock)

	// Default index is 0 ("funny")
	if router.CurrentTemplateIdx() != 0 {
		t.Errorf("expected default index 0, got %d", router.CurrentTemplateIdx())
	}

	// Set to index 1 ("formal")
	name := router.SetTemplate(1)
	if name != "formal" {
		t.Errorf("expected 'formal', got '%s'", name)
	}
	if router.CurrentTemplateIdx() != 1 {
		t.Errorf("expected index 1, got %d", router.CurrentTemplateIdx())
	}
	if router.CurrentTemplateName() != "formal" {
		t.Errorf("expected 'formal', got '%s'", router.CurrentTemplateName())
	}

	// Set to index 2 ("sarcastic")
	name = router.SetTemplate(2)
	if name != "sarcastic" {
		t.Errorf("expected 'sarcastic', got '%s'", name)
	}

	// Out-of-bounds returns empty
	name = router.SetTemplate(-1)
	if name != "" {
		t.Errorf("expected empty for negative index, got '%s'", name)
	}
	name = router.SetTemplate(99)
	if name != "" {
		t.Errorf("expected empty for out-of-range index, got '%s'", name)
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
