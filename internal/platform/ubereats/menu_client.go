package ubereats

import (
	"fmt"
	"net/http"
	"time"
)

// MenuClient wraps the Uber Eats Menu API.
type MenuClient struct {
	baseClient
}

func NewMenuClient(tm *TokenManager, apiBase string) *MenuClient {
	return &MenuClient{newBaseClient(tm, apiBase)}
}

// GetMenu fetches the full menu for a store.
// menuType can be "DELIVERY", "PICK_UP", "DINE_IN" or empty for default.
func (c *MenuClient) GetMenu(storeID, menuType string) (*MenuConfiguration, error) {
	path := fmt.Sprintf("/v2/eats/stores/%s/menus", storeID)
	if menuType != "" {
		path += "?menu_type=" + menuType
	}
	var resp MenuConfiguration
	if err := c.do(http.MethodGet, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("get menu %s: %w", storeID, err)
	}
	return &resp, nil
}

// UpdateItem performs a sparse update on a single menu item.
// Only fields set in the request will be changed.
// Returns 204 No Content on success (no body).
func (c *MenuClient) UpdateItem(storeID, itemID string, update ItemUpdateRequest) error {
	path := fmt.Sprintf("/v2/eats/stores/%s/menus/items/%s", storeID, itemID)
	if err := c.do(http.MethodPost, path, update, nil); err != nil {
		return fmt.Errorf("update item %s/%s: %w", storeID, itemID, err)
	}
	return nil
}

// UploadFullMenu replaces the entire menu for a store.
// WARNING: Any entity not present in the new payload will be DELETED.
// Use UpdateItem for individual changes.
func (c *MenuClient) UploadFullMenu(storeID string, menu MenuConfiguration) error {
	path := fmt.Sprintf("/v2/eats/stores/%s/menus", storeID)
	if err := c.do(http.MethodPut, path, menu, nil); err != nil {
		return fmt.Errorf("upload full menu %s: %w", storeID, err)
	}
	return nil
}

// ── Convenience helpers ───────────────────────────────────────────────────────

// ChangeItemPrice updates a single item's price.
// newPriceCents must be in cents (e.g. 1299 for $12.99).
func (c *MenuClient) ChangeItemPrice(storeID, itemID string, newPriceCents int64) error {
	return c.UpdateItem(storeID, itemID, ItemUpdateRequest{
		PriceInfo: &PriceInfo{Price: newPriceCents},
	})
}

// SuspendItem marks an item as sold-out until the given Unix timestamp.
// Pass a timestamp far in the future to suspend indefinitely.
func (c *MenuClient) SuspendItem(storeID, itemID string, suspendUntil int64, reason string) error {
	if suspendUntil == 0 {
		// Default: 1 year from now
		suspendUntil = time.Now().Add(365 * 24 * time.Hour).Unix()
	}
	return c.UpdateItem(storeID, itemID, ItemUpdateRequest{
		SuspensionInfo: &SuspensionInfo{
			Suspension: &Suspension{
				SuspendUntil: suspendUntil,
				Reason:       reason,
			},
		},
	})
}

// RestoreItem clears any active suspension, making the item available again.
func (c *MenuClient) RestoreItem(storeID, itemID string) error {
	return c.UpdateItem(storeID, itemID, ItemUpdateRequest{
		SuspensionInfo: &SuspensionInfo{
			Suspension: nil,
		},
	})
}

// GetItemByID returns a single item from the menu by its external ID.
// The Uber Eats API doesn't have a single-item GET endpoint, so this fetches
// the full menu and searches for the item.
func (c *MenuClient) GetItemByID(storeID, itemID string) (*MenuItem, error) {
	menu, err := c.GetMenu(storeID, "")
	if err != nil {
		return nil, err
	}
	for _, item := range menu.Items {
		if item.ID == itemID {
			return &item, nil
		}
	}
	return nil, fmt.Errorf("item %s not found in store %s", itemID, storeID)
}
