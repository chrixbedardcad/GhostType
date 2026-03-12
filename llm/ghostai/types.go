// Package ghostai provides an embedded llama.cpp inference engine for GhostSpell.
// It eliminates the subprocess dependency by linking llama.cpp directly via CGo.
//
// Build with -tags ghostai to enable the real engine.
// Without the tag, a stub is used that returns "not available" errors.
package ghostai

import "fmt"

// Config controls the inference engine behavior.
type Config struct {
	ContextSize   int     `json:"context_size"`   // Token context window (default: 2048)
	Threads       int     `json:"threads"`        // CPU threads (0 = auto-detect)
	BatchSize     int     `json:"batch_size"`     // Prompt processing batch size (default: 512)
	MaxTokens     int     `json:"max_tokens"`     // Default max generation tokens (default: 256)
	Temperature   float32 `json:"temperature"`    // Sampling temperature (default: 0.1)
	TopP          float32 `json:"top_p"`          // Nucleus sampling top-p (default: 0.9)
	TopK          int     `json:"top_k"`          // Top-k sampling (default: 40)
	RepeatPenalty float32 `json:"repeat_penalty"` // Repetition penalty (default: 1.1)
	RepeatLastN   int     `json:"repeat_last_n"`  // Tokens for repeat penalty window (default: 64)
	Seed          uint32  `json:"seed"`           // RNG seed (0 = random)
}

// DefaultConfig returns a Config tuned for grammar correction with small models.
func DefaultConfig() Config {
	return Config{
		ContextSize:   2048,
		Threads:       0, // auto-detect
		BatchSize:     512,
		MaxTokens:     256,
		Temperature:   0.1,
		TopP:          0.9,
		TopK:          40,
		RepeatPenalty: 1.1,
		RepeatLastN:   64,
		Seed:          0,
	}
}

// Stats holds performance metrics from a single completion.
type Stats struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	LoadTimeMs       int64   `json:"load_time_ms"`
	PromptTimeMs     int64   `json:"prompt_time_ms"`
	CompletionTimeMs int64   `json:"completion_time_ms"`
	TokensPerSecond  float64 `json:"tokens_per_second"`
}

// ModelInfo describes the loaded model.
type ModelInfo struct {
	Description  string `json:"description"`
	SizeBytes    uint64 `json:"size_bytes"`
	NumParams    uint64 `json:"num_params"`
	ContextTrain int    `json:"context_train"`
	VocabSize    int    `json:"vocab_size"`
}

// engineBackend is implemented by cgoBackend (real) and stubBackend (placeholder).
type engineBackend interface {
	load(modelPath string) error
	complete(prompt string, maxTokens int, abort *int32) (string, Stats, error)
	unload()
	isLoaded() bool
	modelInfo() (ModelInfo, error)
	close()
}

// Sentinel errors for the engine.
var (
	ErrNotLoaded    = fmt.Errorf("ghost-ai: model not loaded")
	ErrNotAvailable = fmt.Errorf("ghost-ai: engine not available (build with -tags ghostai)")
	ErrAborted      = fmt.Errorf("ghost-ai: aborted")
	ErrCircuitOpen  = fmt.Errorf("ghost-ai: circuit breaker open — engine disabled after repeated failures")
)
