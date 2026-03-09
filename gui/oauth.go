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
	IDToken      string `json:"id_token"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
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

	if result.Error != "" {
		return nil, fmt.Errorf("token error: %s — %s", result.Error, result.ErrorDesc)
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

	if result.Error != "" {
		return "", fmt.Errorf("API key exchange error: %s — %s", result.Error, result.ErrorDesc)
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

	if result.Error != "" {
		return "", "", fmt.Errorf("refresh error: %s — %s", result.Error, result.ErrorDesc)
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
	addr := fmt.Sprintf("127.0.0.1:%d", oauthPort)
	redirectURI := fmt.Sprintf("http://localhost:%d%s", oauthPort, oauthPath)

	slog.Info("OAuth: starting OpenAI flow", "redirect", redirectURI)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Warn("OAuth: port unavailable, trying random", "port", oauthPort, "error", err)
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return "", "", fmt.Errorf("start listener: %w", err)
		}
		actualPort := listener.Addr().(*net.TCPAddr).Port
		redirectURI = fmt.Sprintf("http://localhost:%d%s", actualPort, oauthPath)
		slog.Info("OAuth: using fallback port", "port", actualPort)
	}

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
