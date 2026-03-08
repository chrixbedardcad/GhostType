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
	if cfg.DefaultLLM != "" {
		clients[cfg.DefaultLLM] = client
	}

	return &Router{
		cfg:           cfg,
		defaultClient: client,
		clients:       clients,
	}
}

// Process sends text through the LLM using the prompt at the given index.
func (r *Router) Process(ctx context.Context, promptIdx int, text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("nothing to process: empty text")
	}

	if promptIdx < 0 || promptIdx >= len(r.cfg.Prompts) {
		return "", fmt.Errorf("invalid prompt index: %d (have %d prompts)", promptIdx, len(r.cfg.Prompts))
	}

	entry := r.cfg.Prompts[promptIdx]
	prompt := entry.Prompt

	label := r.llmLabelForPrompt(promptIdx)
	client, err := r.resolveClient(label)
	if err != nil {
		return "", err
	}

	truncatedPrompt := prompt
	if len(truncatedPrompt) > 80 {
		truncatedPrompt = truncatedPrompt[:80] + "..."
	}
	slog.Debug("processing text", "prompt", entry.Name, "llm", label, "prompt_text", truncatedPrompt, "input_len", len(text))

	resp, err := client.Send(ctx, llm.Request{
		Prompt: prompt,
		Text:   text,
	})
	if err != nil {
		slog.Debug("LLM request failed", "prompt", entry.Name, "llm", label, "input_len", len(text), "error", err)
		return "", fmt.Errorf("LLM request failed: %w", err)
	}

	slog.Debug("LLM response received", "provider", resp.Provider, "model", resp.Model, "llm", label, "response_len", len(resp.Text))

	return strings.TrimSpace(resp.Text), nil
}

// ResetClients clears all cached LLM clients so that the next request
// lazily creates fresh ones from the (possibly updated) config.
func (r *Router) ResetClients() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients = make(map[string]llm.Client)
	r.defaultClient = nil
	slog.Debug("LLM client cache reset")
}

// resolveClient returns the LLM client for the given label.
// If label is empty, the default client is returned.
// Clients are lazily created and cached.
func (r *Router) resolveClient(label string) (llm.Client, error) {
	if label == "" {
		if r.defaultClient != nil {
			return r.defaultClient, nil
		}
		label = r.cfg.DefaultLLM
		if label == "" {
			return nil, fmt.Errorf("no default LLM configured")
		}
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
	if existing, ok := r.clients[label]; ok {
		r.mu.Unlock()
		return existing, nil
	}
	r.clients[label] = c
	if label == r.cfg.DefaultLLM {
		r.defaultClient = c
	}
	r.mu.Unlock()

	return c, nil
}

// TimeoutForPrompt returns the timeout (in ms) for the provider that will handle
// the given prompt. Uses the per-provider timeout_ms if set, otherwise the global.
func (r *Router) TimeoutForPrompt(promptIdx int) int {
	label := r.llmLabelForPrompt(promptIdx)
	if label != "" {
		if def, ok := r.cfg.LLMProviders[label]; ok && def.TimeoutMs > 0 {
			return def.TimeoutMs
		}
	}
	return r.cfg.TimeoutMs
}

// llmLabelForPrompt returns the LLM provider label for the given prompt index.
func (r *Router) llmLabelForPrompt(promptIdx int) string {
	if promptIdx >= 0 && promptIdx < len(r.cfg.Prompts) {
		if r.cfg.Prompts[promptIdx].LLM != "" {
			return r.cfg.Prompts[promptIdx].LLM
		}
	}
	return r.cfg.DefaultLLM
}

// CyclePrompt cycles to the next prompt, returning the new index and name.
func (r *Router) CyclePrompt() (int, string) {
	if len(r.cfg.Prompts) == 0 {
		return 0, ""
	}
	r.mu.Lock()
	r.cfg.ActivePrompt = (r.cfg.ActivePrompt + 1) % len(r.cfg.Prompts)
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
