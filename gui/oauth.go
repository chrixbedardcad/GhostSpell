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
	"time"
)

// oauthResult is the response from OpenRouter's key exchange endpoint.
type oauthResult struct {
	Key    string `json:"key"`
	UserID string `json:"user_id"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

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

// openBrowser opens a URL in the user's default browser.
func openBrowser(rawURL string) error {
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

	slog.Debug("OpenRouter key exchange response", "status", resp.StatusCode, "body", string(respBody))

	var result oauthResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("OpenRouter error %d: %s", result.Error.Code, result.Error.Message)
	}

	if result.Key == "" {
		return "", fmt.Errorf("empty key in response (HTTP %d)", resp.StatusCode)
	}

	return result.Key, nil
}

// startOpenRouterOAuth runs the full OAuth PKCE flow:
//  1. Starts a localhost HTTP server on a random port
//  2. Opens the user's browser to OpenRouter's auth page
//  3. Waits for the callback with the auth code
//  4. Exchanges the code for a permanent API key
//  5. Returns the API key or an error
//
// The flow times out after 5 minutes.
func startOpenRouterOAuth() (string, error) {
	// 1. Find a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("start listener: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	callbackURL := fmt.Sprintf("http://localhost:%d/callback", port)

	slog.Info("OAuth: starting flow", "callback", callbackURL)

	// 2. Generate PKCE
	verifier, challenge := generatePKCE()

	// 3. Channels for result
	type callbackResult struct {
		code string
		err  error
	}
	resultCh := make(chan callbackResult, 1)

	// 4. HTTP server to receive the callback
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			slog.Error("OAuth callback: no code parameter")
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(400)
			fmt.Fprint(w, oauthErrorPage("No authorization code received. Please try again."))
			resultCh <- callbackResult{err: fmt.Errorf("no code in callback")}
			return
		}
		slog.Info("OAuth callback: received code", "code_len", len(code))
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, oauthSuccessPage())
		resultCh <- callbackResult{code: code}
	})

	server := &http.Server{Handler: mux}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			slog.Error("OAuth server error", "error", err)
		}
	}()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	// 5. Open browser
	authURL := fmt.Sprintf("https://openrouter.ai/auth?callback_url=%s&code_challenge=%s&code_challenge_method=S256",
		url.QueryEscape(callbackURL), url.QueryEscape(challenge))

	if err := openBrowser(authURL); err != nil {
		return "", fmt.Errorf("open browser: %w", err)
	}

	slog.Info("OAuth: browser opened, waiting for callback...")

	// 6. Wait for callback (5 minute timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	select {
	case res := <-resultCh:
		if res.err != nil {
			return "", res.err
		}
		// 7. Exchange code for key
		slog.Info("OAuth: exchanging code for API key...")
		key, err := exchangeCodeForKey(res.code, verifier)
		if err != nil {
			return "", err
		}
		slog.Info("OAuth: API key received", "key_prefix", key[:min(12, len(key))]+"...")
		return key, nil
	case <-ctx.Done():
		return "", fmt.Errorf("OAuth flow timed out (5 minutes)")
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
