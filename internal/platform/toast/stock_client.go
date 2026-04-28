package toast

import (
	"fmt"
	"net/http"
)

// StockClient wraps the Toast Stock API.
type StockClient struct {
	baseClient
}

func NewStockClient(tm *TokenManager, apiBase string) *StockClient {
	return &StockClient{newBaseClient(tm, apiBase)}
}

// GetInventory returns in-stock / out-of-stock / quantity status for all items
// in the restaurant. The returned map is keyed by menuItemGuid for O(1) lookup.
func (c *StockClient) GetInventory(restaurantGUID string) (map[string]StockItem, error) {
	var resp StockInventoryResponse
	if err := c.do(http.MethodGet, "/stock/v1/inventory", restaurantGUID, nil, &resp); err != nil {
		return nil, fmt.Errorf("get inventory for %s: %w", restaurantGUID, err)
	}

	byGUID := make(map[string]StockItem, len(resp.StockDataList))
	for _, s := range resp.StockDataList {
		byGUID[s.MenuItemGUID] = s
	}
	return byGUID, nil
}
