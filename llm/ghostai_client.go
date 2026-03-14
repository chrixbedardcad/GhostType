package llm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/chrixbedardcad/GhostSpell/llm/ghostai"
)

// GhostAIClient implements the Client interface using the embedded Ghost-AI
// engine (llama.cpp via CGo). It replaces the subprocess-based LocalClient
// when built with -tags ghostai.
type GhostAIClient struct {
	mu        sync.Mutex
	engine    *ghostai.Engine
	modelPath string
	modelName string
	maxTokens int
	keepAlive bool
	idleTimer *time.Timer
}

// newGhostAIFromDef creates a GhostAIClient from a provider definition.
// Recovers from C-level panics (e.g. missing DLLs) so the app doesn't crash.
func newGhostAIFromDef(def LLMProviderDefCompat) (client *GhostAIClient, err error) {
	// Catch C-level panics (missing DLL, segfault in llama init, etc.)
	// so GhostSpell can still start with Ghost-AI disabled.
	defer func() {
		if r := recover(); r != nil {
			slog.Error("[ghost-ai] engine init panicked — disabling Ghost-AI", "panic", r)
			err = fmt.Errorf("ghost-ai init panic: %v", r)
			client = nil
		}
	}()

	maxTokens := def.MaxTokens
	if maxTokens == 0 {
		maxTokens = 256
	}

	modelPath, err := resolveLocalModel(def.Model)
	if err != nil {
		return nil, fmt.Errorf("local model %q: %w", def.Model, err)
	}

	config := ghostai.DefaultConfig()
	config.MaxTokens = maxTokens
	// Give thinking models (Qwen3/3.5, DeepSeek) more context for <think>
	// blocks. The default 2048 is too tight when thinking overhead is 200-400+
	// tokens on top of the actual output.
	if isThinkingModel(def.Model) {
		config.ContextSize = 4096
	}

	engine := ghostai.New(config)

	slog.Info("[ghost-ai] loading model", "path", modelPath, "model", def.Model)
	if err := engine.Load(modelPath); err != nil {
		engine.Close()
		return nil, fmt.Errorf("ghost-ai load: %w", err)
	}

	c := &GhostAIClient{
		engine:    engine,
		modelPath: modelPath,
		modelName: def.Model,
		maxTokens: maxTokens,
		keepAlive: def.KeepAlive,
	}

	// Start idle timer (unless keep-alive).
	if !def.KeepAlive {
		c.idleTimer = time.AfterFunc(localIdleTimeout, func() {
			slog.Info("[ghost-ai] idle timeout — unloading model")
			c.mu.Lock()
			c.engine.Unload()
			c.mu.Unlock()
		})
	}

	return c, nil
}

func (c *GhostAIClient) Provider() string { return "local" }

func (c *GhostAIClient) Send(ctx context.Context, req Request) (resp *Response, err error) {
	// Catch C-level panics during inference.
	defer func() {
		if r := recover(); r != nil {
			slog.Error("[ghost-ai] inference panicked", "panic", r)
			err = fmt.Errorf("ghost-ai inference panic: %v", r)
			resp = nil
		}
	}()
	c.mu.Lock()

	// Reload model if it was unloaded by idle timer.
	if !c.engine.IsLoaded() {
		slog.Info("[ghost-ai] reloading model after idle unload", "path", c.modelPath)
		if err := c.engine.Load(c.modelPath); err != nil {
			c.mu.Unlock()
			return nil, fmt.Errorf("ghost-ai reload: %w", err)
		}
	}

	// Reset idle timer.
	if c.idleTimer != nil {
		c.idleTimer.Reset(localIdleTimeout)
	}

	c.mu.Unlock()

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = c.maxTokens
	}
	thinking := isThinkingModel(c.modelName)
	if thinking {
		// Thinking models need room for <think> blocks + actual output.
		// Don't cap — let them use the full context window. The C bridge
		// clamps max_tokens to context_size - prompt_tokens automatically.
		maxTokens = c.engine.Config().ContextSize
	} else {
		// Non-thinking models: cap to ~2x input length to avoid wasted generation.
		inputWords := len(strings.Fields(req.Text))
		dynamicMax := int(float64(inputWords)*2) + 64
		if dynamicMax < maxTokens {
			maxTokens = dynamicMax
		}
	}
	if maxTokens < 64 {
		maxTokens = 64
	}

	// Format using the model's chat template (ChatML for Qwen, etc.).
	// System = instruction prompt, User = the text to process.
	// /no_think goes in the system message — putting it in the user message
	// contaminates the input text (small models echo it back as part of the
	// corrected text instead of recognizing it as a directive).
	systemMsg := req.Prompt
	if thinking {
		systemMsg = "/no_think\n" + req.Prompt
	}
	prompt, err := c.engine.ApplyChat(systemMsg, req.Text)
	if err != nil {
		slog.Warn("[ghost-ai] chat template failed, using raw format", "error", err)
		// Fallback to raw format if template fails.
		prompt = systemMsg + "\n\nUser: " + req.Text
	}

	text, stats, err := c.engine.Complete(ctx, prompt, maxTokens)
	if err != nil {
		slog.Error("[ghost-ai] completion failed", "error", err)
		return nil, fmt.Errorf("ghost-ai: %w", err)
	}

	// Clean up model output: strip thinking tags, ChatML tokens, reasoning.
	raw := text
	text = cleanLocalModelResponse(text)
	if strings.TrimSpace(text) == "" {
		// Model produced output but it was all thinking/formatting tokens.
		slog.Warn("[ghost-ai] cleaned response is empty", "raw_len", len(raw), "raw_preview", truncate(raw, 200))
		return nil, fmt.Errorf("ghost-ai returned empty content (model output was all thinking tokens — try a larger model or increase max_tokens)")
	}

	slog.Info("[ghost-ai] complete",
		"prompt_tok", stats.PromptTokens,
		"gen_tok", stats.CompletionTokens,
		"tps", fmt.Sprintf("%.1f", stats.TokensPerSecond),
		"text_len", len(text))

	return &Response{
		Text:     text,
		Provider: "local",
		Model:    c.modelName,
	}, nil
}

// isThinkingModel returns true for models that generate <think> blocks.
func isThinkingModel(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "qwen3") || strings.Contains(n, "deepseek")
}

// cleanLocalModelResponse strips ChatML artifacts, thinking tags, and reasoning
// from small local model output, extracting just the corrected/processed text.
func cleanLocalModelResponse(s string) string {
	// 1. Strip <think>...</think> blocks.
	s = stripThinkingTags(s)

	// 2. Strip ChatML special tokens and control directives that may leak through.
	for _, tok := range []string{"<|im_end|>", "<|im_start|>", "</s>", "<|endoftext|>", "/no_think", "no_think"} {
		s = strings.ReplaceAll(s, tok, "")
	}

	// 3. If model emitted "Answer:" or "Corrected:", take only the text after it.
	for _, marker := range []string{"Answer:", "Answer :", "Corrected:", "Corrected text:"} {
		if idx := strings.LastIndex(s, marker); idx != -1 {
			after := strings.TrimSpace(s[idx+len(marker):])
			if after != "" {
				s = after
				break
			}
		}
	}

	// 4. Truncate at any "User:" or role marker (model continuing the conversation).
	for _, stop := range []string{"\nUser:", "\nuser:", "\nAssistant:", "\nassistant:", "\nSystem:", "\n---"} {
		if idx := strings.Index(s, stop); idx != -1 {
			s = s[:idx]
		}
	}

	// 5. Strip reasoning preambles ("Okay, let's...", "Let me check...", etc.)
	// If the response starts with reasoning, try to find the actual answer after it.
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > 1 {
		first := strings.ToLower(strings.TrimSpace(lines[0]))
		reasoningStarts := []string{"okay,", "ok,", "let me", "let's", "i need to", "first,",
			"looking at", "the user", "checking", "i'll", "now,", "so,", "here"}
		for _, prefix := range reasoningStarts {
			if strings.HasPrefix(first, prefix) {
				// Find the last non-empty line — small models often put the answer last.
				for i := len(lines) - 1; i >= 1; i-- {
					candidate := strings.TrimSpace(lines[i])
					if candidate != "" && !strings.HasPrefix(strings.ToLower(candidate), "answer") {
						s = candidate
						break
					}
				}
				break
			}
		}
	}

	return strings.TrimSpace(s)
}

// truncate returns the first n bytes of s, appending "…" if truncated.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func (c *GhostAIClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.idleTimer != nil {
		c.idleTimer.Stop()
		c.idleTimer = nil
	}
	c.engine.Close()
}
