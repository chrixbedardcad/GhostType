package ghostai

import (
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"
)

// Tracer provides structured debug output for Ghost-AI engine operations.
// All operations are logged via slog; when verbose mode is on, extra per-token
// detail is emitted.
type Tracer struct {
	verbose atomic.Bool
}

// NewTracer creates a tracer. Verbose mode can be toggled at runtime.
func NewTracer(verbose bool) *Tracer {
	t := &Tracer{}
	t.verbose.Store(verbose)
	return t
}

// SetVerbose enables/disables per-token trace output.
func (t *Tracer) SetVerbose(v bool) { t.verbose.Store(v) }

// Verbose returns whether per-token tracing is on.
func (t *Tracer) Verbose() bool { return t.verbose.Load() }

func (t *Tracer) Info(msg string, args ...any) {
	slog.Info("[ghost-ai] "+msg, args...)
	fmt.Printf("[ghost-ai] "+msg+"\n", args...)
}

func (t *Tracer) Debug(msg string, args ...any) {
	slog.Debug("[ghost-ai] "+msg, args...)
}

func (t *Tracer) Error(msg string, args ...any) {
	slog.Error("[ghost-ai] "+msg, args...)
	fmt.Printf("[ghost-ai] ERROR: "+msg+"\n", args...)
}

func (t *Tracer) Warn(msg string, args ...any) {
	slog.Warn("[ghost-ai] "+msg, args...)
}

// TraceLoad logs model loading.
func (t *Tracer) TraceLoad(path string) {
	t.Info("load: path=%s", path)
}

// TraceLoadDone logs successful model load with timing and model info.
func (t *Tracer) TraceLoadDone(info ModelInfo, elapsed time.Duration) {
	sizeMB := info.SizeBytes / (1024 * 1024)
	paramsM := info.NumParams / 1_000_000
	t.Info("load: complete elapsed=%s size=%dMB params=%dM vocab=%d ctx_train=%d desc=%s",
		elapsed.Round(time.Millisecond), sizeMB, paramsM, info.VocabSize, info.ContextTrain, info.Description)
}

// TraceLoadFail logs model load failure.
func (t *Tracer) TraceLoadFail(err error, elapsed time.Duration) {
	t.Error("load: FAILED elapsed=%s error=%v", elapsed.Round(time.Millisecond), err)
}

// TraceComplete logs the start of a completion.
func (t *Tracer) TraceComplete(promptLen int, maxTokens int) {
	t.Info("complete: prompt_len=%d max_tokens=%d", promptLen, maxTokens)
}

// TraceCompleteDone logs completion results.
func (t *Tracer) TraceCompleteDone(stats Stats, textLen int) {
	t.Info("complete: done prompt_tok=%d gen_tok=%d prompt_ms=%d gen_ms=%d tps=%.1f text_len=%d",
		stats.PromptTokens, stats.CompletionTokens,
		stats.PromptTimeMs, stats.CompletionTimeMs,
		stats.TokensPerSecond, textLen)
}

// TraceCompleteFail logs completion failure.
func (t *Tracer) TraceCompleteFail(err error) {
	t.Error("complete: FAILED error=%v", err)
}

// TraceAbort logs an abort event.
func (t *Tracer) TraceAbort() {
	t.Warn("abort: user cancelled inference")
}

// TraceUnload logs model unload.
func (t *Tracer) TraceUnload() {
	t.Info("unload: freeing model memory")
}

// TraceCircuitTrip logs circuit breaker activation.
func (t *Tracer) TraceCircuitTrip(failures int) {
	t.Error("circuit: TRIPPED after %d consecutive failures — engine disabled", failures)
}

// TraceCircuitReset logs circuit breaker recovery.
func (t *Tracer) TraceCircuitReset() {
	t.Info("circuit: reset — engine re-enabled")
}
