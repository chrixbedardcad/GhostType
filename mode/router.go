package mode

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/chrixbedardcad/GhostType/config"
	"github.com/chrixbedardcad/GhostType/llm"
)

// Mode represents the operating mode of GhostType.
type Mode int

const (
	ModeCorrect   Mode = iota // F7 - spelling/grammar correction
	ModeTranslate             // F8 - translation
	ModeRewrite               // F9 - creative rewrite
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
	cfg                   *config.Config
	client                llm.Client
	currentTranslateIdx   int
	currentTemplateIdx    int
}

// NewRouter creates a new mode router.
func NewRouter(cfg *config.Config, client llm.Client) *Router {
	// Find the index of the default translate target
	translateIdx := 0
	for i, lang := range cfg.Languages {
		if lang == cfg.DefaultTranslateTarget {
			translateIdx = i
			break
		}
	}

	return &Router{
		cfg:                 cfg,
		client:              client,
		currentTranslateIdx: translateIdx,
		currentTemplateIdx:  0,
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

// buildTranslatePrompt builds the bilingual translation prompt.
// It substitutes {language_a} and {language_b} from the configured language pair.
// Also supports legacy {target_language} placeholder for backwards compatibility.
func (r *Router) buildTranslatePrompt() string {
	prompt := r.cfg.Prompts.Translate

	// Populate language pair placeholders for bilingual auto-detection.
	if len(r.cfg.Languages) >= 2 {
		nameA := r.cfg.LanguageNames[r.cfg.Languages[0]]
		if nameA == "" {
			nameA = r.cfg.Languages[0]
		}
		nameB := r.cfg.LanguageNames[r.cfg.Languages[1]]
		if nameB == "" {
			nameB = r.cfg.Languages[1]
		}
		prompt = strings.ReplaceAll(prompt, "{language_a}", nameA)
		prompt = strings.ReplaceAll(prompt, "{language_b}", nameB)
	}

	// Legacy support: also replace {target_language} if present.
	targetLang := r.CurrentTranslateTarget()
	targetName := r.cfg.LanguageNames[targetLang]
	if targetName == "" {
		targetName = targetLang
	}
	prompt = strings.ReplaceAll(prompt, "{target_language}", targetName)

	return prompt
}

// buildRewritePrompt returns the prompt for the current rewrite template.
func (r *Router) buildRewritePrompt() string {
	templates := r.cfg.Prompts.RewriteTemplates
	if len(templates) == 0 {
		return "Rewrite this text. Return ONLY the rewritten text."
	}
	return templates[r.currentTemplateIdx].Prompt
}

// ToggleTranslateTarget cycles to the next translation target language.
// Returns the new target language name.
func (r *Router) ToggleTranslateTarget() string {
	if len(r.cfg.Languages) == 0 {
		return ""
	}
	r.currentTranslateIdx = (r.currentTranslateIdx + 1) % len(r.cfg.Languages)
	target := r.cfg.Languages[r.currentTranslateIdx]
	name := r.cfg.LanguageNames[target]
	if name == "" {
		name = target
	}
	return name
}

// CurrentTranslateTarget returns the current translation target language code.
func (r *Router) CurrentTranslateTarget() string {
	if len(r.cfg.Languages) == 0 {
		return ""
	}
	return r.cfg.Languages[r.currentTranslateIdx]
}

// CycleTemplate cycles to the next rewrite template.
// Returns the name of the newly selected template.
func (r *Router) CycleTemplate() string {
	templates := r.cfg.Prompts.RewriteTemplates
	if len(templates) == 0 {
		return ""
	}
	r.currentTemplateIdx = (r.currentTemplateIdx + 1) % len(templates)
	return templates[r.currentTemplateIdx].Name
}

// CurrentTemplateName returns the name of the current rewrite template.
func (r *Router) CurrentTemplateName() string {
	templates := r.cfg.Prompts.RewriteTemplates
	if len(templates) == 0 {
		return ""
	}
	return templates[r.currentTemplateIdx].Name
}
