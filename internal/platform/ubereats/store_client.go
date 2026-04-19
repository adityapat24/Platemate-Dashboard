package ubereats

import (
	"fmt"
	"net/http"
)

// StoreClient wraps the Uber Eats Store API.
type StoreClient struct {
	baseClient
}

func NewStoreClient(tm *TokenManager, apiBase string) *StoreClient {
	return &StoreClient{newBaseClient(tm, apiBase)}
}

// ListStores returns all stores linked to PlateMate via client-credentials token.
func (c *StoreClient) ListStores(offset, limit int) (*StoreListResponse, error) {
	path := fmt.Sprintf("/v1/delivery/stores?offset=%d&limit=%d", offset, limit)
	var resp StoreListResponse
	if err := c.do(http.MethodGet, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("list stores: %w", err)
	}
	return &resp, nil
}

// GetStore fetches a single store's full details.
func (c *StoreClient) GetStore(storeID string) (*Store, error) {
	path := fmt.Sprintf("/v1/delivery/store/%s?expand=holiday_hours,internal_contact_emails", storeID)
	var resp Store
	if err := c.do(http.MethodGet, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("get store %s: %w", storeID, err)
	}
	return &resp, nil
}

// GetStoreStatus returns whether the store is online or offline with reason.
func (c *StoreClient) GetStoreStatus(storeID string) (*StoreStatusResponse, error) {
	path := fmt.Sprintf("/v1/delivery/store/%s/status", storeID)
	var resp StoreStatusResponse
	if err := c.do(http.MethodGet, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("get store status %s: %w", storeID, err)
	}
	return &resp, nil
}

// SetStoreStatus puts the store ONLINE or OFFLINE.
func (c *StoreClient) SetStoreStatus(storeID, status, reason, isOfflineUntil string) error {
	req := UpdateStoreStatusRequest{
		Status:         status,
		Reason:         reason,
		IsOfflineUntil: isOfflineUntil,
	}
	path := fmt.Sprintf("/v1/delivery/store/%s/update-store-status", storeID)
	if err := c.do(http.MethodPost, path, req, nil); err != nil {
		return fmt.Errorf("set store status %s: %w", storeID, err)
	}
	return nil
}

// UpdatePrepTime sets the default prep time in seconds (max 10800).
func (c *StoreClient) UpdatePrepTime(storeID string, seconds int) error {
	req := UpdatePrepTimeRequest{DefaultPrepTime: seconds}
	path := fmt.Sprintf("/v1/delivery/store/%s/update-store-prep-time", storeID)
	if err := c.do(http.MethodPost, path, req, nil); err != nil {
		return fmt.Errorf("update prep time %s: %w", storeID, err)
	}
	return nil
}

// SetBusyMode adds a temporary delay on top of the default prep time.
func (c *StoreClient) SetBusyMode(storeID, delayUntil string, delayDurationSecs int) error {
	req := UpdatePrepTimeRequest{
		DelayConfig: &DelayConfig{
			DelayUntil:    delayUntil,
			DelayDuration: delayDurationSecs,
		},
	}
	path := fmt.Sprintf("/v1/delivery/store/%s/update-store-prep-time", storeID)
	if err := c.do(http.MethodPost, path, req, nil); err != nil {
		return fmt.Errorf("set busy mode %s: %w", storeID, err)
	}
	return nil
}

// ── Onboarding endpoints (use auth-code token) ────────────────────────────────

// ListMerchantStores returns the stores owned by the authorizing merchant.
// Requires an authorization-code token, not a client-credentials token.
func (c *StoreClient) ListMerchantStores(userToken string) (*StoresResponse, error) {
	var resp StoresResponse
	if err := c.doWithAuthCodeToken(http.MethodGet, "/v1/eats/stores", nil, &resp, userToken); err != nil {
		return nil, fmt.Errorf("list merchant stores: %w", err)
	}
	return &resp, nil
}

// ProvisionStore activates PlateMate's integration on a specific store.
// After this call, the client-credentials token has perpetual access to the store.
func (c *StoreClient) ProvisionStore(storeID, userToken, webhookURL string) error {
	req := PosProvisionRequest{WebhookURL: webhookURL}
	path := fmt.Sprintf("/v1/eats/stores/%s/pos_data", storeID)
	if err := c.doWithAuthCodeToken(http.MethodPost, path, req, nil, userToken); err != nil {
		return fmt.Errorf("provision store %s: %w", storeID, err)
	}
	return nil
}

// DeprovisionStore permanently removes PlateMate's integration from a store.
func (c *StoreClient) DeprovisionStore(storeID, userToken string) error {
	path := fmt.Sprintf("/v1/eats/stores/%s/pos_data", storeID)
	if err := c.doWithAuthCodeToken(http.MethodDelete, path, nil, nil, userToken); err != nil {
		return fmt.Errorf("deprovision store %s: %w", storeID, err)
	}
	return nil
}
