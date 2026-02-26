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
	mu                    sync.Mutex
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
// Caller must NOT hold r.mu — this method reads the index under lock internally.
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

	// Copy index under lock to avoid holding mu during string work.
	r.mu.Lock()
	idx := r.currentTranslateIdx
	r.mu.Unlock()

	targetLang := ""
	if len(r.cfg.Languages) > 0 {
		targetLang = r.cfg.Languages[idx]
	}
	targetName := r.cfg.LanguageNames[targetLang]
	if targetName == "" {
		targetName = targetLang
	}
	prompt = strings.ReplaceAll(prompt, "{target_language}", targetName)

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

// ToggleTranslateTarget cycles to the next translation target language.
// Returns the new target language name.
func (r *Router) ToggleTranslateTarget() string {
	if len(r.cfg.Languages) == 0 {
		return ""
	}
	r.mu.Lock()
	r.currentTranslateIdx = (r.currentTranslateIdx + 1) % len(r.cfg.Languages)
	idx := r.currentTranslateIdx
	r.mu.Unlock()
	target := r.cfg.Languages[idx]
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
	r.mu.Lock()
	idx := r.currentTranslateIdx
	r.mu.Unlock()
	return r.cfg.Languages[idx]
}

// SetTranslateTarget sets the translation target to the given index.
// Returns the language name at that index.
func (r *Router) SetTranslateTarget(idx int) string {
	if len(r.cfg.Languages) == 0 || idx < 0 || idx >= len(r.cfg.Languages) {
		return ""
	}
	r.mu.Lock()
	r.currentTranslateIdx = idx
	r.mu.Unlock()
	target := r.cfg.Languages[idx]
	name := r.cfg.LanguageNames[target]
	if name == "" {
		name = target
	}
	return name
}

// CurrentTranslateIdx returns the current translation target index.
func (r *Router) CurrentTranslateIdx() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentTranslateIdx
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
