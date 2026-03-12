//go:build ghostai && integration

package ghostai

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// Integration tests require:
// 1. Built with -tags "ghostai integration"
// 2. GHOSTAI_TEST_MODEL env var pointing to a GGUF file
//
// Run: go test -tags "ghostai integration" -v ./llm/ghostai/ -run Integration

func testModelPath(t *testing.T) string {
	path := os.Getenv("GHOSTAI_TEST_MODEL")
	if path == "" {
		t.Skip("GHOSTAI_TEST_MODEL not set — skipping integration test")
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("model not found: %s", path)
	}
	return path
}

func TestIntegration_LoadAndInfo(t *testing.T) {
	model := testModelPath(t)

	e := New(DefaultConfig())
	defer e.Close()

	if err := e.Load(model); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !e.IsLoaded() {
		t.Fatal("expected IsLoaded=true after Load")
	}

	info, err := e.ModelInfo()
	if err != nil {
		t.Fatalf("ModelInfo: %v", err)
	}

	t.Logf("Model: %s", info.Description)
	t.Logf("Size: %d MB", info.SizeBytes/(1024*1024))
	t.Logf("Params: %d M", info.NumParams/1_000_000)
	t.Logf("Vocab: %d", info.VocabSize)
	t.Logf("Context train: %d", info.ContextTrain)

	if info.SizeBytes == 0 {
		t.Error("SizeBytes should be non-zero")
	}
	if info.NumParams == 0 {
		t.Error("NumParams should be non-zero")
	}
}

func TestIntegration_SimpleCompletion(t *testing.T) {
	model := testModelPath(t)

	e := New(Config{
		ContextSize: 2048,
		MaxTokens:   64,
		Temperature: 0.1,
		TopP:        0.9,
		TopK:        40,
	})
	defer e.Close()

	if err := e.Load(model); err != nil {
		t.Fatalf("Load: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	prompt := "Fix only spelling and grammar errors. Return ONLY the corrected text.\n\nUser: Ths is a tset of the grammer corrction engne."
	text, stats, err := e.Complete(ctx, prompt, 64)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	t.Logf("Output: %q", text)
	t.Logf("Stats: prompt=%d gen=%d prompt_ms=%d gen_ms=%d tps=%.1f",
		stats.PromptTokens, stats.CompletionTokens,
		stats.PromptTimeMs, stats.CompletionTimeMs,
		stats.TokensPerSecond)

	if text == "" {
		t.Error("expected non-empty output")
	}
	if stats.PromptTokens == 0 {
		t.Error("expected non-zero prompt tokens")
	}
	if stats.CompletionTokens == 0 {
		t.Error("expected non-zero completion tokens")
	}
}

func TestIntegration_Abort(t *testing.T) {
	model := testModelPath(t)

	e := New(Config{
		ContextSize: 2048,
		MaxTokens:   256,
		Temperature: 0.8, // higher temp = more varied output = longer generation
	})
	defer e.Close()

	if err := e.Load(model); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Start completion and abort after 500ms.
	ctx := context.Background()
	go func() {
		time.Sleep(500 * time.Millisecond)
		e.Abort()
	}()

	start := time.Now()
	_, stats, err := e.Complete(ctx, "Write a very long essay about all the countries in the world.", 256)
	elapsed := time.Since(start)

	t.Logf("Abort test: elapsed=%s gen_tokens=%d err=%v", elapsed, stats.CompletionTokens, err)

	// Should complete in roughly 500ms-1s (not the full generation time).
	if elapsed > 5*time.Second {
		t.Errorf("abort took too long: %s (expected <5s)", elapsed)
	}
}

func TestIntegration_ContextCancel(t *testing.T) {
	model := testModelPath(t)

	e := New(Config{
		ContextSize: 2048,
		MaxTokens:   256,
	})
	defer e.Close()

	if err := e.Load(model); err != nil {
		t.Fatalf("Load: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, _, err := e.Complete(ctx, "Write a very long essay about every city in Europe.", 256)
	elapsed := time.Since(start)

	t.Logf("Context cancel test: elapsed=%s err=%v", elapsed, err)

	if elapsed > 5*time.Second {
		t.Errorf("context cancel took too long: %s", elapsed)
	}
}

func TestIntegration_UnloadAndReload(t *testing.T) {
	model := testModelPath(t)

	e := New(DefaultConfig())
	defer e.Close()

	// Load.
	if err := e.Load(model); err != nil {
		t.Fatalf("first Load: %v", err)
	}
	if !e.IsLoaded() {
		t.Fatal("expected loaded")
	}

	// Unload.
	e.Unload()
	if e.IsLoaded() {
		t.Fatal("expected unloaded")
	}

	// Complete should fail.
	_, _, err := e.Complete(context.Background(), "test", 10)
	if err != ErrNotLoaded {
		t.Errorf("expected ErrNotLoaded, got %v", err)
	}

	// Reload.
	if err := e.Load(model); err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if !e.IsLoaded() {
		t.Fatal("expected loaded after reload")
	}
}

func TestIntegration_CircuitBreaker(t *testing.T) {
	e := New(DefaultConfig())
	defer e.Close()

	// Trigger circuit breaker by loading invalid models.
	for i := 0; i < 3; i++ {
		_ = e.Load("/nonexistent/model.gguf")
	}

	if !e.breaker.IsOpen() {
		t.Fatal("circuit should be open after 3 failures")
	}

	// Requests should be rejected.
	_, _, err := e.Complete(context.Background(), "test", 10)
	if err != ErrCircuitOpen {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}

	// Manual reset.
	e.ResetCircuit()
	if e.breaker.IsOpen() {
		t.Fatal("circuit should be closed after reset")
	}
}

func TestIntegration_ConcurrentComplete(t *testing.T) {
	model := testModelPath(t)

	e := New(Config{
		ContextSize: 2048,
		MaxTokens:   32,
		Temperature: 0.1,
	})
	defer e.Close()

	if err := e.Load(model); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Run 3 concurrent completions. They serialize on the mutex but
	// shouldn't deadlock or corrupt state.
	var done int32
	errs := make(chan error, 3)

	for i := 0; i < 3; i++ {
		go func(idx int) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			text, _, err := e.Complete(ctx, "Fix: teh quick brwon fox", 32)
			if err != nil {
				errs <- err
				return
			}
			if text == "" {
				errs <- fmt.Errorf("goroutine %d: empty output", idx)
				return
			}
			atomic.AddInt32(&done, 1)
			errs <- nil
		}(i)
	}

	for i := 0; i < 3; i++ {
		if err := <-errs; err != nil {
			t.Errorf("concurrent completion error: %v", err)
		}
	}

	completed := atomic.LoadInt32(&done)
	t.Logf("Concurrent completions: %d/3 succeeded", completed)
}
