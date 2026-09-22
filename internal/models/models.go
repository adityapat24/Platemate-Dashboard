package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ── Restaurant ──────────────────────────────────────────────────────────────

type Restaurant struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name      string             `bson:"name" json:"name"`
	APIKey    string             `bson:"api_key" json:"-"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

// ── Platform connection status ───────────────────────────────────────────────

const (
	ConnectionStatusConnected    = "connected"
	ConnectionStatusPending      = "pending"
	ConnectionStatusError        = "error"
	ConnectionStatusDisconnected = "disconnected"
)

type PlatformConnection struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	RestaurantID primitive.ObjectID `bson:"restaurant_id" json:"restaurant_id"`
	Platform     string             `bson:"platform" json:"platform"` // "uber_eats", "toast", "r365", "google"
	Status       string             `bson:"status" json:"status"`
	StoreID      string             `bson:"store_id,omitempty" json:"store_id,omitempty"`
	StoreName    string             `bson:"store_name,omitempty" json:"store_name,omitempty"`
	LastSync     time.Time          `bson:"last_sync" json:"last_sync"`
	ErrorMsg     string             `bson:"error,omitempty" json:"error,omitempty"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time          `bson:"updated_at" json:"updated_at"`
}

// ── Dish mapping (unified entity across platforms) ───────────────────────────

type UberEatsPlatformData struct {
	ItemID       string    `bson:"item_id" json:"item_id"`
	StoreID      string    `bson:"store_id" json:"store_id"`
	PriceCents   int64     `bson:"price_cents" json:"price_cents"`
	Active       bool      `bson:"active" json:"active"`
	Suspended    bool      `bson:"suspended" json:"suspended"`
	SuspendUntil int64     `bson:"suspend_until,omitempty" json:"suspend_until,omitempty"`
	LastSynced   time.Time `bson:"last_synced" json:"last_synced"`
}

type ToastPlatformData struct {
	GUID            string    `bson:"guid" json:"guid"`
	MultiLocationID string    `bson:"multi_location_id" json:"multi_location_id"`
	PriceCents      int64     `bson:"price_cents" json:"price_cents"`
	StockStatus     string    `bson:"stock_status" json:"stock_status"` // "IN_STOCK", "OUT_OF_STOCK"
	MenuGroup       string    `bson:"menu_group" json:"menu_group"`
	LastSynced      time.Time `bson:"last_synced" json:"last_synced"`

	// Analytics fields populated from Toast ERA (enterprise-metrics:read scope).
	WeeklyUnitsSold  int       `bson:"weekly_units_sold,omitempty" json:"weekly_units_sold,omitempty"`
	WeeklyNetSales   float64   `bson:"weekly_net_sales,omitempty" json:"weekly_net_sales,omitempty"`
	WeeklyAvgPrice   float64   `bson:"weekly_avg_price,omitempty" json:"weekly_avg_price,omitempty"`
	AnalyticsUpdated time.Time `bson:"analytics_updated,omitempty" json:"analytics_updated,omitempty"`
}

type R365PlatformData struct {
	MenuItemName  string    `bson:"menu_item_name" json:"menu_item_name"`
	IngredientIDs []string  `bson:"ingredient_ids" json:"ingredient_ids"`
	LastSynced    time.Time `bson:"last_synced" json:"last_synced"`
}

type GooglePlatformData struct {
	MentionCount  int       `bson:"mention_count" json:"mention_count"`
	AvgSentiment  float64   `bson:"avg_sentiment" json:"avg_sentiment"`
	CommonThemes  []string  `bson:"common_themes" json:"common_themes"`
	LastAnalyzed  time.Time `bson:"last_analyzed" json:"last_analyzed"`
}

type DishPlatformData struct {
	UberEats *UberEatsPlatformData `bson:"uber_eats,omitempty" json:"uber_eats,omitempty"`
	Toast    *ToastPlatformData    `bson:"toast,omitempty" json:"toast,omitempty"`
	R365     *R365PlatformData     `bson:"r365,omitempty" json:"r365,omitempty"`
	Google   *GooglePlatformData   `bson:"google,omitempty" json:"google,omitempty"`
}

type DishAnalytics struct {
	PlateMateReviewCount int       `bson:"platemate_review_count" json:"platemate_review_count"`
	AvgOverall           float64   `bson:"avg_overall" json:"avg_overall"`
	AvgTaste             float64   `bson:"avg_taste" json:"avg_taste"`
	AvgPortion           float64   `bson:"avg_portion" json:"avg_portion"`
	AvgValue             float64   `bson:"avg_value" json:"avg_value"`
	ReorderRate          float64   `bson:"reorder_rate" json:"reorder_rate"`
	TrendDirection       string    `bson:"trend_direction" json:"trend_direction"` // "improving", "declining", "stable"
	HealthScore          float64   `bson:"health_score" json:"health_score"`
	LastCalculated       time.Time `bson:"last_calculated" json:"last_calculated"`
}

type DishFinancials struct {
	FoodCost       float64   `bson:"food_cost" json:"food_cost"`
	MarginPercent  float64   `bson:"margin_percent" json:"margin_percent"`
	WeeklyUnits    int       `bson:"weekly_units" json:"weekly_units"`
	WeeklyRevenue  float64   `bson:"weekly_revenue" json:"weekly_revenue"`
	LastCalculated time.Time `bson:"last_calculated" json:"last_calculated"`
}

type DishMapping struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	RestaurantID  primitive.ObjectID `bson:"restaurant_id" json:"restaurant_id"`
	CanonicalName string             `bson:"canonical_name" json:"canonical_name"`
	Category      string             `bson:"category" json:"category"`
	Platforms     DishPlatformData   `bson:"platforms" json:"platforms"`
	Analytics     DishAnalytics      `bson:"analytics" json:"analytics"`
	Financials    DishFinancials     `bson:"financials" json:"financials"`
	CreatedAt     time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time          `bson:"updated_at" json:"updated_at"`
}

// ── Actions ──────────────────────────────────────────────────────────────────

const (
	ActionTypeRemoveDish  = "REMOVE_DISH"
	ActionTypeChangePrice = "CHANGE_PRICE"
	ActionTypeRunPromo    = "RUN_PROMO"
	ActionTypeRestoreDish = "RESTORE_DISH"
	ActionTypeRollback    = "ROLLBACK"

	ActionStatusPending  = "pending"
	ActionStatusComplete = "complete"
	ActionStatusPartial  = "partial"
	ActionStatusFailed   = "failed"

	PlatformActionPending = "pending"
	PlatformActionSuccess = "success"
	PlatformActionFailed  = "failed"
	PlatformActionManual  = "manual"
)

type PlatformActionResult struct {
	Platform    string     `bson:"platform" json:"platform"`
	Status      string     `bson:"status" json:"status"`
	Message     string     `bson:"message,omitempty" json:"message,omitempty"`
	CompletedAt *time.Time `bson:"completed_at,omitempty" json:"completed_at,omitempty"`
	Metadata    map[string]interface{} `bson:"metadata,omitempty" json:"metadata,omitempty"`
}

type Action struct {
	ID            primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	RestaurantID  primitive.ObjectID     `bson:"restaurant_id" json:"restaurant_id"`
	ActionType    string                 `bson:"action_type" json:"action_type"`
	DishMappingID *primitive.ObjectID    `bson:"dish_mapping_id,omitempty" json:"dish_mapping_id,omitempty"`
	DishName      string                 `bson:"dish_name" json:"dish_name"`
	Params        map[string]interface{} `bson:"params" json:"params"`
	Initiator     string                 `bson:"initiator" json:"initiator"`
	Status        string                 `bson:"status" json:"status"`
	Results       []PlatformActionResult `bson:"results" json:"results"`
	CanRollback   bool                   `bson:"can_rollback" json:"can_rollback"`
	RolledBack    bool                   `bson:"rolled_back" json:"rolled_back"`
	CreatedAt     time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time              `bson:"updated_at" json:"updated_at"`
}

// ── Execution snapshots (pre-change state for rollback) ──────────────────────

type ExecutionSnapshot struct {
	ID             primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	ActionID       primitive.ObjectID     `bson:"action_id" json:"action_id"`
	Platform       string                 `bson:"platform" json:"platform"`
	DishMappingID  primitive.ObjectID     `bson:"dish_mapping_id" json:"dish_mapping_id"`
	PreChangeState map[string]interface{} `bson:"pre_change_state" json:"pre_change_state"`
	CreatedAt      time.Time              `bson:"created_at" json:"created_at"`
}

// ── Uber Eats order events (webhook storage) ─────────────────────────────────

type UberEatsOrderEvent struct {
	ID           primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	RestaurantID primitive.ObjectID     `bson:"restaurant_id" json:"restaurant_id"`
	EventType    string                 `bson:"event_type" json:"event_type"`
	OrderID      string                 `bson:"order_id,omitempty" json:"order_id,omitempty"`
	Payload      map[string]interface{} `bson:"payload" json:"payload"`
	ReceivedAt   time.Time              `bson:"received_at" json:"received_at"`
}
