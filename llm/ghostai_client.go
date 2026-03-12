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
func newGhostAIFromDef(def LLMProviderDefCompat) (*GhostAIClient, error) {
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

func (c *GhostAIClient) Provider() string { return "ghostai" }

func (c *GhostAIClient) Send(ctx context.Context, req Request) (*Response, error) {
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

	// Build the same prompt format as the subprocess client.
	prompt := "/no_think\n" + req.Prompt + "\n\nUser: " + req.Text

	text, stats, err := c.engine.Complete(ctx, prompt, maxTokens)
	if err != nil {
		slog.Error("[ghost-ai] completion failed", "error", err)
		return nil, fmt.Errorf("ghost-ai: %w", err)
	}

	// Strip thinking tags (same as subprocess client).
	text = stripThinkingTags(text)
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("ghost-ai returned empty content")
	}

	slog.Info("[ghost-ai] complete",
		"prompt_tok", stats.PromptTokens,
		"gen_tok", stats.CompletionTokens,
		"tps", fmt.Sprintf("%.1f", stats.TokensPerSecond),
		"text_len", len(text))

	return &Response{
		Text:     text,
		Provider: "ghostai",
		Model:    c.modelName,
	}, nil
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
