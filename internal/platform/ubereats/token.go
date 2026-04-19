package ubereats

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const tokenRefreshBuffer = 60 * time.Second

// tokenResponse is the raw JSON from Uber's /oauth/v2/token endpoint.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// cachedToken holds a token and its expiry.
type cachedToken struct {
	value     string
	expiresAt time.Time
}

func (t *cachedToken) isValid() bool {
	return t.value != "" && time.Now().Before(t.expiresAt.Add(-tokenRefreshBuffer))
}

// TokenManager handles both Uber Eats OAuth flows.
// Client-credentials tokens are cached and auto-refreshed.
// Authorization-code tokens are per-session and stored in memory by stateKey.
type TokenManager struct {
	clientID     string
	clientSecret string
	redirectURI  string
	authBase     string

	mu    sync.RWMutex
	cc    *cachedToken            // client-credentials token (shared)
	codes map[string]*cachedToken // authorization-code tokens keyed by stateKey
}

func NewTokenManager(clientID, clientSecret, redirectURI, authBase string) *TokenManager {
	return &TokenManager{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		authBase:     authBase,
		codes:        make(map[string]*cachedToken),
	}
}

// GetClientCredentialsToken returns a valid client-credentials token,
// fetching a new one if the cached token is missing or near-expiry.
func (tm *TokenManager) GetClientCredentialsToken() (string, error) {
	tm.mu.RLock()
	if tm.cc != nil && tm.cc.isValid() {
		tok := tm.cc.value
		tm.mu.RUnlock()
		return tok, nil
	}
	tm.mu.RUnlock()

	return tm.refreshClientCredentials()
}

func (tm *TokenManager) refreshClientCredentials() (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Double-check after acquiring write lock.
	if tm.cc != nil && tm.cc.isValid() {
		return tm.cc.value, nil
	}

	scopes := strings.Join([]string{
		"eats.store",
		"eats.order",
		"eats.report",
		"eats.store.status.write",
		"eats.store.orders.read",
		"eats.store.promotion.write",
		"eats.store.promotion.read",
	}, " ")

	tok, expiresIn, err := tm.fetchToken(url.Values{
		"client_id":     {tm.clientID},
		"client_secret": {tm.clientSecret},
		"grant_type":    {"client_credentials"},
		"scope":         {scopes},
	})
	if err != nil {
		return "", fmt.Errorf("refresh client credentials: %w", err)
	}

	tm.cc = &cachedToken{
		value:     tok,
		expiresAt: time.Now().Add(time.Duration(expiresIn) * time.Second),
	}
	return tok, nil
}

// GenerateAuthURL returns the OAuth redirect URL for the merchant onboarding flow.
// state should be a cryptographically random string to prevent CSRF.
func (tm *TokenManager) GenerateAuthURL(state string) string {
	// Use production auth URL for authorization code flow
	authBase := strings.Replace(tm.authBase, "sandbox-login", "auth", 1)
	// If already production, keep as-is
	if !strings.Contains(authBase, "sandbox") {
		authBase = "https://auth.uber.com"
	}

	params := url.Values{
		"client_id":     {tm.clientID},
		"response_type": {"code"},
		"redirect_uri":  {tm.redirectURI},
		"scope":         {"eats.pos_provisioning"},
		"state":         {state},
	}
	return fmt.Sprintf("%s/oauth/v2/authorize?%s", authBase, params.Encode())
}

// ExchangeCodeForToken exchanges an authorization code for a user access token.
// The returned token is valid only for store discovery and provisioning.
func (tm *TokenManager) ExchangeCodeForToken(code, stateKey string) (string, error) {
	tok, expiresIn, err := tm.fetchToken(url.Values{
		"client_id":     {tm.clientID},
		"client_secret": {tm.clientSecret},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {tm.redirectURI},
		"code":          {code},
	})
	if err != nil {
		return "", fmt.Errorf("exchange code: %w", err)
	}

	tm.mu.Lock()
	tm.codes[stateKey] = &cachedToken{
		value:     tok,
		expiresAt: time.Now().Add(time.Duration(expiresIn) * time.Second),
	}
	tm.mu.Unlock()

	return tok, nil
}

// GetAuthCodeToken retrieves a previously-exchanged authorization-code token.
func (tm *TokenManager) GetAuthCodeToken(stateKey string) (string, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	t, ok := tm.codes[stateKey]
	if !ok || !t.isValid() {
		return "", false
	}
	return t.value, true
}

// DeleteAuthCodeToken removes a stored auth-code token after provisioning is done.
func (tm *TokenManager) DeleteAuthCodeToken(stateKey string) {
	tm.mu.Lock()
	delete(tm.codes, stateKey)
	tm.mu.Unlock()
}

// fetchToken is the low-level token request against Uber's /oauth/v2/token endpoint.
func (tm *TokenManager) fetchToken(vals url.Values) (token string, expiresIn int64, err error) {
	endpoint := tm.authBase + "/oauth/v2/token"
	resp, err := http.PostForm(endpoint, vals)
	if err != nil {
		return "", 0, fmt.Errorf("POST %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("token endpoint %d: %s", resp.StatusCode, string(body))
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", 0, fmt.Errorf("decode token response: %w", err)
	}
	return tr.AccessToken, tr.ExpiresIn, nil
}
