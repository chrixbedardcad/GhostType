package ghostai

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// --- Config defaults ---

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if c.ContextSize != 2048 {
		t.Errorf("ContextSize = %d, want 2048", c.ContextSize)
	}
	if c.Temperature != 0.1 {
		t.Errorf("Temperature = %f, want 0.1", c.Temperature)
	}
	if c.MaxTokens != 256 {
		t.Errorf("MaxTokens = %d, want 256", c.MaxTokens)
	}
}

func TestApplyConfigDefaults(t *testing.T) {
	c := Config{} // all zeros
	applyConfigDefaults(&c)

	if c.ContextSize != 2048 {
		t.Errorf("ContextSize = %d, want 2048", c.ContextSize)
	}
	if c.Threads <= 0 {
		t.Errorf("Threads should be auto-detected, got %d", c.Threads)
	}
	if c.BatchSize != 512 {
		t.Errorf("BatchSize = %d, want 512", c.BatchSize)
	}
	if c.TopK != 40 {
		t.Errorf("TopK = %d, want 40", c.TopK)
	}
}

func TestApplyConfigDefaults_PreservesExplicit(t *testing.T) {
	c := Config{
		ContextSize: 4096,
		Threads:     2,
		Temperature: 0.5,
	}
	applyConfigDefaults(&c)

	if c.ContextSize != 4096 {
		t.Errorf("ContextSize = %d, want 4096", c.ContextSize)
	}
	if c.Threads != 2 {
		t.Errorf("Threads = %d, want 2", c.Threads)
	}
	if c.Temperature != 0.5 {
		t.Errorf("Temperature = %f, want 0.5", c.Temperature)
	}
}

// --- Engine lifecycle ---

func TestNew(t *testing.T) {
	e := New(DefaultConfig())
	if e == nil {
		t.Fatal("New returned nil")
	}
	defer e.Close()

	if e.IsLoaded() {
		t.Error("new engine should not have a model loaded")
	}
}

func TestEngine_CompleteWithoutLoad(t *testing.T) {
	e := New(DefaultConfig())
	defer e.Close()

	_, _, err := e.Complete(context.Background(), "test", 10)
	if err != ErrNotLoaded && err != ErrNotAvailable {
		t.Errorf("expected ErrNotLoaded or ErrNotAvailable, got %v", err)
	}
}

func TestEngine_LoadNonexistent(t *testing.T) {
	e := New(DefaultConfig())
	defer e.Close()

	err := e.Load("/nonexistent/path/model.gguf")
	if err == nil {
		t.Error("expected error loading nonexistent model")
	}
}

func TestEngine_DoubleClose(t *testing.T) {
	e := New(DefaultConfig())
	if err := e.Close(); err != nil {
		t.Errorf("first close: %v", err)
	}
	if err := e.Close(); err != nil {
		t.Errorf("second close: %v", err)
	}
}

func TestEngine_CloseRejectsRequests(t *testing.T) {
	e := New(DefaultConfig())
	e.Close()

	_, _, err := e.Complete(context.Background(), "test", 10)
	if err != ErrNotAvailable {
		t.Errorf("expected ErrNotAvailable after close, got %v", err)
	}
}

func TestEngine_Available(t *testing.T) {
	// This returns true with ghostai tag, false without.
	// We just verify it doesn't panic.
	_ = Available()
}

// --- Circuit Breaker ---

func TestCircuitBreaker_StartsClosedl(t *testing.T) {
	cb := NewCircuitBreaker(3, 1000, nil)
	if cb.IsOpen() {
		t.Error("circuit should start closed")
	}
	if cb.State() != "closed" {
		t.Errorf("state = %s, want closed", cb.State())
	}
}

func TestCircuitBreaker_TripsAfterMaxFails(t *testing.T) {
	cb := NewCircuitBreaker(3, 1000, nil)

	cb.RecordFailure()
	cb.RecordFailure()
	if cb.IsOpen() {
		t.Error("should not trip after 2 failures (max=3)")
	}

	cb.RecordFailure()
	if !cb.IsOpen() {
		t.Error("should trip after 3 failures")
	}
	if cb.State() != "open" {
		t.Errorf("state = %s, want open", cb.State())
	}

	err := cb.Allow()
	if err != ErrCircuitOpen {
		t.Errorf("Allow() = %v, want ErrCircuitOpen", err)
	}
}

func TestCircuitBreaker_SuccessResetsCount(t *testing.T) {
	cb := NewCircuitBreaker(3, 1000, nil)

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()

	if cb.Failures() != 0 {
		t.Errorf("failures = %d, want 0 after success", cb.Failures())
	}

	// Should not trip now.
	cb.RecordFailure()
	if cb.IsOpen() {
		t.Error("should not trip after 1 failure (reset by success)")
	}
}

func TestCircuitBreaker_CooldownAndHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(1, 50, nil) // 50ms cooldown

	cb.RecordFailure() // trips immediately (maxFails=1)
	if !cb.IsOpen() {
		t.Fatal("should be open")
	}

	// Wait for cooldown.
	time.Sleep(60 * time.Millisecond)

	err := cb.Allow()
	if err != nil {
		t.Errorf("Allow() after cooldown should succeed, got %v", err)
	}
	if cb.State() != "half-open" {
		t.Errorf("state = %s, want half-open", cb.State())
	}

	// Success closes the circuit.
	cb.RecordSuccess()
	if cb.State() != "closed" {
		t.Errorf("state = %s, want closed", cb.State())
	}
}

func TestCircuitBreaker_ManualReset(t *testing.T) {
	cb := NewCircuitBreaker(1, 60000, nil)

	cb.RecordFailure()
	if !cb.IsOpen() {
		t.Fatal("should be open")
	}

	cb.Reset()
	if cb.IsOpen() {
		t.Error("should be closed after reset")
	}
	if cb.Failures() != 0 {
		t.Errorf("failures = %d, want 0", cb.Failures())
	}
}

// --- Tracer ---

func TestTracer_VerboseToggle(t *testing.T) {
	tr := NewTracer(false)
	if tr.Verbose() {
		t.Error("should start non-verbose")
	}

	tr.SetVerbose(true)
	if !tr.Verbose() {
		t.Error("should be verbose after SetVerbose(true)")
	}
}

// --- Abort ---

func TestEngine_Abort(t *testing.T) {
	e := New(DefaultConfig())
	defer e.Close()

	// Abort before any operation — should set the flag.
	e.Abort()
	if atomic.LoadInt32(&e.abort) != 1 {
		t.Error("abort flag should be set")
	}
}

func TestEngine_ContextCancellation(t *testing.T) {
	e := New(DefaultConfig())
	defer e.Close()

	// Cancel context before Complete — should set abort flag via goroutine.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediate cancel

	_, _, err := e.Complete(ctx, "test", 10)
	// Depending on build (stub vs cgo), we get different errors.
	// Just verify it doesn't hang.
	if err == nil {
		t.Error("expected error with cancelled context")
	}
}

// --- Integration test placeholder ---
// Integration tests that require a real model file use the "integration" build tag.
// See engine_integration_test.go.
