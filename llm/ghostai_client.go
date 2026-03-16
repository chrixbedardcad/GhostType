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
	// Thinking models get extra context as a safety margin. With thinking
	// properly disabled via template (empty <think> block for Qwen3.5,
	// /no_think for Qwen3), this mostly serves as headroom for the
	// </think> early stop safety net in bridge.c.
	if isThinkingModel(def.Model) {
		config.ContextSize = 2048
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

	// Dynamic token cap: allow ~3x input length + generous buffer.
	// With thinking properly disabled via template (see below), the model
	// outputs directly without <think> blocks, so we don't need the full
	// context window. Keep a safety margin of 512 minimum for complex text.
	inputWords := len(strings.Fields(req.Text))
	dynamicMax := inputWords*3 + 128
	if dynamicMax < 512 {
		dynamicMax = 512
	}
	if thinking {
		// Thinking models: even with thinking disabled, allow generous room.
		// The </think> early stop in bridge.c acts as a safety net if
		// thinking somehow triggers despite the template disable.
		if dynamicMax < 1024 {
			dynamicMax = 1024
		}
	}
	if dynamicMax < maxTokens {
		maxTokens = dynamicMax
	}
	if maxTokens < 64 {
		maxTokens = 64
	}

	// Format using the model's chat template (ChatML for Qwen, etc.).
	// System = instruction prompt, User = the text to process.
	systemMsg := req.Prompt
	if thinking && !isQwen35(c.modelName) {
		// Qwen3 (not 3.5) supports the /no_think soft switch in system message.
		systemMsg = "/no_think\n" + req.Prompt
	}
	prompt, err := c.engine.ApplyChat(systemMsg, req.Text)
	if err != nil {
		slog.Warn("[ghost-ai] chat template failed, using raw format", "error", err)
		// Fallback to raw format if template fails.
		prompt = systemMsg + "\n\nUser: " + req.Text
	}

	// For Qwen3.5: inject an empty <think> block after the assistant turn
	// start to disable thinking at the template level. This is the official
	// Qwen3.5 mechanism (enable_thinking=false) — the model sees the empty
	// block and skips reasoning, going straight to the answer.
	// Qwen3.5 does NOT support the /no_think soft switch.
	if isQwen35(c.modelName) {
		prompt += "<think>\n\n</think>\n\n"
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
		// Try to extract useful content from inside the last <think> block —
		// often the corrected text appears near the end of the thinking.
		text = extractFromThinking(raw)
		if strings.TrimSpace(text) == "" {
			slog.Warn("[ghost-ai] cleaned response is empty", "raw_len", len(raw), "raw_preview", truncate(raw, 200))
			return nil, fmt.Errorf("ghost-ai returned empty content (model output was all thinking tokens — try a larger model or increase max_tokens)")
		}
		slog.Info("[ghost-ai] extracted answer from thinking block", "text_len", len(text))
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

// isThinkingModel returns true for models that can generate <think> blocks.
func isThinkingModel(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "qwen3") || strings.Contains(n, "deepseek")
}

// isQwen35 returns true for Qwen3.5 models (not Qwen3).
// Qwen3.5 does NOT support the /no_think soft switch — it requires
// template-level disable via an empty <think></think> block.
func isQwen35(name string) bool {
	return strings.Contains(strings.ToLower(name), "qwen3.5")
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

	// 3. If the first line starts with "Answer:" or "Corrected:", strip that prefix.
	// Only match at the beginning of the text to avoid truncating valid content
	// that happens to contain these words mid-sentence.
	trimmed := strings.TrimSpace(s)
	firstLine := trimmed
	if nl := strings.IndexByte(trimmed, '\n'); nl != -1 {
		firstLine = trimmed[:nl]
	}
	for _, marker := range []string{"Answer:", "Answer :", "Corrected:", "Corrected text:"} {
		if strings.HasPrefix(firstLine, marker) {
			after := strings.TrimSpace(trimmed[len(marker):])
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

// extractFromThinking tries to find the corrected/processed text inside
// a <think> block when the model's entire output was thinking content.
// Thinking models often put the final answer near the end of the thinking
// block, preceded by markers like "Corrected:", "Answer:", "Result:", etc.
func extractFromThinking(raw string) string {
	// Find content inside <think>...</think>
	start := strings.Index(raw, "<think>")
	if start == -1 {
		return ""
	}
	content := raw[start+7:]
	if end := strings.Index(content, "</think>"); end != -1 {
		content = content[:end]
	}

	// Look for common answer markers near the end of the thinking block.
	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		for _, marker := range []string{
			"Corrected text:", "Corrected:", "Corrected Text:",
			"Answer:", "Result:", "Output:", "Fixed:",
			"Final answer:", "Final:", "Response:",
		} {
			if strings.HasPrefix(line, marker) {
				answer := strings.TrimSpace(line[len(marker):])
				// If the answer continues on subsequent lines, collect them.
				for j := i + 1; j < len(lines); j++ {
					next := strings.TrimSpace(lines[j])
					if next == "" || strings.HasPrefix(next, "**") || strings.HasPrefix(next, "---") {
						break
					}
					answer += "\n" + next
				}
				if answer != "" {
					return cleanLocalModelResponse(answer)
				}
			}
		}
	}
	return ""
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
