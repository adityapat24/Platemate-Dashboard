package platform

import (
	"context"
	"time"
)

// ActionType identifies the kind of agentic action being executed.
type ActionType string

const (
	ActionRemoveDish  ActionType = "REMOVE_DISH"
	ActionRestoreDish ActionType = "RESTORE_DISH"
	ActionChangePrice ActionType = "CHANGE_PRICE"
	ActionRunPromo    ActionType = "RUN_PROMO"
	ActionRevokePromo ActionType = "REVOKE_PROMO"
)

// PromoType maps to Uber Eats promo_type field.
type PromoType string

const (
	PromoFlatOff          PromoType = "FLATOFF"
	PromoPercentOff       PromoType = "PERCENTOFF"
	PromoBOGO             PromoType = "BOGO"
	PromoFreeItemMinBasket PromoType = "FREEITEM_MINBASKET"
	PromoMenuItemDiscount  PromoType = "MENU_ITEM_DISCOUNT"
	PromoFreeDelivery     PromoType = "FREEDELIVERY"
)

// PromoConfig is the platform-agnostic promo configuration the dashboard sends.
type PromoConfig struct {
	PromoType           PromoType
	DiscountPercent     int    // for PERCENTOFF / MENU_ITEM_DISCOUNT
	DiscountAmountCents int64  // for FLATOFF (smallest currency unit)
	ItemIDs             []string
	UserGroup           string // "ALL_CUSTOMERS" or "FIRST_TIME_CUSTOMERS"
	StartTime           time.Time
	EndTime             time.Time
	BudgetCents         int64
	UnlimitedBudget     bool
	ExternalPromoID     string
}

// DishMappingResult is returned from SyncMenu — a lightweight item description
// with just enough info for entity resolution.
type DishMappingResult struct {
	PlatformItemID  string
	PlatformStoreID string
	CanonicalName   string
	PriceCents      int64
	Active          bool
	Suspended       bool
	Category        string
}

// ActionResult is the per-platform outcome of an executed action.
type ActionResult struct {
	Platform string
	Status   string // "success", "failed", "manual"
	Message  string
	Metadata map[string]interface{}
}

// ManualStep is a single instruction for a platform that can't be automated.
type ManualStep struct {
	Step        int
	Description string
	URL         string
}

// ManualChecklist is returned for platforms that require operator intervention.
type ManualChecklist struct {
	Platform string
	Steps    []ManualStep
}

// PlatformAdapter is the interface every integration must implement.
// The execution engine never imports platform-specific code — only this interface.
type PlatformAdapter interface {
	// Name returns the canonical platform identifier ("uber_eats", "toast", etc.).
	Name() string

	// CanAutomate reports whether the given action can be performed automatically.
	CanAutomate(actionType ActionType) bool

	// SyncMenu pulls the current menu from the platform and returns a slice of
	// lightweight DishMappingResult structs for entity-resolution downstream.
	SyncMenu(ctx context.Context, restaurantID string) ([]DishMappingResult, error)

	// GetPreChangeState captures the current state of a dish so it can be
	// restored during a rollback.  Must be called before any write operation.
	GetPreChangeState(ctx context.Context, dishMappingID string, actionType ActionType) (map[string]interface{}, error)

	// RemoveDish suspends / marks the dish out-of-stock on this platform.
	RemoveDish(ctx context.Context, dishMappingID string) (*ActionResult, error)

	// RestoreDish reverses a RemoveDish.
	RestoreDish(ctx context.Context, dishMappingID string) (*ActionResult, error)

	// ChangePrice updates the dish price on this platform.
	// newPriceCents is always in the platform's smallest currency unit (cents).
	ChangePrice(ctx context.Context, dishMappingID string, newPriceCents int64) (*ActionResult, error)

	// RunPromo creates a promotion on this platform.
	RunPromo(ctx context.Context, dishMappingID string, config PromoConfig) (*ActionResult, error)

	// RevokePromo cancels a running promotion.
	RevokePromo(ctx context.Context, promoID string) (*ActionResult, error)

	// Rollback reverses the last write operation using the stored pre-change state.
	Rollback(ctx context.Context, actionID string, preChangeState map[string]interface{}) (*ActionResult, error)

	// GetManualSteps returns a checklist for platforms that require manual action.
	// Returns nil if the action is fully automated.
	GetManualSteps(actionType ActionType, dishMappingID string, params map[string]interface{}) *ManualChecklist
}
