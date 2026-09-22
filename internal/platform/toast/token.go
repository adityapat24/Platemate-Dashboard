package toast

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const tokenRefreshBuffer = 60 * time.Second

// tokenRequest is the JSON body sent to Toast's authentication endpoint.
// Toast uses JSON (not form-encoding) and a custom userAccessType field.
type tokenRequest struct {
	ClientID       string `json:"clientId"`
	ClientSecret   string `json:"clientSecret"`
	UserAccessType string `json:"userAccessType"`
}

// tokenResponse is the raw JSON from Toast's /authentication/v1/authentication/login.
type tokenResponse struct {
	Status string `json:"status"`
	Token  struct {
		TokenType string `json:"tokenType"`
		Token     string `json:"token"`
		ExpiresIn int64  `json:"expiresIn"` // seconds
	} `json:"token"`
}

// cachedToken holds a token and its expiry.
type cachedToken struct {
	value     string
	expiresAt time.Time
}

func (t *cachedToken) isValid() bool {
	return t.value != "" && time.Now().Before(t.expiresAt.Add(-tokenRefreshBuffer))
}

// TokenManager handles Toast OAuth2 client_credentials flow.
// Tokens are cached and auto-refreshed before expiry.
// Toast uses a single flow (no authorization-code flow needed for POS read access).
type TokenManager struct {
	clientID     string
	clientSecret string
	authBase     string

	mu sync.RWMutex
	cc *cachedToken
}

func NewTokenManager(clientID, clientSecret, authBase string) *TokenManager {
	return &TokenManager{
		clientID:     clientID,
		clientSecret: clientSecret,
		authBase:     authBase,
	}
}

// GetToken returns a valid client-credentials token, refreshing if near-expiry.
func (tm *TokenManager) GetToken() (string, error) {
	tm.mu.RLock()
	if tm.cc != nil && tm.cc.isValid() {
		tok := tm.cc.value
		tm.mu.RUnlock()
		return tok, nil
	}
	tm.mu.RUnlock()

	return tm.refresh()
}

// GetTokenForCredentials fetches a token using the provided credentials without
// caching — used by the onboarding endpoint to validate restaurant credentials.
func (tm *TokenManager) GetTokenForCredentials(clientID, clientSecret string) (string, error) {
	_, tok, err := tm.fetchToken(clientID, clientSecret)
	return tok, err
}

func (tm *TokenManager) refresh() (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Double-check after acquiring write lock.
	if tm.cc != nil && tm.cc.isValid() {
		return tm.cc.value, nil
	}

	expiresIn, tok, err := tm.fetchToken(tm.clientID, tm.clientSecret)
	if err != nil {
		return "", fmt.Errorf("refresh token: %w", err)
	}

	tm.cc = &cachedToken{
		value:     tok,
		expiresAt: time.Now().Add(time.Duration(expiresIn) * time.Second),
	}
	return tok, nil
}

// InvalidateToken forces the next GetToken call to fetch a fresh token.
func (tm *TokenManager) InvalidateToken() {
	tm.mu.Lock()
	tm.cc = nil
	tm.mu.Unlock()
}

// fetchToken performs the raw HTTP call to Toast's authentication endpoint.
// Returns (expiresIn seconds, token string, error).
func (tm *TokenManager) fetchToken(clientID, clientSecret string) (int64, string, error) {
	endpoint := tm.authBase + "/authentication/v1/authentication/login"

	body, err := json.Marshal(tokenRequest{
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		UserAccessType: "TOAST_MACHINE_CLIENT",
	})
	if err != nil {
		return 0, "", fmt.Errorf("marshal token request: %w", err)
	}

	resp, err := http.Post(endpoint, "application/json", bytes.NewReader(body)) //nolint:noctx
	if err != nil {
		return 0, "", fmt.Errorf("POST %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, "", fmt.Errorf("token endpoint %d: %s", resp.StatusCode, string(respBody))
	}

	var tr tokenResponse
	if err := json.Unmarshal(respBody, &tr); err != nil {
		return 0, "", fmt.Errorf("decode token response: %w", err)
	}
	if tr.Token.Token == "" {
		return 0, "", fmt.Errorf("empty token in response: %s", string(respBody))
	}
	return tr.Token.ExpiresIn, tr.Token.Token, nil
}
