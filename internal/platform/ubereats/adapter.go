package ubereats

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/adityapat24/platemate-agentic/internal/platform"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Adapter implements platform.PlatformAdapter for Uber Eats.
type Adapter struct {
	storeClient  *StoreClient
	menuClient   *MenuClient
	promoClient  *PromotionsClient
	tokenManager *TokenManager
	db           *mongo.Database
}

// Config holds all configuration for the Uber Eats adapter.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	APIBase      string
	AuthBase     string
}

func NewAdapter(cfg Config, db *mongo.Database) *Adapter {
	tm := NewTokenManager(cfg.ClientID, cfg.ClientSecret, cfg.RedirectURI, cfg.AuthBase)
	return &Adapter{
		storeClient:  NewStoreClient(tm, cfg.APIBase),
		menuClient:   NewMenuClient(tm, cfg.APIBase),
		promoClient:  NewPromotionsClient(tm, cfg.APIBase),
		tokenManager: tm,
		db:           db,
	}
}

// TokenManager exposes the token manager for onboarding handlers.
func (a *Adapter) TokenManager() *TokenManager { return a.tokenManager }

// StoreClient exposes the store client for onboarding handlers.
func (a *Adapter) StoreClient() *StoreClient { return a.storeClient }

func (a *Adapter) Name() string { return "uber_eats" }

func (a *Adapter) CanAutomate(actionType platform.ActionType) bool {
	switch actionType {
	case platform.ActionRemoveDish, platform.ActionRestoreDish,
		platform.ActionChangePrice, platform.ActionRunPromo, platform.ActionRevokePromo:
		return true
	}
	return false
}

// SyncMenu pulls the full menu from Uber Eats and returns lightweight results
// suitable for entity resolution.  It also upserts dish_mappings in MongoDB.
func (a *Adapter) SyncMenu(ctx context.Context, restaurantID string) ([]platform.DishMappingResult, error) {
	conn, err := a.getConnection(ctx, restaurantID)
	if err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}

	menu, err := a.menuClient.GetMenu(conn.StoreID, "DELIVERY")
	if err != nil {
		return nil, fmt.Errorf("get menu: %w", err)
	}

	// Build a category lookup: item_id → category name
	catByItemID := make(map[string]string)
	for _, cat := range menu.Categories {
		for _, e := range cat.Entities {
			catByItemID[e.ID] = cat.Title.TitleString()
		}
	}

	results := make([]platform.DishMappingResult, 0, len(menu.Items))
	for _, item := range menu.Items {
		suspended := false
		var suspendUntil int64
		if item.SuspensionInfo != nil && item.SuspensionInfo.Suspension != nil {
			s := item.SuspensionInfo.Suspension
			if s.SuspendUntil > time.Now().Unix() {
				suspended = true
				suspendUntil = s.SuspendUntil
			}
		}

		r := platform.DishMappingResult{
			PlatformItemID:  item.ID,
			PlatformStoreID: conn.StoreID,
			CanonicalName:   item.Title.TitleString(),
			PriceCents:      item.PriceInfo.Price,
			Active:          true,
			Suspended:       suspended,
			Category:        catByItemID[item.ID],
		}
		results = append(results, r)

		// Upsert dish_mapping for this item.
		restID, _ := primitive.ObjectIDFromHex(restaurantID)
		now := time.Now()
		filter := bson.M{
			"restaurant_id":  restID,
			"canonical_name": r.CanonicalName,
		}
		update := bson.M{
			"$set": bson.M{
				"platforms.uber_eats": bson.M{
					"item_id":       item.ID,
					"store_id":      conn.StoreID,
					"price_cents":   item.PriceInfo.Price,
					"active":        true,
					"suspended":     suspended,
					"suspend_until": suspendUntil,
					"last_synced":   now,
				},
				"category":   r.Category,
				"updated_at": now,
			},
			"$setOnInsert": bson.M{
				"restaurant_id":  restID,
				"canonical_name": r.CanonicalName,
				"category":       r.Category,
				"analytics":      bson.M{},
				"financials":     bson.M{},
				"created_at":     now,
			},
		}
		opts := options.Update().SetUpsert(true)
		if _, err := a.db.Collection("dish_mappings").UpdateOne(ctx, filter, update, opts); err != nil {
			return nil, fmt.Errorf("upsert dish_mapping %s: %w", r.CanonicalName, err)
		}
	}

	// Update connection last_sync timestamp.
	_, _ = a.db.Collection("platform_connections").UpdateOne(ctx,
		bson.M{"restaurant_id": func() primitive.ObjectID {
			id, _ := primitive.ObjectIDFromHex(restaurantID)
			return id
		}(), "platform": "uber_eats"},
		bson.M{"$set": bson.M{"last_sync": time.Now(), "status": "connected"}},
	)

	return results, nil
}

// GetPreChangeState captures the current Uber Eats state of a dish for rollback.
func (a *Adapter) GetPreChangeState(ctx context.Context, dishMappingID string, actionType platform.ActionType) (map[string]interface{}, error) {
	dm, err := a.getDishMapping(ctx, dishMappingID)
	if err != nil {
		return nil, err
	}
	if dm.UberEats == nil {
		return nil, fmt.Errorf("dish %s has no Uber Eats mapping", dishMappingID)
	}

	state := map[string]interface{}{
		"dish_mapping_id": dishMappingID,
		"action_type":     string(actionType),
		"item_id":         dm.UberEats.ItemID,
		"store_id":        dm.UberEats.StoreID,
		"price_cents":     dm.UberEats.PriceCents,
		"suspended":       dm.UberEats.Suspended,
		"suspend_until":   dm.UberEats.SuspendUntil,
		"snapshot_time":   time.Now().Unix(),
	}
	return state, nil
}

// RemoveDish suspends the dish on Uber Eats indefinitely.
func (a *Adapter) RemoveDish(ctx context.Context, dishMappingID string) (*platform.ActionResult, error) {
	dm, err := a.getDishMapping(ctx, dishMappingID)
	if err != nil {
		return nil, err
	}
	if dm.UberEats == nil {
		return nil, fmt.Errorf("dish %s has no Uber Eats mapping", dishMappingID)
	}

	suspendUntil := time.Now().Add(365 * 24 * time.Hour).Unix()
	if err := a.menuClient.SuspendItem(dm.UberEats.StoreID, dm.UberEats.ItemID, suspendUntil, "Removed from menu"); err != nil {
		return &platform.ActionResult{
			Platform: "uber_eats",
			Status:   "failed",
			Message:  err.Error(),
		}, nil
	}

	// Update dish_mapping in DB.
	dmID, _ := primitive.ObjectIDFromHex(dishMappingID)
	_, _ = a.db.Collection("dish_mappings").UpdateOne(ctx,
		bson.M{"_id": dmID},
		bson.M{"$set": bson.M{
			"platforms.uber_eats.suspended":     true,
			"platforms.uber_eats.suspend_until": suspendUntil,
			"updated_at":                        time.Now(),
		}},
	)

	return &platform.ActionResult{
		Platform: "uber_eats",
		Status:   "success",
		Message:  fmt.Sprintf("Item %s suspended on Uber Eats", dm.UberEats.ItemID),
		Metadata: map[string]interface{}{
			"item_id":       dm.UberEats.ItemID,
			"store_id":      dm.UberEats.StoreID,
			"suspend_until": suspendUntil,
		},
	}, nil
}

// RestoreDish clears the suspension on Uber Eats.
func (a *Adapter) RestoreDish(ctx context.Context, dishMappingID string) (*platform.ActionResult, error) {
	dm, err := a.getDishMapping(ctx, dishMappingID)
	if err != nil {
		return nil, err
	}
	if dm.UberEats == nil {
		return nil, fmt.Errorf("dish %s has no Uber Eats mapping", dishMappingID)
	}

	if err := a.menuClient.RestoreItem(dm.UberEats.StoreID, dm.UberEats.ItemID); err != nil {
		return &platform.ActionResult{
			Platform: "uber_eats",
			Status:   "failed",
			Message:  err.Error(),
		}, nil
	}

	dmID, _ := primitive.ObjectIDFromHex(dishMappingID)
	_, _ = a.db.Collection("dish_mappings").UpdateOne(ctx,
		bson.M{"_id": dmID},
		bson.M{"$set": bson.M{
			"platforms.uber_eats.suspended":     false,
			"platforms.uber_eats.suspend_until": 0,
			"updated_at":                        time.Now(),
		}},
	)

	return &platform.ActionResult{
		Platform: "uber_eats",
		Status:   "success",
		Message:  fmt.Sprintf("Item %s restored on Uber Eats", dm.UberEats.ItemID),
	}, nil
}

// ChangePrice updates the item price on Uber Eats.
// newPriceCents is in cents (e.g. 1499 for $14.99).
func (a *Adapter) ChangePrice(ctx context.Context, dishMappingID string, newPriceCents int64) (*platform.ActionResult, error) {
	dm, err := a.getDishMapping(ctx, dishMappingID)
	if err != nil {
		return nil, err
	}
	if dm.UberEats == nil {
		return nil, fmt.Errorf("dish %s has no Uber Eats mapping", dishMappingID)
	}

	if err := a.menuClient.ChangeItemPrice(dm.UberEats.StoreID, dm.UberEats.ItemID, newPriceCents); err != nil {
		return &platform.ActionResult{
			Platform: "uber_eats",
			Status:   "failed",
			Message:  err.Error(),
		}, nil
	}

	dmID, _ := primitive.ObjectIDFromHex(dishMappingID)
	_, _ = a.db.Collection("dish_mappings").UpdateOne(ctx,
		bson.M{"_id": dmID},
		bson.M{"$set": bson.M{
			"platforms.uber_eats.price_cents": newPriceCents,
			"updated_at":                      time.Now(),
		}},
	)

	return &platform.ActionResult{
		Platform: "uber_eats",
		Status:   "success",
		Message:  fmt.Sprintf("Price updated to $%.2f on Uber Eats", float64(newPriceCents)/100),
		Metadata: map[string]interface{}{
			"item_id":         dm.UberEats.ItemID,
			"new_price_cents": newPriceCents,
		},
	}, nil
}

// RunPromo creates a promotion on Uber Eats based on the PromoConfig.
func (a *Adapter) RunPromo(ctx context.Context, dishMappingID string, config platform.PromoConfig) (*platform.ActionResult, error) {
	dm, err := a.getDishMapping(ctx, dishMappingID)
	if err != nil {
		return nil, err
	}
	if dm.UberEats == nil {
		return nil, fmt.Errorf("dish %s has no Uber Eats mapping", dishMappingID)
	}

	conn, connErr := a.getConnectionByDishMapping(ctx, dm)
	if connErr != nil {
		return nil, connErr
	}

	userGroup := config.UserGroup
	if userGroup == "" {
		userGroup = "ALL_CUSTOMERS"
	}

	var promoResp *PromotionResponse
	var promoErr error

	switch config.PromoType {
	case platform.PromoMenuItemDiscount:
		itemIDs := config.ItemIDs
		if len(itemIDs) == 0 {
			itemIDs = []string{dm.UberEats.ItemID}
		}
		promoResp, promoErr = a.promoClient.CreateMenuItemDiscount(
			conn.StoreID,
			itemIDs,
			config.DiscountPercent,
			userGroup,
			config.StartTime,
			config.EndTime,
			config.BudgetCents,
			config.UnlimitedBudget,
			config.ExternalPromoID,
		)
	case platform.PromoPercentOff:
		promoResp, promoErr = a.promoClient.CreatePercentOff(
			conn.StoreID,
			config.DiscountPercent,
			0, 0, // no min spend or max discount for simple case
			userGroup,
			config.StartTime,
			config.EndTime,
			config.BudgetCents,
			config.UnlimitedBudget,
			config.ExternalPromoID,
		)
	default:
		return nil, fmt.Errorf("unsupported promo type: %s", config.PromoType)
	}

	if promoErr != nil {
		return &platform.ActionResult{
			Platform: "uber_eats",
			Status:   "failed",
			Message:  promoErr.Error(),
		}, nil
	}

	return &platform.ActionResult{
		Platform: "uber_eats",
		Status:   "success",
		Message:  fmt.Sprintf("Promotion %s created on Uber Eats", promoResp.PromotionID),
		Metadata: map[string]interface{}{
			"promotion_id": promoResp.PromotionID,
			"promo_type":   promoResp.PromoType,
			"state":        promoResp.State,
		},
	}, nil
}

// RevokePromo cancels an active promotion.
func (a *Adapter) RevokePromo(_ context.Context, promoID string) (*platform.ActionResult, error) {
	if err := a.promoClient.RevokePromotion(promoID); err != nil {
		return &platform.ActionResult{
			Platform: "uber_eats",
			Status:   "failed",
			Message:  err.Error(),
		}, nil
	}
	return &platform.ActionResult{
		Platform: "uber_eats",
		Status:   "success",
		Message:  fmt.Sprintf("Promotion %s revoked", promoID),
	}, nil
}

// Rollback reverses the last write action using the stored pre-change state.
func (a *Adapter) Rollback(ctx context.Context, _ string, preChangeState map[string]interface{}) (*platform.ActionResult, error) {
	actionType, _ := preChangeState["action_type"].(string)
	dishMappingID, _ := preChangeState["dish_mapping_id"].(string)

	switch platform.ActionType(actionType) {
	case platform.ActionRemoveDish:
		return a.RestoreDish(ctx, dishMappingID)

	case platform.ActionRestoreDish:
		return a.RemoveDish(ctx, dishMappingID)

	case platform.ActionChangePrice:
		oldPrice, ok := preChangeState["price_cents"]
		if !ok {
			return nil, fmt.Errorf("rollback CHANGE_PRICE: missing price_cents in snapshot")
		}
		var oldPriceCents int64
		switch v := oldPrice.(type) {
		case int64:
			oldPriceCents = v
		case int32:
			oldPriceCents = int64(v)
		case float64:
			oldPriceCents = int64(v)
		}
		return a.ChangePrice(ctx, dishMappingID, oldPriceCents)

	case platform.ActionRunPromo:
		promoID, _ := preChangeState["promotion_id"].(string)
		if promoID == "" {
			return nil, fmt.Errorf("rollback RUN_PROMO: missing promotion_id in snapshot")
		}
		return a.RevokePromo(ctx, promoID)

	default:
		return nil, fmt.Errorf("rollback: unknown action type %s", actionType)
	}
}

// GetManualSteps returns nil because Uber Eats is fully automated.
func (a *Adapter) GetManualSteps(_ platform.ActionType, _ string, _ map[string]interface{}) *platform.ManualChecklist {
	return nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

type dishUberEatsData struct {
	UberEats *struct {
		ItemID       string `bson:"item_id"`
		StoreID      string `bson:"store_id"`
		PriceCents   int64  `bson:"price_cents"`
		Suspended    bool   `bson:"suspended"`
		SuspendUntil int64  `bson:"suspend_until"`
	} `bson:"uber_eats"`
	RestaurantID primitive.ObjectID `bson:"restaurant_id"`
}

func (a *Adapter) getDishMapping(ctx context.Context, dishMappingID string) (*dishUberEatsData, error) {
	id, err := primitive.ObjectIDFromHex(dishMappingID)
	if err != nil {
		return nil, fmt.Errorf("invalid dish_mapping_id: %w", err)
	}

	var result struct {
		RestaurantID primitive.ObjectID `bson:"restaurant_id"`
		Platforms    dishUberEatsData   `bson:"platforms"`
	}
	err = a.db.Collection("dish_mappings").FindOne(ctx, bson.M{"_id": id}).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("get dish_mapping %s: %w", dishMappingID, err)
	}
	result.Platforms.RestaurantID = result.RestaurantID
	return &result.Platforms, nil
}

type connectionData struct {
	StoreID string `bson:"store_id"`
}

func (a *Adapter) getConnection(ctx context.Context, restaurantID string) (*connectionData, error) {
	restID, err := primitive.ObjectIDFromHex(restaurantID)
	if err != nil {
		return nil, fmt.Errorf("invalid restaurant_id: %w", err)
	}

	var conn connectionData
	err = a.db.Collection("platform_connections").FindOne(ctx, bson.M{
		"restaurant_id": restID,
		"platform":      "uber_eats",
		"status":        "connected",
	}).Decode(&conn)
	if err != nil {
		return nil, fmt.Errorf("uber eats not connected for restaurant %s: %w", restaurantID, err)
	}
	return &conn, nil
}

func (a *Adapter) getConnectionByDishMapping(ctx context.Context, dm *dishUberEatsData) (*connectionData, error) {
	if dm.UberEats != nil && dm.UberEats.StoreID != "" {
		return &connectionData{StoreID: dm.UberEats.StoreID}, nil
	}
	restIDHex := dm.RestaurantID.Hex()
	conn, err := a.getConnection(ctx, restIDHex)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// NormalizeItemName strips common prefixes/suffixes for fuzzy matching.
func NormalizeItemName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	prefixes := []string{"classic ", "house ", "our famous ", "the "}
	for _, p := range prefixes {
		name = strings.TrimPrefix(name, p)
	}
	suffixes := []string{" plate", " platter", " entrée", " entree"}
	for _, s := range suffixes {
		name = strings.TrimSuffix(name, s)
	}
	return name
}
