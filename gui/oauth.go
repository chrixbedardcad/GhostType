package gui

import (
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
	"strings"
	"sync"
	"time"
)

// OpenAI OAuth constants — uses the public Codex CLI client ID.
const (
	openaiClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
	openaiAuthURL  = "https://auth.openai.com/oauth/authorize"
	openaiTokenURL = "https://auth.openai.com/oauth/token"
	openaiScopes   = "openid profile email offline_access"
	oauthPort      = 1455
	oauthPath      = "/auth/callback"
)

// oauthState holds the async result of the OAuth flow.
type oauthState struct {
	mu           sync.Mutex
	running      bool
	apiKey       string
	refreshToken string
	errMsg       string
}

var oauth oauthState

// generatePKCE creates a PKCE code_verifier (64 random bytes, base64url) and
// its S256 code_challenge.
func generatePKCE() (verifier, challenge string) {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return
}

// randomState generates a random state parameter for CSRF protection.
func randomState() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// openBrowserURL opens a URL in the user's default browser.
func openBrowserURL(rawURL string) error {
	slog.Info("OAuth: opening browser", "url_len", len(rawURL))
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

// tokenResponse represents the response from OpenAI's token endpoint.
type tokenResponse struct {
	IDToken      string          `json:"id_token"`
	AccessToken  string          `json:"access_token"`
	RefreshToken string          `json:"refresh_token"`
	Error        json.RawMessage `json:"error,omitempty"`
	ErrorDesc    string          `json:"error_description,omitempty"`
}

// errorString extracts a human-readable error from the token response.
// Handles both string errors ("invalid_grant") and object errors ({"message":"..."}).
func (r *tokenResponse) errorString() string {
	if len(r.Error) == 0 {
		return ""
	}
	// Try string first.
	var s string
	if json.Unmarshal(r.Error, &s) == nil && s != "" {
		return s
	}
	// Try object with message field.
	var obj struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	}
	if json.Unmarshal(r.Error, &obj) == nil && obj.Message != "" {
		if obj.Type != "" {
			return obj.Type + ": " + obj.Message
		}
		return obj.Message
	}
	return string(r.Error)
}

// exchangeCodeForTokens exchanges the authorization code for tokens.
func exchangeCodeForTokens(code, verifier, redirectURI string) (*tokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {openaiClientID},
		"code_verifier": {verifier},
	}

	slog.Info("OAuth: exchanging code for tokens")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openaiTokenURL,
		strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	slog.Info("OAuth: token response", "status", resp.StatusCode)

	var result tokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w (body: %s)", err, string(body))
	}

	if errStr := result.errorString(); errStr != "" {
		return nil, fmt.Errorf("token error: %s — %s", errStr, result.ErrorDesc)
	}

	if result.IDToken == "" {
		return nil, fmt.Errorf("no id_token in response")
	}

	return &result, nil
}

// exchangeIDTokenForAPIKey performs a token-exchange to get a standard sk-... API key.
func exchangeIDTokenForAPIKey(idToken string) (string, error) {
	data := url.Values{
		"grant_type":         {"urn:ietf:params:oauth:grant-type:token-exchange"},
		"client_id":          {openaiClientID},
		"requested_token":    {"openai-api-key"},
		"subject_token":      {idToken},
		"subject_token_type": {"urn:ietf:params:oauth:token-type:id_token"},
	}

	slog.Info("OAuth: exchanging id_token for API key")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openaiTokenURL,
		strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API key exchange failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	slog.Info("OAuth: API key exchange response", "status", resp.StatusCode)

	var result tokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse response: %w (body: %s)", err, string(body))
	}

	if errStr := result.errorString(); errStr != "" {
		return "", fmt.Errorf("API key exchange error: %s — %s", errStr, result.ErrorDesc)
	}

	if result.AccessToken == "" {
		return "", fmt.Errorf("no access_token (API key) in response")
	}

	return result.AccessToken, nil
}

// RefreshOpenAITokens refreshes an OAuth provider's tokens using the stored
// refresh_token. Returns the new API key and refresh token.
func RefreshOpenAITokens(refreshToken string) (apiKey, newRefreshToken string, err error) {
	slog.Info("OAuth: refreshing tokens")

	// Refresh request uses JSON body (different from initial exchange)
	payload, _ := json.Marshal(map[string]string{
		"client_id":     openaiClientID,
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openaiTokenURL,
		strings.NewReader(string(payload)))
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read response: %w", err)
	}

	slog.Info("OAuth: refresh response", "status", resp.StatusCode)

	var result tokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("parse response: %w", err)
	}

	if errStr := result.errorString(); errStr != "" {
		return "", "", fmt.Errorf("refresh error: %s — %s", errStr, result.ErrorDesc)
	}

	// Get the new refresh token (may be rotated)
	newRefreshToken = result.RefreshToken
	if newRefreshToken == "" {
		newRefreshToken = refreshToken // keep old if not rotated
	}

	// Exchange the new id_token for a fresh API key
	if result.IDToken == "" {
		return "", "", fmt.Errorf("no id_token in refresh response")
	}

	apiKey, err = exchangeIDTokenForAPIKey(result.IDToken)
	if err != nil {
		return "", "", fmt.Errorf("API key exchange after refresh: %w", err)
	}

	slog.Info("OAuth: tokens refreshed successfully")
	return apiKey, newRefreshToken, nil
}

// startOpenAIOAuthAsync runs the OAuth flow in a background goroutine.
func startOpenAIOAuthAsync() {
	oauth.mu.Lock()
	if oauth.running {
		oauth.mu.Unlock()
		return
	}
	oauth.running = true
	oauth.apiKey = ""
	oauth.refreshToken = ""
	oauth.errMsg = ""
	oauth.mu.Unlock()

	go func() {
		apiKey, refreshToken, err := runOpenAIOAuthFlow()

		oauth.mu.Lock()
		defer oauth.mu.Unlock()
		oauth.running = false
		if err != nil {
			oauth.errMsg = err.Error()
			slog.Error("OAuth flow failed", "error", err)
		} else {
			oauth.apiKey = apiKey
			oauth.refreshToken = refreshToken
			slog.Info("OAuth flow succeeded")
		}
	}()
}

// getOAuthResult returns the current state of the OAuth flow.
func getOAuthResult() string {
	oauth.mu.Lock()
	defer oauth.mu.Unlock()
	if oauth.running {
		return "pending"
	}
	if oauth.errMsg != "" {
		msg := oauth.errMsg
		oauth.errMsg = ""
		return "error: " + msg
	}
	if oauth.apiKey != "" {
		result, _ := json.Marshal(map[string]string{
			"api_key":       oauth.apiKey,
			"refresh_token": oauth.refreshToken,
		})
		oauth.apiKey = ""
		oauth.refreshToken = ""
		return "ok:" + string(result)
	}
	return "error: no OAuth flow started"
}

// runOpenAIOAuthFlow executes the full OpenAI OAuth PKCE flow.
func runOpenAIOAuthFlow() (apiKey, refreshToken string, err error) {
	// Try the preferred port first, then fallback ports, then random.
	var listener net.Listener
	var port int
	for _, p := range []int{oauthPort, 9004, 9005, 9006, 9007} {
		l, listenErr := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if listenErr == nil {
			listener = l
			port = p
			break
		}
		slog.Warn("OAuth: port unavailable", "port", p, "error", listenErr)
	}
	if listener == nil {
		// Fallback: random port
		l, listenErr := net.Listen("tcp", "127.0.0.1:0")
		if listenErr != nil {
			return "", "", fmt.Errorf("no available port for OAuth callback: %w", listenErr)
		}
		listener = l
		port = l.Addr().(*net.TCPAddr).Port
		slog.Info("OAuth: using random fallback port", "port", port)
	}
	redirectURI := fmt.Sprintf("http://localhost:%d%s", port, oauthPath)

	slog.Info("OAuth: starting OpenAI flow", "redirect", redirectURI)

	verifier, challenge := generatePKCE()
	state := randomState()

	type callbackResult struct {
		code string
		err  error
	}
	resultCh := make(chan callbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(oauthPath, func(w http.ResponseWriter, r *http.Request) {
		slog.Info("OAuth: callback received", "query_keys", len(r.URL.Query()))

		// Verify state
		if gotState := r.URL.Query().Get("state"); gotState != state {
			slog.Error("OAuth: state mismatch", "expected_len", len(state), "got_len", len(gotState))
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(400)
			fmt.Fprint(w, oauthPage(false, "Security error: state mismatch. Please try again."))
			resultCh <- callbackResult{err: fmt.Errorf("state mismatch")}
			return
		}

		// Check for error
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			desc := r.URL.Query().Get("error_description")
			if desc == "" {
				desc = errMsg
			}
			slog.Error("OAuth: error from provider", "error", errMsg, "desc", desc)
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(400)
			fmt.Fprint(w, oauthPage(false, desc))
			resultCh <- callbackResult{err: fmt.Errorf("%s", desc)}
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			slog.Error("OAuth: no code in callback")
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(400)
			fmt.Fprint(w, oauthPage(false, "No authorization code received."))
			resultCh <- callbackResult{err: fmt.Errorf("no code in callback")}
			return
		}

		slog.Info("OAuth: received authorization code", "code_len", len(code))
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, oauthPage(true, ""))
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

	// Build authorization URL
	params := url.Values{
		"response_type":       {"code"},
		"client_id":           {openaiClientID},
		"redirect_uri":        {redirectURI},
		"scope":               {openaiScopes},
		"code_challenge":      {challenge},
		"code_challenge_method": {"S256"},
		"state":               {state},
	}
	authURL := openaiAuthURL + "?" + params.Encode()

	if err := openBrowserURL(authURL); err != nil {
		return "", "", fmt.Errorf("failed to open browser: %w", err)
	}

	slog.Info("OAuth: browser opened, waiting for callback...")

	// Wait for callback (5 minute timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	select {
	case res := <-resultCh:
		if res.err != nil {
			return "", "", res.err
		}

		// Step 1: Exchange code for tokens
		tokens, err := exchangeCodeForTokens(res.code, verifier, redirectURI)
		if err != nil {
			return "", "", err
		}

		// Step 2: Exchange id_token for API key
		apiKey, err := exchangeIDTokenForAPIKey(tokens.IDToken)
		if err != nil {
			return "", "", err
		}

		prefix := apiKey
		if len(prefix) > 12 {
			prefix = prefix[:12]
		}
		slog.Info("OAuth: complete", "key_prefix", prefix+"...", "has_refresh", tokens.RefreshToken != "")

		return apiKey, tokens.RefreshToken, nil

	case <-ctx.Done():
		return "", "", fmt.Errorf("timed out waiting for authorization (5 minutes)")
	}
}

func oauthPage(success bool, errMsg string) string {
	if success {
		return `<!DOCTYPE html>
<html><head><style>
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;background:#1e1e2e;color:#cdd6f4;display:flex;align-items:center;justify-content:center;height:100vh;margin:0}
.box{text-align:center;padding:40px;background:#313244;border-radius:12px;max-width:440px}
.logos{display:flex;align-items:center;justify-content:center;gap:16px;margin-bottom:20px}
.logo-circle{width:64px;height:64px;border-radius:50%;display:flex;align-items:center;justify-content:center}
.logo-ghost{background:#2a2d42;border:2px solid #a6e3a1}
.logo-oai{background:#2a2d42;border:2px solid #45475a}
.arrow{color:#a6e3a1;font-size:1.5em;animation:pulse 1.5s ease-in-out infinite}
@keyframes pulse{0%,100%{opacity:.5;transform:scale(1)}50%{opacity:1;transform:scale(1.2)}}
h2{color:#a6e3a1;margin-bottom:12px}
p{color:#a6adc8;font-size:0.9em}
</style></head><body>
<div class="box">
<div class="logos">
<div class="logo-circle logo-oai">
<svg width="40" height="40" viewBox="0 0 24 24"><path d="M22.28 9.82a5.98 5.98 0 0 0-.52-4.91 6.05 6.05 0 0 0-6.51-2.9A6.07 6.07 0 0 0 4.98 4.18a5.98 5.98 0 0 0-4 2.9 6.05 6.05 0 0 0 .74 7.1 5.98 5.98 0 0 0 .51 4.91 6.05 6.05 0 0 0 6.51 2.9A5.98 5.98 0 0 0 13.26 24a6.06 6.06 0 0 0 5.77-4.21 5.99 5.99 0 0 0 4-2.9 6.06 6.06 0 0 0-.75-7.07z" fill="#cdd6f4"/></svg>
</div>
<span class="arrow">&#10132;</span>
<div class="logo-circle logo-ghost">
<img src="data:image/png;base64,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" width="44" height="44" style="border-radius:50%" alt="GhostSpell">
</div>
</div>
<h2>&#9989; Authorization Successful!</h2>
<p>ChatGPT is now connected to GhostSpell.<br>You can close this tab.</p>
</div>
<script>setTimeout(function(){ window.close(); }, 3000);</script>
</body></html>`
	}
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
</body></html>`, errMsg)
}
