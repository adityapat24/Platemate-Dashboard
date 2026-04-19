package ubereats

import (
	"fmt"
	"net/http"
	"time"
)

// PromotionsClient wraps the Uber Eats Promotions API.
type PromotionsClient struct {
	baseClient
}

func NewPromotionsClient(tm *TokenManager, apiBase string) *PromotionsClient {
	return &PromotionsClient{newBaseClient(tm, apiBase)}
}

// CreatePromotion creates a new promotion on Uber Eats.
// Promotions cannot be modified after creation — use RevokePromotion + CreatePromotion to change.
func (c *PromotionsClient) CreatePromotion(storeID string, req CreatePromotionRequest) (*PromotionResponse, error) {
	path := fmt.Sprintf("/v1/delivery/stores/%s/promotion", storeID)
	var resp PromotionResponse
	if err := c.do(http.MethodPost, path, req, &resp); err != nil {
		return nil, fmt.Errorf("create promotion: %w", err)
	}
	return &resp, nil
}

// RevokePromotion cancels an active promotion immediately.
// Returns 204 No Content on success.
func (c *PromotionsClient) RevokePromotion(promotionID string) error {
	path := fmt.Sprintf("/v1/delivery/promotions/%s/revoke", promotionID)
	if err := c.do(http.MethodPost, path, nil, nil); err != nil {
		return fmt.Errorf("revoke promotion %s: %w", promotionID, err)
	}
	return nil
}

// GetPromotion fetches details of a specific promotion.
func (c *PromotionsClient) GetPromotion(promotionID string) (*PromotionResponse, error) {
	path := fmt.Sprintf("/v1/delivery/promotions/%s", promotionID)
	var resp PromotionResponse
	if err := c.do(http.MethodGet, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("get promotion %s: %w", promotionID, err)
	}
	return &resp, nil
}

// ListPromotions returns all promotions for a store, optionally filtered by state.
// state can be "active", "pending", "completed", "revoked", "expired" or empty for all.
func (c *PromotionsClient) ListPromotions(storeID, state string) (*PromotionListResponse, error) {
	path := fmt.Sprintf("/v1/delivery/stores/%s/promotions", storeID)
	if state != "" {
		path += "?state=" + state
	}
	var resp PromotionListResponse
	if err := c.do(http.MethodGet, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("list promotions %s: %w", storeID, err)
	}
	return &resp, nil
}

// ── Convenience builders ──────────────────────────────────────────────────────

// CreateMenuItemDiscount creates a MENU_ITEM_DISCOUNT promotion — the most common
// PlateMate use case (e.g. "20% off Chicken Parm for 7 days").
func (c *PromotionsClient) CreateMenuItemDiscount(
	storeID string,
	itemIDs []string,
	percentOff int,
	userGroup string,
	startTime, endTime time.Time,
	budgetCents int64,
	unlimitedBudget bool,
	externalID string,
) (*PromotionResponse, error) {
	discounts := make([]ItemDiscount, 0, len(itemIDs))
	for _, id := range itemIDs {
		discounts = append(discounts, ItemDiscount{
			Item: PromoItemRef{ItemExternalID: id},
			DiscountAmount: ItemDiscountAmt{
				PercentDiscount: &PercentDiscount{PercentValue: percentOff},
			},
		})
	}

	budget := PromoBudget{UnlimitedBudget: unlimitedBudget}
	if !unlimitedBudget && budgetCents > 0 {
		budget.UnlimitedBudget = false
		budget.PeriodicBudget = &PeriodicBudget{
			BudgetAmount: budgetCents,
			BudgetPeriod: "WEEKLY",
		}
	}

	req := CreatePromotionRequest{
		StartTime:           startTime.Format(time.RFC3339),
		EndTime:             endTime.Format(time.RFC3339),
		ExternalPromotionID: externalID,
		UserGroup:           userGroup,
		AllowUnlimitedApply: true,
		CurrencyCode:        "USD",
		Budget:              budget,
		PromoType:           "MENU_ITEM_DISCOUNT",
		PromotionDiscount: PromotionDiscount{
			MenuItemDiscount: &MenuItemDiscount{ItemDiscounts: discounts},
		},
	}
	return c.CreatePromotion(storeID, req)
}

// CreatePercentOff creates a store-wide PERCENTOFF promotion.
func (c *PromotionsClient) CreatePercentOff(
	storeID string,
	percentOff int,
	minSpendCents int64,
	maxDiscountCents int64,
	userGroup string,
	startTime, endTime time.Time,
	budgetCents int64,
	unlimitedBudget bool,
	externalID string,
) (*PromotionResponse, error) {
	discount := &PercentOffDiscount{PercentValue: percentOff}
	if minSpendCents > 0 {
		discount.MinBasketConstraint = &MinBasketConstraint{
			MinSpend: MoneyAmount{Amount: minSpendCents, CurrencyCode: "USD"},
		}
	}
	if maxDiscountCents > 0 {
		discount.MaxDiscountValue = &MoneyAmount{Amount: maxDiscountCents, CurrencyCode: "USD"}
	}

	budget := PromoBudget{UnlimitedBudget: unlimitedBudget}
	if !unlimitedBudget {
		budget.PeriodicBudget = &PeriodicBudget{
			BudgetAmount: budgetCents,
			BudgetPeriod: "WEEKLY",
		}
	}

	req := CreatePromotionRequest{
		StartTime:           startTime.Format(time.RFC3339),
		EndTime:             endTime.Format(time.RFC3339),
		ExternalPromotionID: externalID,
		UserGroup:           userGroup,
		AllowUnlimitedApply: true,
		CurrencyCode:        "USD",
		Budget:              budget,
		PromoType:           "PERCENTOFF",
		PromotionDiscount:   PromotionDiscount{PercentOffDiscount: discount},
	}
	return c.CreatePromotion(storeID, req)
}
