package ghostai

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Engine is the Ghost-AI inference engine. Thread-safe.
//
// Lifecycle:
//
//	engine := ghostai.New(config)  // create
//	engine.Load("model.gguf")      // load model
//	text, _ := engine.Complete(...) // run inference (repeatable)
//	engine.Unload()                 // free model memory
//	engine.Close()                  // destroy engine
//
// Kill switch: call Abort() to immediately cancel any running inference.
// Circuit breaker: after 3 consecutive failures, the engine disables itself
// for 30 seconds to prevent cascading issues.
type Engine struct {
	mu      sync.Mutex
	config  Config
	tracer  *Tracer
	breaker *CircuitBreaker
	be      engineBackend
	abort   int32 // atomic: 0=ok, 1=abort (checked by C inference loop)
	closed  bool
}

// New creates a Ghost-AI engine with the given configuration.
// The engine is created but no model is loaded yet.
func New(config Config) *Engine {
	applyConfigDefaults(&config)

	tracer := NewTracer(false)
	breaker := NewCircuitBreaker(3, 30000, tracer)

	e := &Engine{
		config:  config,
		tracer:  tracer,
		breaker: breaker,
		be:      newBackend(config),
	}

	return e
}

// Load loads a GGUF model file. Call this before Complete().
// If a model is already loaded, it is unloaded first.
func (e *Engine) Load(modelPath string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return ErrNotAvailable
	}

	e.tracer.TraceLoad(modelPath)
	start := time.Now()

	if err := e.be.load(modelPath); err != nil {
		elapsed := time.Since(start)
		e.tracer.TraceLoadFail(err, elapsed)
		e.breaker.RecordFailure()
		return err
	}

	elapsed := time.Since(start)
	info, _ := e.be.modelInfo()
	e.tracer.TraceLoadDone(info, elapsed)
	e.breaker.RecordSuccess()

	return nil
}

// Complete runs text completion. The prompt should include both the system
// instruction and user text. Returns the generated text and performance stats.
//
// Respects context cancellation: if ctx is cancelled, inference stops and
// partial results may be returned.
//
// The circuit breaker may reject requests if the engine has failed repeatedly.
func (e *Engine) Complete(ctx context.Context, prompt string, maxTokens int) (string, Stats, error) {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return "", Stats{}, ErrNotAvailable
	}

	if !e.be.isLoaded() {
		e.mu.Unlock()
		return "", Stats{}, ErrNotLoaded
	}

	if err := e.breaker.Allow(); err != nil {
		e.mu.Unlock()
		return "", Stats{}, err
	}

	if maxTokens <= 0 {
		maxTokens = e.config.MaxTokens
	}

	// Reset abort flag.
	atomic.StoreInt32(&e.abort, 0)

	e.tracer.TraceComplete(len(prompt), maxTokens)
	e.mu.Unlock()

	// Monitor context cancellation in a goroutine.
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			atomic.StoreInt32(&e.abort, 1)
			e.tracer.TraceAbort()
		case <-done:
		}
	}()

	// Run inference (this is the potentially long-running call).
	text, stats, err := e.be.complete(prompt, maxTokens, &e.abort)
	close(done)

	if err != nil {
		// Don't count user aborts as failures.
		if !isAbortError(err) {
			e.breaker.RecordFailure()
		}
		e.tracer.TraceCompleteFail(err)
		return text, stats, err
	}

	e.breaker.RecordSuccess()
	e.tracer.TraceCompleteDone(stats, len(text))
	return text, stats, nil
}

// Abort immediately cancels any running inference.
// Safe to call from any goroutine. The running Complete() will return
// with whatever text has been generated so far (or an error if nothing was generated).
func (e *Engine) Abort() {
	atomic.StoreInt32(&e.abort, 1)
	e.tracer.TraceAbort()
}

// Unload frees the loaded model from memory.
// The engine remains usable — call Load() again to load a new model.
func (e *Engine) Unload() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.be.isLoaded() {
		e.tracer.TraceUnload()
		e.be.unload()
	}
}

// Close destroys the engine and frees all resources.
// The engine cannot be used after Close().
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}

	e.tracer.Info("close: destroying engine")
	e.be.close()
	e.closed = true
	return nil
}

// IsLoaded returns whether a model is currently loaded.
func (e *Engine) IsLoaded() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.be.isLoaded()
}

// ModelInfo returns information about the loaded model.
func (e *Engine) ModelInfo() (ModelInfo, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.be.modelInfo()
}

// Config returns the current engine configuration.
func (e *Engine) Config() Config {
	return e.config
}

// Tracer returns the engine's tracer for external debug control.
func (e *Engine) Tracer() *Tracer {
	return e.tracer
}

// CircuitBreaker returns the engine's circuit breaker for external monitoring.
func (e *Engine) CircuitBreaker() *CircuitBreaker {
	return e.breaker
}

// ResetCircuit manually resets the circuit breaker, re-enabling the engine.
func (e *Engine) ResetCircuit() {
	e.breaker.Reset()
}

// Available reports whether the Ghost-AI engine is compiled in (vs. stub).
func Available() bool {
	return backendAvailable()
}

// --- Helpers ---

func applyConfigDefaults(c *Config) {
	if c.ContextSize <= 0 {
		c.ContextSize = 2048
	}
	if c.Threads <= 0 {
		c.Threads = runtime.NumCPU()
		if c.Threads > 8 {
			c.Threads = 8 // diminishing returns for small models
		}
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 512
	}
	if c.MaxTokens <= 0 {
		c.MaxTokens = 256
	}
	if c.Temperature <= 0 {
		c.Temperature = 0.1
	}
	if c.TopP <= 0 {
		c.TopP = 0.9
	}
	if c.TopK <= 0 {
		c.TopK = 40
	}
	if c.RepeatPenalty <= 0 {
		c.RepeatPenalty = 1.1
	}
	if c.RepeatLastN <= 0 {
		c.RepeatLastN = 64
	}
}

func isAbortError(err error) bool {
	return err == ErrAborted || (err != nil && err.Error() == "ghost-ai: aborted")
}
