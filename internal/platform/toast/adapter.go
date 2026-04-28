package toast

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/adityapat24/platemate-agentic/internal/platform"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ErrWriteNotSupported is returned for all write operations.
// Toast write access requires custom integration credentials (Phase 3).
var ErrWriteNotSupported = errors.New("Toast write access requires custom integration credentials")

// Adapter implements platform.PlatformAdapter for Toast (read-only, Phase 2).
type Adapter struct {
	restaurantClient *RestaurantClient
	menuClient       *MenuClient
	analyticsClient  *AnalyticsClient
	stockClient      *StockClient
	tokenManager     *TokenManager
	db               *mongo.Database
}

// Config holds all configuration for the Toast adapter.
type Config struct {
	ClientID     string
	ClientSecret string
	APIBase      string // e.g. "https://ws-sandbox.toasttab.com"
	AuthBase     string // e.g. "https://ws-sandbox.toasttab.com"
}

func NewAdapter(cfg Config, db *mongo.Database) *Adapter {
	tm := NewTokenManager(cfg.ClientID, cfg.ClientSecret, cfg.AuthBase)
	return &Adapter{
		restaurantClient: NewRestaurantClient(tm, cfg.APIBase),
		menuClient:       NewMenuClient(tm, cfg.APIBase),
		analyticsClient:  NewAnalyticsClient(tm, cfg.APIBase),
		stockClient:      NewStockClient(tm, cfg.APIBase),
		tokenManager:     tm,
		db:               db,
	}
}

// TokenManager exposes the token manager for the onboarding handler.
func (a *Adapter) TokenManager() *TokenManager { return a.tokenManager }

// RestaurantClient exposes the restaurant client for the onboarding handler.
func (a *Adapter) RestaurantClient() *RestaurantClient { return a.restaurantClient }

func (a *Adapter) Name() string { return "toast" }

// CanAutomate returns false for all write operations (Phase 2 is read-only).
func (a *Adapter) CanAutomate(actionType platform.ActionType) bool {
	return false
}

// SyncMenu pulls the full Toast menu into dish_mappings and returns lightweight
// DishMappingResult structs for Arnav's entity resolution service.
//
// Flow:
//  1. Load platform_connection to get restaurantGUID
//  2. GET /menus/v2/menus — full hierarchy
//  3. GET /stock/v1/inventory — stock status per item
//  4. GET analytics (THIS_WEEK) — best-effort; skipped if ERA scope unavailable
//  5. Upsert dish_mappings for each item
func (a *Adapter) SyncMenu(ctx context.Context, restaurantID string) ([]platform.DishMappingResult, error) {
	conn, err := a.getConnection(ctx, restaurantID)
	if err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}
	restaurantGUID := conn.restaurantGUID

	menus, err := a.menuClient.GetMenus(restaurantGUID)
	if err != nil {
		return nil, fmt.Errorf("get menus: %w", err)
	}

	// Build stock lookup: itemGUID → StockItem (best-effort).
	stockByGUID := map[string]StockItem{}
	if inv, err := a.stockClient.GetInventory(restaurantGUID); err != nil {
		log.Printf("[toast] stock fetch skipped for %s: %v", restaurantGUID, err)
	} else {
		stockByGUID = inv
	}

	// Fetch this-week analytics (best-effort — requires enterprise-metrics:read scope).
	analyticsByGUID := map[string]MenuItemAnalytics{}
	if report, err := a.analyticsClient.GetMenuReport(restaurantGUID, "THIS_WEEK"); err != nil {
		log.Printf("[toast] analytics fetch skipped for %s: %v", restaurantGUID, err)
	} else {
		for _, row := range report.Data {
			analyticsByGUID[row.MenuItemGUID] = row
		}
	}

	flatItems := FlattenItems(menus)
	restID, _ := primitive.ObjectIDFromHex(restaurantID)
	now := time.Now()

	results := make([]platform.DishMappingResult, 0, len(flatItems))

	for _, fi := range flatItems {
		item := fi.Item

		// Determine stable ID: prefer multiLocationId, fall back to guid.
		stableID := item.MultiLocationID
		if stableID == "" {
			stableID = item.GUID
		}

		priceCents := item.PriceCents()
		stockStatus := "IN_STOCK"
		if s, ok := stockByGUID[item.GUID]; ok {
			stockStatus = s.StockStatus
		}

		r := platform.DishMappingResult{
			PlatformItemID:  stableID,
			PlatformStoreID: restaurantGUID,
			CanonicalName:   item.Name,
			PriceCents:      priceCents,
			Active:          true,
			Suspended:       stockStatus == "OUT_OF_STOCK",
			Category:        fi.GroupName,
		}
		results = append(results, r)

		// Build the Toast sub-document for dish_mappings.
		toastDoc := bson.M{
			"guid":             item.GUID,
			"multi_location_id": stableID,
			"price_cents":      priceCents,
			"stock_status":     stockStatus,
			"menu_group":       fi.GroupName,
			"last_synced":      now,
		}

		// Attach analytics if available.
		if analytics, ok := analyticsByGUID[item.GUID]; ok {
			toastDoc["weekly_units_sold"] = analytics.QuantitySold
			toastDoc["weekly_net_sales"] = analytics.NetSalesAmount
			toastDoc["weekly_avg_price"] = analytics.AveragePrice
			toastDoc["analytics_updated"] = now
		}

		filter := bson.M{
			"restaurant_id":  restID,
			"canonical_name": item.Name,
		}
		update := bson.M{
			"$set": bson.M{
				"platforms.toast": toastDoc,
				"category":        fi.GroupName,
				"updated_at":      now,
			},
			"$setOnInsert": bson.M{
				"restaurant_id":  restID,
				"canonical_name": item.Name,
				"category":       fi.GroupName,
				"analytics":      bson.M{},
				"financials":     bson.M{},
				"created_at":     now,
			},
		}
		opts := options.Update().SetUpsert(true)
		if _, err := a.db.Collection("dish_mappings").UpdateOne(ctx, filter, update, opts); err != nil {
			return nil, fmt.Errorf("upsert dish_mapping %s: %w", item.Name, err)
		}
	}

	// Update platform_connection last_sync.
	_, _ = a.db.Collection("platform_connections").UpdateOne(ctx,
		bson.M{"restaurant_id": restID, "platform": "toast"},
		bson.M{"$set": bson.M{"last_sync": now, "status": "connected"}},
	)

	return results, nil
}

// GetPreChangeState captures the current Toast state for rollback bookkeeping.
// Since writes are not supported in Phase 2, this returns current read state only.
func (a *Adapter) GetPreChangeState(ctx context.Context, dishMappingID string, actionType platform.ActionType) (map[string]interface{}, error) {
	dm, err := a.getDishMapping(ctx, dishMappingID)
	if err != nil {
		return nil, err
	}
	if dm.Toast == nil {
		return nil, fmt.Errorf("dish %s has no Toast mapping", dishMappingID)
	}

	return map[string]interface{}{
		"dish_mapping_id": dishMappingID,
		"action_type":     string(actionType),
		"guid":            dm.Toast.GUID,
		"price_cents":     dm.Toast.PriceCents,
		"stock_status":    dm.Toast.StockStatus,
		"snapshot_time":   time.Now().Unix(),
	}, nil
}

// ── Write operations — all return ErrWriteNotSupported ───────────────────────

func (a *Adapter) RemoveDish(_ context.Context, _ string) (*platform.ActionResult, error) {
	return writeNotSupported()
}

func (a *Adapter) RestoreDish(_ context.Context, _ string) (*platform.ActionResult, error) {
	return writeNotSupported()
}

func (a *Adapter) ChangePrice(_ context.Context, _ string, _ int64) (*platform.ActionResult, error) {
	return writeNotSupported()
}

func (a *Adapter) RunPromo(_ context.Context, _ string, _ platform.PromoConfig) (*platform.ActionResult, error) {
	return writeNotSupported()
}

func (a *Adapter) RevokePromo(_ context.Context, _ string) (*platform.ActionResult, error) {
	return writeNotSupported()
}

func (a *Adapter) Rollback(_ context.Context, _ string, _ map[string]interface{}) (*platform.ActionResult, error) {
	return writeNotSupported()
}

func writeNotSupported() (*platform.ActionResult, error) {
	return &platform.ActionResult{
		Platform: "toast",
		Status:   "failed",
		Message:  ErrWriteNotSupported.Error(),
	}, ErrWriteNotSupported
}

// GetManualSteps returns a checklist directing operators to the Toast dashboard
// for any write operation (since the adapter cannot automate writes in Phase 2).
func (a *Adapter) GetManualSteps(actionType platform.ActionType, _ string, _ map[string]interface{}) *platform.ManualChecklist {
	var steps []platform.ManualStep
	switch actionType {
	case platform.ActionRemoveDish:
		steps = []platform.ManualStep{
			{Step: 1, Description: "Log in to Toast Web at https://www.toasttab.com/login"},
			{Step: 2, Description: "Navigate to Menus → select the item"},
			{Step: 3, Description: "Set the item to Out of Stock or toggle Visibility off"},
			{Step: 4, Description: "Save changes"},
		}
	case platform.ActionRestoreDish:
		steps = []platform.ManualStep{
			{Step: 1, Description: "Log in to Toast Web at https://www.toasttab.com/login"},
			{Step: 2, Description: "Navigate to Menus → select the item"},
			{Step: 3, Description: "Set the item back to In Stock or toggle Visibility on"},
			{Step: 4, Description: "Save changes"},
		}
	case platform.ActionChangePrice:
		steps = []platform.ManualStep{
			{Step: 1, Description: "Log in to Toast Web at https://www.toasttab.com/login"},
			{Step: 2, Description: "Navigate to Menus → select the item"},
			{Step: 3, Description: "Update the price field and save"},
		}
	default:
		steps = []platform.ManualStep{
			{Step: 1, Description: "Log in to Toast Web and make the change manually"},
		}
	}
	return &platform.ManualChecklist{Platform: "toast", Steps: steps}
}

// ── Internal helpers ──────────────────────────────────────────────────────────

type toastConnectionData struct {
	restaurantGUID string
}

func (a *Adapter) getConnection(ctx context.Context, restaurantID string) (*toastConnectionData, error) {
	restID, err := primitive.ObjectIDFromHex(restaurantID)
	if err != nil {
		return nil, fmt.Errorf("invalid restaurant_id: %w", err)
	}

	var conn struct {
		StoreID string `bson:"store_id"`
	}
	err = a.db.Collection("platform_connections").FindOne(ctx, bson.M{
		"restaurant_id": restID,
		"platform":      "toast",
		"status":        "connected",
	}).Decode(&conn)
	if err != nil {
		return nil, fmt.Errorf("toast not connected for restaurant %s: %w", restaurantID, err)
	}
	if conn.StoreID == "" {
		return nil, fmt.Errorf("toast connection for restaurant %s has no restaurant_guid", restaurantID)
	}
	return &toastConnectionData{restaurantGUID: conn.StoreID}, nil
}

type dishToastData struct {
	Toast *struct {
		GUID        string `bson:"guid"`
		PriceCents  int64  `bson:"price_cents"`
		StockStatus string `bson:"stock_status"`
	} `bson:"toast"`
}

func (a *Adapter) getDishMapping(ctx context.Context, dishMappingID string) (*dishToastData, error) {
	id, err := primitive.ObjectIDFromHex(dishMappingID)
	if err != nil {
		return nil, fmt.Errorf("invalid dish_mapping_id: %w", err)
	}

	var result struct {
		Platforms dishToastData `bson:"platforms"`
	}
	if err := a.db.Collection("dish_mappings").FindOne(ctx, bson.M{"_id": id}).Decode(&result); err != nil {
		return nil, fmt.Errorf("get dish_mapping %s: %w", dishMappingID, err)
	}
	return &result.Platforms, nil
}

// NormalizeItemName strips common prefixes/suffixes for fuzzy matching (mirrors UberEats adapter).
func NormalizeItemName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, p := range []string{"classic ", "house ", "our famous ", "the "} {
		name = strings.TrimPrefix(name, p)
	}
	for _, s := range []string{" plate", " platter", " entrée", " entree"} {
		name = strings.TrimSuffix(name, s)
	}
	return name
}
