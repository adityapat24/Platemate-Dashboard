package mock

import (
	"context"
	"fmt"
	"time"

	"github.com/adityapat24/platemate-agentic/internal/platform"
)

// Adapter is a mock PlatformAdapter that returns realistic fake data.
// Used for local development without live API credentials.
type Adapter struct{}

func New() *Adapter { return &Adapter{} }

func (a *Adapter) Name() string { return "mock" }

func (a *Adapter) CanAutomate(_ platform.ActionType) bool { return true }

func (a *Adapter) SyncMenu(_ context.Context, _ string) ([]platform.DishMappingResult, error) {
	return []platform.DishMappingResult{
		{PlatformItemID: "CHKN_PRM", CanonicalName: "Chicken Parmesan", PriceCents: 1699, Active: true, Category: "Entrees"},
		{PlatformItemID: "CAESAR_S", CanonicalName: "Caesar Salad", PriceCents: 1299, Active: true, Category: "Salads"},
		{PlatformItemID: "LOBSTER_R", CanonicalName: "Lobster Roll", PriceCents: 2499, Active: true, Category: "Entrees"},
		{PlatformItemID: "FISH_CHIPS", CanonicalName: "Fish & Chips", PriceCents: 1599, Active: true, Category: "Entrees"},
		{PlatformItemID: "TIRAMISU", CanonicalName: "Tiramisu", PriceCents: 899, Active: true, Category: "Desserts"},
	}, nil
}

func (a *Adapter) GetPreChangeState(_ context.Context, dishMappingID string, actionType platform.ActionType) (map[string]interface{}, error) {
	return map[string]interface{}{
		"dish_mapping_id": dishMappingID,
		"action_type":     string(actionType),
		"snapshot_time":   time.Now().Unix(),
		"price_cents":     1699,
		"suspended":       false,
	}, nil
}

func (a *Adapter) RemoveDish(_ context.Context, dishMappingID string) (*platform.ActionResult, error) {
	return &platform.ActionResult{
		Platform: "mock",
		Status:   "success",
		Message:  fmt.Sprintf("Mock: dish %s suspended", dishMappingID),
	}, nil
}

func (a *Adapter) RestoreDish(_ context.Context, dishMappingID string) (*platform.ActionResult, error) {
	return &platform.ActionResult{
		Platform: "mock",
		Status:   "success",
		Message:  fmt.Sprintf("Mock: dish %s restored", dishMappingID),
	}, nil
}

func (a *Adapter) ChangePrice(_ context.Context, dishMappingID string, newPriceCents int64) (*platform.ActionResult, error) {
	return &platform.ActionResult{
		Platform: "mock",
		Status:   "success",
		Message:  fmt.Sprintf("Mock: dish %s price changed to %d cents", dishMappingID, newPriceCents),
		Metadata: map[string]interface{}{"new_price_cents": newPriceCents},
	}, nil
}

func (a *Adapter) RunPromo(_ context.Context, dishMappingID string, config platform.PromoConfig) (*platform.ActionResult, error) {
	return &platform.ActionResult{
		Platform: "mock",
		Status:   "success",
		Message:  fmt.Sprintf("Mock: promo %s created for dish %s", config.PromoType, dishMappingID),
		Metadata: map[string]interface{}{"promo_id": "mock-promo-001"},
	}, nil
}

func (a *Adapter) RevokePromo(_ context.Context, promoID string) (*platform.ActionResult, error) {
	return &platform.ActionResult{
		Platform: "mock",
		Status:   "success",
		Message:  fmt.Sprintf("Mock: promo %s revoked", promoID),
	}, nil
}

func (a *Adapter) Rollback(_ context.Context, actionID string, _ map[string]interface{}) (*platform.ActionResult, error) {
	return &platform.ActionResult{
		Platform: "mock",
		Status:   "success",
		Message:  fmt.Sprintf("Mock: action %s rolled back", actionID),
	}, nil
}

func (a *Adapter) GetManualSteps(_ platform.ActionType, _ string, _ map[string]interface{}) *platform.ManualChecklist {
	return nil
}
