# Comprehensive Code Review: Findings & Recommendations

Deep review of the entire GhostSpell codebase covering security, error handling, reliability, testing, code quality, performance, and observability.

---

## What's Good (Strengths)

- **Clean architecture** — Well-separated packages (`clipboard`, `keyboard`, `hotkey`, `sound`, `llm`, `mode`, `config`, `gui`). Each has a single responsibility.
- **Platform abstraction** — Excellent use of build-tagged files (`*_darwin.go`, `*_windows.go`, `*_linux.go`) for clipboard, keyboard, and hotkey implementations.
- **Multi-strategy text capture** — `capture.go` implements a smart 4-strategy fallback (Accessibility → CGEvent copy → Frontmost app AX → full-screen AX) with detailed inline comments explaining each step.
- **Config validation & migration** — `config/config.go` includes thorough validation (`Validate()`) and migration support (`MigrateIfNeeded()`). Good defensive programming.
- **Solid test coverage for core modules** — `config/config_test.go`, `llm/anthropic_test.go`, `llm/openai_test.go`, `mode/router_test.go` cover critical paths with table-driven tests.
- **Context-based LLM cancellation** — `process.go` properly uses `context.WithCancel` so in-flight LLM requests can be aborted.
- **Circuit breaker in Ghost-AI** — `llm/ghostai.go` implements circuit breaker pattern for API resilience.
- **Structured logging** — Consistent use of `slog` throughout.
- **Connection pooling** — `llm/transport.go` reuses HTTP connections with keep-alive to save TLS handshake overhead.
- **Well-written README and inline comments** — Especially `capture.go` which documents macOS-specific quirks extensively.

---

## Security (HIGH Priority)

### 1. Config file permissions too open — API keys world-readable
**File:** `config/config.go:462`
```go
return os.WriteFile(path, data, 0644)
```
API keys are stored in `config.json` with `0644` (owner read/write, world-readable). Any local user can read them.

**Fix:** Use `0600` for config files containing secrets.

### 2. User text logged in plaintext to stdout
**File:** `process.go:168-169`
```go
slog.Info("Captured text", "prompt", promptName, "len", len(cap.Text), "selection", cap.HasAX, "method", cap.Method, "text", cap.Text)
fmt.Printf("[%s] Captured: %q\n", promptName, cap.Text)
```
Full user-captured text (potentially passwords, private messages, medical info) is logged to both `slog` and stdout. This is a privacy risk — log files or terminal history could expose sensitive data.

**Fix:** Remove `"text", cap.Text` from production logging. Keep only length. Use debug-level only if needed.

### 3. OAuth global mutable state with no PKCE state timeout
**File:** `gui/oauth.go:32-41`
```go
var oauth oauthState // global, mutable singleton
```
The PKCE state parameter is generated (`randomState()`) but has no expiration timeout. If intercepted, it could be replayed indefinitely. The global `oauthState` singleton with a simple `running` bool flag is vulnerable to race conditions if multiple OAuth flows are triggered rapidly.

**Fix:** Add a timestamp to the state and reject states older than ~5 minutes. Consider making `running` an atomic bool.

### 4. No input length validation at entry point
**File:** `mode/router.go:39-41`
```go
if strings.TrimSpace(text) == "" {
    return nil, fmt.Errorf("nothing to process: empty text")
}
```
Only empty strings are rejected here. While `MaxInputChars` is checked later in `process.go`, extremely large clipboard contents could consume memory before reaching that check.

**Fix:** Validate length at the earliest entry point.

### 5. API keys stored in plaintext JSON with no encryption at rest
**File:** `config/config.go` — `ProviderConfig` struct

API keys are written as plain strings in the JSON config. On shared machines or if the config directory is backed up to cloud storage, keys are fully exposed.

**Fix:** Consider using OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service) for API key storage.

---

## Error Handling (MEDIUM Priority)

### 6. `json.Marshal` error silently ignored
**File:** `stats/stats.go:128`
```go
data, _ := json.Marshal(summary)
```
If marshaling fails (e.g., due to unexpected data), the error is silently discarded and an empty/malformed string is returned.

**Fix:** Return the error or log it.

### 7. `os.Remove` error not checked
**File:** `stats/stats.go:137`
```go
os.Remove(s.path)
```
File removal error (e.g., permission denied, file locked) is completely ignored.

**Fix:** At minimum log a warning.

### 8. Config backup restore silently fails
**File:** `main.go:182-192`
```go
os.WriteFile(configPath, bdata, 0644)  // Error ignored
cfg, _ = config.LoadRaw(configPath)    // Error ignored
```
If the backup restore fails, the app continues with a potentially broken config and the user has no idea the restore didn't work.

**Fix:** Check and log these errors. Consider notifying the user.

### 9. Async stats writes are fire-and-forget
**File:** `stats/stats.go:99-108`

Stats are written in a goroutine with no feedback mechanism to the caller. If writes consistently fail (e.g., disk full), the user loses all usage data silently.

**Fix:** Log errors and consider a health check mechanism.

---

## Reliability & Resilience (MEDIUM Priority)

### 10. No retry/backoff for transient LLM API failures
Most LLM provider clients (`anthropic.go`, `openai.go`, `gemini.go`, etc.) make a single HTTP request with no retry logic. Transient network errors (DNS hiccups, 502/503 responses) cause immediate failure. Only `ghostai.go` has a circuit breaker.

**Fix:** Add configurable retry with exponential backoff for 5xx and network errors across all providers.

### 11. Hardcoded sleep delays in clipboard capture
**File:** `capture.go:84,97`
```go
time.Sleep(50 * time.Millisecond)
// ...
delay := time.Duration(100+attempt*50) * time.Millisecond // 100ms, 150ms, 200ms
```
Fixed delays that may be too short on slow machines or too long on fast ones. Electron apps, remote desktops, and games are called out in comments as needing more time.

**Fix:** Make clipboard retry delays configurable or use adaptive timing.

### 12. Full config replacement is not atomic relative to in-flight requests
**File:** `app.go:194-198`
```go
mu.Lock()
*cfg = *newCfg  // Replace entire config struct
if router != nil {
    router.ResetClients()
}
```
There's a brief window where the router may use old clients while config has already changed. If an LLM request is in-flight during config save, behavior is undefined.

**Fix:** Use copy-on-write or versioned config with proper synchronization.

### 13. No watchdog timer beyond HTTP timeout
If an LLM provider hangs at the TCP level (connection established but no response), the only safeguard is the HTTP client timeout. There's no application-level watchdog to detect and recover from stuck processing pipelines.

---

## Test Coverage Gaps (MEDIUM Priority)

### 14. No tests for `capture.go`
The text capture module has the most complex logic in the app (4-strategy fallback, platform-specific behavior, retry loops) but zero test coverage. This is the highest-risk untested code.

### 15. No tests for hotkey registration/deregistration
`hotkey/` package has no tests. Registration failures, duplicate hotkey conflicts, and cleanup on shutdown are untested.

### 16. No tests for keyboard simulation
`keyboard/` package (Copy, Paste, SelectAll) has no tests. These are critical to the core workflow.

### 17. GUI logic mostly untested
`gui/bindings_test.go` only checks method presence via reflection — no behavioral tests for settings save/load, OAuth flow, or update checks.

### 18. No concurrency tests
No tests for concurrent hotkey presses (the `processingGuard` mutex), concurrent config saves, or concurrent LLM cancellation.

### 19. No tests for malformed/truncated LLM responses
Provider tests use well-formed responses. No tests for partial JSON, unexpected status codes, or empty bodies.

---

## Code Quality (LOW-MEDIUM Priority)

### 20. Provider creation uses switch statement — not extensible
**File:** `llm/client.go:45-90`
```go
switch def.Provider {
case "anthropic":
    return newAnthropicFromDef(def), nil
// ... 7 more cases
}
```
Adding a new provider requires modifying this switch. A registry pattern would be more maintainable.

### 21. Client defaults duplicated across 7 provider files
Each provider file (`anthropic.go`, `openai.go`, `gemini.go`, `deepseek.go`, `ollama.go`, `lmstudio.go`, `xai.go`) duplicates:
- Timeout default logic
- MaxTokens default logic
- HTTP client creation

Could be extracted into a shared `applyProviderDefaults()` helper.

### 22. Provider string keys not type-safe
**File:** `config/config.go:607-617`
```go
validProviders := map[string]bool{
    "anthropic": true,
    "openai":    true,
    // ...
}
```
Provider names are raw strings throughout. A typo compiles fine but fails at runtime.

**Fix:** Use a `ProviderType` type with const values.

### 23. Map iteration order is undefined in config validation
**File:** `config/config.go:283` and others

Multiple maps are iterated without sorting keys. This makes validation output, error messages, and UI display order nondeterministic.

### 24. Settings service lacks transaction semantics
**File:** `gui/service_config.go`

Multiple setter methods modify `cfgCopy` incrementally, but validation only runs on save. Intermediate states may be inconsistent (e.g., provider set but model not yet assigned).

---

## Performance (LOW Priority)

### 25. Slice allocated on every tray menu refresh
**File:** `mode/router.go:253-257`
```go
names := make([]string, len(cfg.Prompts))
for i, p := range cfg.Prompts {
    names[i] = p.Name
}
```
A new slice is created every time the tray menu is refreshed. Could cache until config changes.

### 26. String concatenation for map keys in stats
**File:** `stats/stats.go:179`
```go
key := e.ModelLabel + "|" + e.Provider + "|" + e.Model
```
Creates new strings in a loop. A struct key or `strings.Builder` would be more efficient.

### 27. Connection pool defaults may be too conservative
**File:** `llm/transport.go:21-22`
```go
MaxIdleConns:        5,
MaxIdleConnsPerHost: 2,
```
With multiple providers and concurrent requests, these limits may cause unnecessary connection churn.

### 28. Full stats file read/written on every invocation
**File:** `stats/stats.go`

The entire stats file (up to 1000 entries) is read on startup and fully rewritten after each invocation. No incremental append or buffering.

---

## Observability (LOW Priority)

### 29. No metrics collection
No request counts, latency histograms, error rates, or provider-specific metrics. Makes it hard to diagnose issues in production.

### 30. No request correlation IDs
The capture → LLM → paste pipeline has no request ID to correlate log entries across the full flow.

### 31. No health check for API connectivity
No proactive check whether configured providers are reachable. Users only discover connectivity issues when they trigger a hotkey.

### 32. Debug log timeout hardcoded
**File:** `internal/debuglog/debuglog.go:17`

Debug logging auto-disables after 30 minutes. This is not configurable.

---

## Suggested Priority Order

| Priority | Items | Action |
|----------|-------|--------|
| **P0 — Do Now** | #1 (file perms), #2 (text logging) | Security/privacy fixes |
| **P1 — Soon** | #3 (OAuth), #6-#8 (error handling), #10 (retry/backoff) | Reliability |
| **P2 — Next** | #14-#16 (test coverage for capture/hotkey/keyboard) | Quality |
| **P3 — Later** | #20-#21 (provider registry, dedup), #5 (keychain) | Architecture |
| **P4 — Nice to have** | #25-#32 (perf, observability) | Polish |
