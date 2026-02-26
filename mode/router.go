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
	client             llm.Client
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

	return &Router{
		cfg:                cfg,
		client:             client,
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

	truncatedPrompt := prompt
	if len(truncatedPrompt) > 80 {
		truncatedPrompt = truncatedPrompt[:80] + "..."
	}
	slog.Debug("processing text", "mode", mode.String(), "prompt", truncatedPrompt, "input_len", len(text))

	resp, err := r.client.Send(ctx, llm.Request{
		Prompt:    prompt,
		Text:      text,
		MaxTokens: r.cfg.MaxTokens,
	})
	if err != nil {
		slog.Debug("LLM request failed", "mode", mode.String(), "input_len", len(text), "error", err)
		return "", fmt.Errorf("LLM request failed: %w", err)
	}

	slog.Debug("LLM response received", "provider", resp.Provider, "model", resp.Model, "response_len", len(resp.Text))

	return strings.TrimSpace(resp.Text), nil
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
