package mode

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/chrixbedardcad/GhostSpell/config"
	"github.com/chrixbedardcad/GhostSpell/llm"
)

// Router manages prompt selection and dispatches text processing to the LLM.
type Router struct {
	mu            sync.Mutex
	cfg           *config.Config
	defaultClient llm.Client
	clients       map[string]llm.Client // lazy cache: label -> client
}

// NewRouter creates a new mode router.
func NewRouter(cfg *config.Config, client llm.Client) *Router {
	clients := make(map[string]llm.Client)
	if cfg.DefaultModel != "" {
		clients[cfg.DefaultModel] = client
	}

	return &Router{
		cfg:           cfg,
		defaultClient: client,
		clients:       clients,
	}
}

// Process sends text through the LLM using the prompt at the given index.
// Returns the full LLM response (with Provider/Model metadata) for stats tracking.
func (r *Router) Process(ctx context.Context, promptIdx int, text string) (*llm.Response, error) {
	return r.ProcessWithImages(ctx, promptIdx, text, nil)
}

// ProcessWithImages sends text and optional images through the LLM.
// When images is non-nil, the request includes image data for vision models.
func (r *Router) ProcessWithImages(ctx context.Context, promptIdx int, text string, images [][]byte) (*llm.Response, error) {
	if strings.TrimSpace(text) == "" && len(images) == 0 {
		return nil, fmt.Errorf("nothing to process: empty text and no images")
	}

	if promptIdx < 0 || promptIdx >= len(r.cfg.Prompts) {
		return nil, fmt.Errorf("invalid prompt index: %d (have %d prompts)", promptIdx, len(r.cfg.Prompts))
	}

	entry := r.cfg.Prompts[promptIdx]
	prompt := entry.Prompt

	// Resolve {{language}} template variable from global config.
	if r.cfg.Language != "" {
		prompt = strings.ReplaceAll(prompt, "{{language}}", r.cfg.Language)
	} else {
		// No language set — fall back to "its original language".
		prompt = strings.ReplaceAll(prompt, "in {{language}}", "in its original language")
		prompt = strings.ReplaceAll(prompt, "{{language}}", "the original language")
	}

	label := r.llmLabelForPrompt(promptIdx)
	client, err := r.resolveClient(label)
	if err != nil {
		return nil, err
	}

	truncatedPrompt := prompt
	if len(truncatedPrompt) > 80 {
		truncatedPrompt = truncatedPrompt[:80] + "..."
	}
	slog.Debug("processing", "prompt", entry.Name, "llm", label, "prompt_text", truncatedPrompt, "input_len", len(text), "images", len(images))

	resp, err := client.Send(ctx, llm.Request{
		Prompt: prompt,
		Text:   text,
		Images: images,
	})

	if err != nil {
		slog.Debug("LLM request failed", "prompt", entry.Name, "llm", label, "input_len", len(text), "error", err)
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	slog.Debug("LLM response received", "provider", resp.Provider, "model", resp.Model, "llm", label, "response_len", len(resp.Text))

	resp.Text = strings.TrimSpace(resp.Text)
	return resp, nil
}


// ResetClients clears all cached LLM clients so that the next request
// lazily creates fresh ones from the (possibly updated) config.
// Closes idle HTTP connections on old clients to prevent resource leaks.
func (r *Router) ResetClients() {
	r.mu.Lock()
	old := r.clients
	oldDefault := r.defaultClient
	r.clients = make(map[string]llm.Client)
	r.defaultClient = nil
	r.mu.Unlock()

	// Close old clients outside the lock to avoid blocking.
	for _, c := range old {
		c.Close()
	}
	if oldDefault != nil {
		oldDefault.Close()
	}
	slog.Debug("LLM client cache reset", "closed", len(old))
}

// resolveClient returns the LLM client for the given label.
// If label is empty, the default client is returned.
// Clients are lazily created and cached.
func (r *Router) resolveClient(label string) (llm.Client, error) {
	if label == "" {
		if r.defaultClient != nil {
			return r.defaultClient, nil
		}
		label = r.cfg.DefaultModel
		if label == "" {
			return nil, fmt.Errorf("no default model configured")
		}
	}

	r.mu.Lock()
	if c, ok := r.clients[label]; ok {
		r.mu.Unlock()
		return c, nil
	}

	// Look up model entry
	model, ok := r.cfg.Models[label]
	if !ok {
		fallback := r.defaultClient
		r.mu.Unlock()
		if fallback == nil {
			return nil, fmt.Errorf("model %q not found and no default client configured", label)
		}
		slog.Warn("Model label not found, falling back to default", "label", label)
		return fallback, nil
	}

	// Look up provider credentials
	prov, provOK := r.cfg.Providers[model.Provider]
	r.mu.Unlock()

	if !provOK {
		return nil, fmt.Errorf("provider %q not configured for model %q", model.Provider, label)
	}

	// Merge model + provider into LLMProviderDef for client creation
	def := config.LLMProviderDef{
		Provider:     model.Provider,
		APIKey:       prov.APIKey,
		Model:        model.Model,
		APIEndpoint:  prov.APIEndpoint,
		RefreshToken: prov.RefreshToken,
		KeepAlive:    prov.KeepAlive,
	}
	// Per-model timeout overrides provider timeout
	if model.TimeoutMs > 0 {
		def.TimeoutMs = model.TimeoutMs
	} else {
		def.TimeoutMs = prov.TimeoutMs
	}
	if model.MaxTokens > 0 {
		def.MaxTokens = model.MaxTokens
	}

	c, err := llm.NewClientFromDef(def)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client for %q: %w", label, err)
	}

	r.mu.Lock()
	if existing, ok := r.clients[label]; ok {
		r.mu.Unlock()
		c.Close()
		return existing, nil
	}
	r.clients[label] = c
	if label == r.cfg.DefaultModel {
		r.defaultClient = c
	}
	r.mu.Unlock()

	return c, nil
}

// TimeoutForPrompt returns the timeout (in ms) for the provider that will handle
// the given prompt. Priority: per-prompt override > per-model > per-provider > global.
func (r *Router) TimeoutForPrompt(promptIdx int) int {
	// Check per-prompt timeout override first.
	if promptIdx >= 0 && promptIdx < len(r.cfg.Prompts) {
		if r.cfg.Prompts[promptIdx].TimeoutMs > 0 {
			return r.cfg.Prompts[promptIdx].TimeoutMs
		}
	}

	label := r.llmLabelForPrompt(promptIdx)
	if label != "" {
		if model, ok := r.cfg.Models[label]; ok {
			if model.TimeoutMs > 0 {
				return model.TimeoutMs
			}
			if prov, ok := r.cfg.Providers[model.Provider]; ok && prov.TimeoutMs > 0 {
				return prov.TimeoutMs
			}
		}
	}
	return r.cfg.TimeoutMs
}

// llmLabelForPrompt returns the LLM provider label for the given prompt index.
func (r *Router) llmLabelForPrompt(promptIdx int) string {
	if promptIdx >= 0 && promptIdx < len(r.cfg.Prompts) {
		if r.cfg.Prompts[promptIdx].LLM != "" {
			slog.Debug("llmLabelForPrompt: using per-prompt override", "prompt", r.cfg.Prompts[promptIdx].Name, "llm", r.cfg.Prompts[promptIdx].LLM, "default", r.cfg.DefaultModel)
			return r.cfg.Prompts[promptIdx].LLM
		}
	}
	slog.Debug("llmLabelForPrompt: using default model", "default", r.cfg.DefaultModel)
	return r.cfg.DefaultModel
}

// CyclePrompt cycles to the next enabled prompt, returning the new index and name.
// Disabled prompts are skipped. If all prompts are disabled, stays on the current one.
func (r *Router) CyclePrompt() (int, string) {
	if len(r.cfg.Prompts) == 0 {
		return 0, ""
	}
	r.mu.Lock()
	next, found := config.NextEnabledPrompt(r.cfg.Prompts, r.cfg.ActivePrompt)
	if found {
		r.cfg.ActivePrompt = next
	}
	idx := r.cfg.ActivePrompt
	r.mu.Unlock()
	return idx, r.cfg.Prompts[idx].Name
}

// SetPrompt sets the active prompt to the given index.
// Returns the prompt name at that index.
func (r *Router) SetPrompt(idx int) string {
	if idx < 0 || idx >= len(r.cfg.Prompts) {
		return ""
	}
	r.mu.Lock()
	r.cfg.ActivePrompt = idx
	r.mu.Unlock()
	return r.cfg.Prompts[idx].Name
}

// CurrentPromptIdx returns the current active prompt index.
func (r *Router) CurrentPromptIdx() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cfg.ActivePrompt
}

// CurrentPromptName returns the name of the current active prompt.
func (r *Router) CurrentPromptName() string {
	r.mu.Lock()
	idx := r.cfg.ActivePrompt
	r.mu.Unlock()
	if idx < 0 || idx >= len(r.cfg.Prompts) {
		return ""
	}
	return r.cfg.Prompts[idx].Name
}
