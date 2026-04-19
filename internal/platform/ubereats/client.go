package ubereats

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// APIError is a structured error from the Uber Eats API.
type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("uber eats API %d: %s", e.StatusCode, e.Message)
}

// baseClient holds the shared HTTP client and token manager used by all clients.
type baseClient struct {
	httpClient   *http.Client
	tokenManager *TokenManager
	apiBase      string
}

func newBaseClient(tm *TokenManager, apiBase string) baseClient {
	return baseClient{
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		tokenManager: tm,
		apiBase:      apiBase,
	}
}

// do executes an authenticated request, retrying once on 401.
func (c *baseClient) do(method, path string, body interface{}, out interface{}) error {
	return c.doWithToken(method, path, body, out, false)
}

func (c *baseClient) doWithToken(method, path string, body interface{}, out interface{}, retried bool) error {
	token, err := c.tokenManager.GetClientCredentialsToken()
	if err != nil {
		return fmt.Errorf("get token: %w", err)
	}

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	fullURL := c.apiBase + path
	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s %s: %w", method, fullURL, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[ubereats] %s %s → %d (%dms)", method, path, resp.StatusCode, time.Since(start).Milliseconds())

	if resp.StatusCode == http.StatusUnauthorized && !retried {
		// Force token refresh and retry once.
		c.tokenManager.mu.Lock()
		c.tokenManager.cc = nil
		c.tokenManager.mu.Unlock()
		return c.doWithToken(method, path, body, out, true)
	}

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	if resp.StatusCode >= 400 {
		var apiErr struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		}
		_ = json.Unmarshal(respBody, &apiErr)
		msg := apiErr.Message
		if msg == "" {
			msg = string(respBody)
		}
		return &APIError{StatusCode: resp.StatusCode, Message: msg, Body: string(respBody)}
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode response: %w (body: %s)", err, string(respBody))
		}
	}
	return nil
}

// doWithAuthCodeToken uses the per-session authorization-code token instead of
// the shared client-credentials token.  Used only for onboarding endpoints.
func (c *baseClient) doWithAuthCodeToken(method, path string, body interface{}, out interface{}, userToken string) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	fullURL := c.apiBase + path
	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[ubereats] %s %s → %d (auth-code)", method, path, resp.StatusCode)

	if resp.StatusCode >= 400 {
		return &APIError{StatusCode: resp.StatusCode, Message: string(respBody)}
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode: %w", err)
		}
	}
	return nil
}
