package gui

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

// oauthKeyResponse is the response from OpenRouter's key exchange endpoint.
type oauthKeyResponse struct {
	Key    string `json:"key"`
	UserID string `json:"user_id"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// oauthState holds the state of an in-progress OAuth flow so the
// Wails binding doesn't block the main thread.
type oauthState struct {
	mu      sync.Mutex
	running bool
	key     string
	errMsg  string
}

var oauth oauthState

// generatePKCE creates a cryptographically random code_verifier and its
// S256 code_challenge for the PKCE OAuth flow.
func generatePKCE() (verifier, challenge string) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return
}

// openBrowserURL opens a URL in the user's default browser.
func openBrowserURL(rawURL string) error {
	slog.Info("OAuth: opening browser", "url", rawURL)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	case "darwin":
		cmd = exec.Command("open", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}

// exchangeCodeForKey sends the auth code + PKCE verifier to OpenRouter and
// returns the permanent API key.
func exchangeCodeForKey(code, verifier string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"code":                  code,
		"code_verifier":         verifier,
		"code_challenge_method": "S256",
	})

	slog.Info("OAuth: exchanging code for key", "code_len", len(code))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://openrouter.ai/api/v1/auth/keys",
		bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("key exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	slog.Info("OAuth: key exchange response", "status", resp.StatusCode, "body", string(respBody))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("key exchange failed (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var result oauthKeyResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("OpenRouter error %d: %s", result.Error.Code, result.Error.Message)
	}

	if result.Key == "" {
		return "", fmt.Errorf("empty key in response")
	}

	return result.Key, nil
}

// startOpenRouterOAuthAsync runs the OAuth flow in a background goroutine.
// The result can be polled via getOAuthResult().
func startOpenRouterOAuthAsync() {
	oauth.mu.Lock()
	if oauth.running {
		oauth.mu.Unlock()
		return
	}
	oauth.running = true
	oauth.key = ""
	oauth.errMsg = ""
	oauth.mu.Unlock()

	go func() {
		key, err := runOAuthFlow()

		oauth.mu.Lock()
		defer oauth.mu.Unlock()
		oauth.running = false
		if err != nil {
			oauth.errMsg = err.Error()
			slog.Error("OAuth flow failed", "error", err)
		} else {
			oauth.key = key
			slog.Info("OAuth flow succeeded")
		}
	}()
}

// getOAuthResult returns the current state of the OAuth flow.
// Returns: "pending", "error: ...", or "ok:sk-or-v1-..."
func getOAuthResult() string {
	oauth.mu.Lock()
	defer oauth.mu.Unlock()
	if oauth.running {
		return "pending"
	}
	if oauth.errMsg != "" {
		msg := oauth.errMsg
		oauth.errMsg = "" // clear for next attempt
		return "error: " + msg
	}
	if oauth.key != "" {
		key := oauth.key
		oauth.key = "" // clear for security
		return "ok:" + key
	}
	return "error: no OAuth flow started"
}

// runOAuthFlow executes the full PKCE flow synchronously.
func runOAuthFlow() (string, error) {
	// OpenRouter only allows callback URLs on port 443 (HTTPS) or 3000 (localhost).
	const port = 3000
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	callbackURL := fmt.Sprintf("http://localhost:%d/callback", port)

	slog.Info("OAuth: starting flow", "callback", callbackURL)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		// Port might be in use — try a random port as fallback
		slog.Warn("OAuth: fixed port unavailable, trying random", "error", err)
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return "", fmt.Errorf("start listener: %w", err)
		}
		actualPort := listener.Addr().(*net.TCPAddr).Port
		callbackURL = fmt.Sprintf("http://localhost:%d/callback", actualPort)
		slog.Info("OAuth: using random port", "port", actualPort)
	}

	verifier, challenge := generatePKCE()

	type callbackResult struct {
		code string
		err  error
	}
	resultCh := make(chan callbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		slog.Info("OAuth: callback received", "url", r.URL.String())

		// Check for error response from OpenRouter
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			desc := r.URL.Query().Get("error_description")
			if desc == "" {
				desc = errMsg
			}
			slog.Error("OAuth callback: error from provider", "error", errMsg, "description", desc)
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(400)
			fmt.Fprint(w, oauthErrorPage(desc))
			resultCh <- callbackResult{err: fmt.Errorf("%s", desc)}
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			slog.Error("OAuth callback: no code parameter", "query", r.URL.RawQuery)
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(400)
			fmt.Fprint(w, oauthErrorPage("No authorization code received. Please try again."))
			resultCh <- callbackResult{err: fmt.Errorf("no code in callback (query: %s)", r.URL.RawQuery)}
			return
		}

		slog.Info("OAuth callback: received code", "code_len", len(code))
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, oauthSuccessPage())
		resultCh <- callbackResult{code: code}
	})

	server := &http.Server{Handler: mux}
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
			slog.Error("OAuth server error", "error", serveErr)
		}
	}()
	defer func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutCancel()
		server.Shutdown(shutCtx)
	}()

	// Build auth URL
	authURL := fmt.Sprintf("https://openrouter.ai/auth?callback_url=%s&code_challenge=%s&code_challenge_method=S256",
		url.QueryEscape(callbackURL), url.QueryEscape(challenge))

	if err := openBrowserURL(authURL); err != nil {
		return "", fmt.Errorf("failed to open browser: %w", err)
	}

	slog.Info("OAuth: browser opened, waiting for callback...")

	// Wait for callback (5 minute timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	select {
	case res := <-resultCh:
		if res.err != nil {
			return "", res.err
		}
		key, err := exchangeCodeForKey(res.code, verifier)
		if err != nil {
			return "", err
		}
		prefix := key
		if len(prefix) > 12 {
			prefix = prefix[:12]
		}
		slog.Info("OAuth: API key received", "key_prefix", prefix+"...")
		return key, nil
	case <-ctx.Done():
		return "", fmt.Errorf("timed out waiting for authorization (5 minutes)")
	}
}

func oauthSuccessPage() string {
	return `<!DOCTYPE html>
<html><head><style>
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;background:#1e1e2e;color:#cdd6f4;display:flex;align-items:center;justify-content:center;height:100vh;margin:0}
.box{text-align:center;padding:40px;background:#313244;border-radius:12px;max-width:400px}
h2{color:#a6e3a1;margin-bottom:12px}
p{color:#a6adc8;font-size:0.9em}
</style></head><body>
<div class="box">
<h2>&#9989; Authorization Successful!</h2>
<p>You can close this tab and return to GhostType.</p>
</div>
</body></html>`
}

func oauthErrorPage(msg string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html><head><style>
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;background:#1e1e2e;color:#cdd6f4;display:flex;align-items:center;justify-content:center;height:100vh;margin:0}
.box{text-align:center;padding:40px;background:#313244;border-radius:12px;max-width:400px}
h2{color:#f38ba8;margin-bottom:12px}
p{color:#a6adc8;font-size:0.9em}
</style></head><body>
<div class="box">
<h2>&#10060; Authorization Failed</h2>
<p>%s</p>
</div>
</body></html>`, msg)
}
