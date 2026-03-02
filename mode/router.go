package mode

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/chrixbedardcad/GhostType/config"
	"github.com/chrixbedardcad/GhostType/llm"
)

// Mode represents the operating mode of GhostType.
type Mode int

const (
	ModeCorrect   Mode = iota // spelling/grammar correction
	ModeTranslate             // translation
	ModeRewrite               // creative rewrite
)

func (m Mode) String() string {
	switch m {
	case ModeCorrect:
		return "correct"
	case ModeTranslate:
		return "translate"
	case ModeRewrite:
		return "rewrite"
	default:
		return "unknown"
	}
}

// Router manages mode selection and dispatches text processing to the LLM.
type Router struct {
	mu                 sync.Mutex
	cfg                *config.Config
	defaultClient      llm.Client
	clients            map[string]llm.Client // lazy cache: label -> client
	currentTargetIdx   int
	currentTemplateIdx int
}

// NewRouter creates a new mode router.
func NewRouter(cfg *config.Config, client llm.Client) *Router {
	// Find the index of the default translate target (first target containing the default language).
	targetIdx := 0
	for i, t := range cfg.ParsedTargets {
		if t.LangA == cfg.DefaultTranslateTarget || t.LangB == cfg.DefaultTranslateTarget {
			targetIdx = i
			break
		}
	}

	clients := make(map[string]llm.Client)
	if cfg.DefaultLLM != "" {
		clients[cfg.DefaultLLM] = client
	}

	return &Router{
		cfg:                cfg,
		defaultClient:      client,
		clients:            clients,
		currentTargetIdx:   targetIdx,
		currentTemplateIdx: 0,
	}
}

// Process sends text through the LLM using the specified mode.
func (r *Router) Process(ctx context.Context, mode Mode, text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("nothing to process: empty text")
	}

	var prompt string
	switch mode {
	case ModeCorrect:
		prompt = r.cfg.Prompts.Correct
	case ModeTranslate:
		prompt = r.buildTranslatePrompt()
	case ModeRewrite:
		prompt = r.buildRewritePrompt()
	default:
		return "", fmt.Errorf("unknown mode: %d", mode)
	}

	label := r.llmLabelForMode(mode)
	client, err := r.resolveClient(label)
	if err != nil {
		return "", err
	}

	truncatedPrompt := prompt
	if len(truncatedPrompt) > 80 {
		truncatedPrompt = truncatedPrompt[:80] + "..."
	}
	slog.Debug("processing text", "mode", mode.String(), "llm", label, "prompt", truncatedPrompt, "input_len", len(text))

	resp, err := client.Send(ctx, llm.Request{
		Prompt: prompt,
		Text:   text,
	})
	if err != nil {
		slog.Debug("LLM request failed", "mode", mode.String(), "llm", label, "input_len", len(text), "error", err)
		return "", fmt.Errorf("LLM request failed: %w", err)
	}

	slog.Debug("LLM response received", "provider", resp.Provider, "model", resp.Model, "llm", label, "response_len", len(resp.Text))

	return strings.TrimSpace(resp.Text), nil
}

// resolveClient returns the LLM client for the given label.
// If label is empty, the default client is returned.
// Clients are lazily created and cached.
func (r *Router) resolveClient(label string) (llm.Client, error) {
	if label == "" {
		return r.defaultClient, nil
	}

	r.mu.Lock()
	if c, ok := r.clients[label]; ok {
		r.mu.Unlock()
		return c, nil
	}
	r.mu.Unlock()

	def, ok := r.cfg.LLMProviders[label]
	if !ok {
		slog.Warn("LLM label not found in llm_providers, falling back to default", "label", label)
		return r.defaultClient, nil
	}

	c, err := llm.NewClientFromDef(def)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client for %q: %w", label, err)
	}

	r.mu.Lock()
	// Double-check: another goroutine may have created it.
	if existing, ok := r.clients[label]; ok {
		r.mu.Unlock()
		return existing, nil
	}
	r.clients[label] = c
	r.mu.Unlock()

	return c, nil
}

// TimeoutForMode returns the timeout (in ms) for the provider that will handle
// the given mode. Uses the per-provider timeout_ms if set, otherwise the global.
func (r *Router) TimeoutForMode(m Mode) int {
	label := r.llmLabelForMode(m)
	if label != "" {
		if def, ok := r.cfg.LLMProviders[label]; ok && def.TimeoutMs > 0 {
			return def.TimeoutMs
		}
	}
	return r.cfg.TimeoutMs
}

// llmLabelForMode returns the LLM provider label for the given mode.
func (r *Router) llmLabelForMode(m Mode) string {
	switch m {
	case ModeCorrect:
		if r.cfg.CorrectLLM != "" {
			return r.cfg.CorrectLLM
		}
	case ModeTranslate:
		if r.cfg.TranslateLLM != "" {
			return r.cfg.TranslateLLM
		}
	case ModeRewrite:
		templates := r.cfg.Prompts.RewriteTemplates
		if len(templates) > 0 {
			r.mu.Lock()
			idx := r.currentTemplateIdx
			r.mu.Unlock()
			if templates[idx].LLM != "" {
				return templates[idx].LLM
			}
		}
	}
	return r.cfg.DefaultLLM
}

// buildTranslatePrompt builds the translation prompt based on the current target.
// For pair targets: substitutes {language_a} and {language_b} in Prompts.Translate.
// For single targets: substitutes {target_language} in Prompts.TranslateSingle.
// Caller must NOT hold r.mu — this method reads the index under lock internally.
func (r *Router) buildTranslatePrompt() string {
	r.mu.Lock()
	idx := r.currentTargetIdx
	r.mu.Unlock()

	if len(r.cfg.ParsedTargets) == 0 {
		return r.cfg.Prompts.Translate
	}

	target := r.cfg.ParsedTargets[idx]

	if target.IsPair() {
		prompt := r.cfg.Prompts.Translate
		nameA := r.cfg.LanguageNames[target.LangA]
		if nameA == "" {
			nameA = target.LangA
		}
		nameB := r.cfg.LanguageNames[target.LangB]
		if nameB == "" {
			nameB = target.LangB
		}
		prompt = strings.ReplaceAll(prompt, "{language_a}", nameA)
		prompt = strings.ReplaceAll(prompt, "{language_b}", nameB)
		return prompt
	}

	// Single target mode.
	prompt := r.cfg.Prompts.TranslateSingle
	if prompt == "" {
		prompt = "Translate the following text to {target_language}. Preserve the tone and intent. Return ONLY the translation with no explanation."
	}
	nameA := r.cfg.LanguageNames[target.LangA]
	if nameA == "" {
		nameA = target.LangA
	}
	prompt = strings.ReplaceAll(prompt, "{target_language}", nameA)
	return prompt
}

// buildRewritePrompt returns the prompt for the current rewrite template.
// Caller must NOT hold r.mu — this method reads the index under lock internally.
func (r *Router) buildRewritePrompt() string {
	templates := r.cfg.Prompts.RewriteTemplates
	if len(templates) == 0 {
		return "Rewrite this text. Return ONLY the rewritten text."
	}
	r.mu.Lock()
	idx := r.currentTemplateIdx
	r.mu.Unlock()
	return templates[idx].Prompt
}

// ToggleTranslateTarget cycles to the next translation target.
// Returns the display label for the new target.
func (r *Router) ToggleTranslateTarget() string {
	labels := r.cfg.TranslateTargetLabels()
	if len(labels) == 0 {
		return ""
	}
	r.mu.Lock()
	r.currentTargetIdx = (r.currentTargetIdx + 1) % len(r.cfg.ParsedTargets)
	idx := r.currentTargetIdx
	r.mu.Unlock()
	return labels[idx]
}

// SetTranslateTarget sets the translation target to the given index.
// Returns the display label at that index.
func (r *Router) SetTranslateTarget(idx int) string {
	labels := r.cfg.TranslateTargetLabels()
	if len(labels) == 0 || idx < 0 || idx >= len(labels) {
		return ""
	}
	r.mu.Lock()
	r.currentTargetIdx = idx
	r.mu.Unlock()
	return labels[idx]
}

// CurrentTranslateIdx returns the current translation target index.
func (r *Router) CurrentTranslateIdx() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentTargetIdx
}

// CycleTemplate cycles to the next rewrite template.
// Returns the name of the newly selected template.
func (r *Router) CycleTemplate() string {
	templates := r.cfg.Prompts.RewriteTemplates
	if len(templates) == 0 {
		return ""
	}
	r.mu.Lock()
	r.currentTemplateIdx = (r.currentTemplateIdx + 1) % len(templates)
	idx := r.currentTemplateIdx
	r.mu.Unlock()
	return templates[idx].Name
}

// CurrentTemplateName returns the name of the current rewrite template.
func (r *Router) CurrentTemplateName() string {
	templates := r.cfg.Prompts.RewriteTemplates
	if len(templates) == 0 {
		return ""
	}
	r.mu.Lock()
	idx := r.currentTemplateIdx
	r.mu.Unlock()
	return templates[idx].Name
}

// SetTemplate sets the rewrite template to the given index.
// Returns the template name at that index.
func (r *Router) SetTemplate(idx int) string {
	templates := r.cfg.Prompts.RewriteTemplates
	if len(templates) == 0 || idx < 0 || idx >= len(templates) {
		return ""
	}
	r.mu.Lock()
	r.currentTemplateIdx = idx
	r.mu.Unlock()
	return templates[idx].Name
}

// CurrentTemplateIdx returns the current rewrite template index.
func (r *Router) CurrentTemplateIdx() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentTemplateIdx
}
