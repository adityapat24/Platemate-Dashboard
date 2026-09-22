package toast

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// APIError is a structured error from the Toast API.
type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("toast API %d: %s", e.StatusCode, e.Message)
}

// baseClient holds the shared HTTP client and token manager used by all sub-clients.
// Every request includes:
//   - Authorization: Bearer <token>
//   - Toast-Restaurant-External-ID: <restaurantGUID>  (for restaurant-scoped endpoints)
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

// do executes an authenticated request scoped to a restaurant, retrying once on 401.
// restaurantGUID must be the Toast external GUID for the restaurant.
// Pass an empty restaurantGUID only for non-restaurant-scoped endpoints.
func (c *baseClient) do(method, path, restaurantGUID string, body, out interface{}) error {
	return c.doWithRetry(method, path, restaurantGUID, body, out, false)
}

func (c *baseClient) doWithRetry(method, path, restaurantGUID string, body, out interface{}, retried bool) error {
	token, err := c.tokenManager.GetToken()
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
	if restaurantGUID != "" {
		req.Header.Set("Toast-Restaurant-External-ID", restaurantGUID)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s %s: %w", method, fullURL, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[toast] %s %s → %d (%dms)", method, path, resp.StatusCode, time.Since(start).Milliseconds())

	if resp.StatusCode == http.StatusUnauthorized && !retried {
		c.tokenManager.InvalidateToken()
		return c.doWithRetry(method, path, restaurantGUID, body, out, true)
	}

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	if resp.StatusCode == http.StatusAccepted {
		// Caller handles 202 (e.g., analytics polling).
		if out != nil && len(respBody) > 0 {
			_ = json.Unmarshal(respBody, out)
		}
		return &APIError{StatusCode: http.StatusAccepted, Body: string(respBody)}
	}

	if resp.StatusCode >= 400 {
		var apiErr struct {
			Message string `json:"message"`
			Code    string `json:"code"`
			Status  int    `json:"status"`
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

// isAccepted reports whether an error is a 202 Accepted from the Toast API.
func isAccepted(err error) bool {
	if err == nil {
		return false
	}
	apiErr, ok := err.(*APIError)
	return ok && apiErr.StatusCode == http.StatusAccepted
}
