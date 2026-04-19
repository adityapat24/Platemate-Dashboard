package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/adityapat24/platemate-agentic/internal/db"
	"github.com/adityapat24/platemate-agentic/internal/models"
	"github.com/adityapat24/platemate-agentic/internal/platform"
	"github.com/adityapat24/platemate-agentic/internal/platform/ubereats"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type executeActionRequest struct {
	ActionType    string                 `json:"action_type" binding:"required"`
	DishMappingID string                 `json:"dish_mapping_id"`
	Params        map[string]interface{} `json:"params"`
}

// ExecuteAction dispatches an action to the Uber Eats adapter and stores the result.
func ExecuteAction(ueAdapter *ubereats.Adapter) gin.HandlerFunc {
	return func(c *gin.Context) {
		restaurantID := c.GetString("restaurant_id")
		restID, _ := primitive.ObjectIDFromHex(restaurantID)

		var req executeActionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Look up dish name for the action record.
		dishName := ""
		var dishMappingOID *primitive.ObjectID
		if req.DishMappingID != "" {
			dmID, err := primitive.ObjectIDFromHex(req.DishMappingID)
			if err == nil {
				dishMappingOID = &dmID
				var dm models.DishMapping
				if dbErr := db.Col("dish_mappings").FindOne(ctx, bson.M{"_id": dmID}).Decode(&dm); dbErr == nil {
					dishName = dm.CanonicalName
				}
			}
		}

		// Create action record.
		actionType := platform.ActionType(req.ActionType)
		now := time.Now()
		action := models.Action{
			RestaurantID:  restID,
			ActionType:    req.ActionType,
			DishMappingID: dishMappingOID,
			DishName:      dishName,
			Params:        req.Params,
			Initiator:     "operator",
			Status:        models.ActionStatusPending,
			Results:       []models.PlatformActionResult{},
			CanRollback:   false,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		insertRes, err := db.Col("actions").InsertOne(ctx, action)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "create action record: " + err.Error()})
			return
		}
		actionOID := insertRes.InsertedID.(primitive.ObjectID)

		// Capture pre-change state for rollback.
		if req.DishMappingID != "" {
			preState, err := ueAdapter.GetPreChangeState(ctx, req.DishMappingID, actionType)
			if err == nil {
				snap := models.ExecutionSnapshot{
					ActionID:       actionOID,
					Platform:       "uber_eats",
					DishMappingID:  *dishMappingOID,
					PreChangeState: preState,
					CreatedAt:      now,
				}
				_, _ = db.Col("execution_snapshots").InsertOne(ctx, snap)
			}
		}

		// Dispatch to Uber Eats adapter.
		var result *platform.ActionResult
		var execErr error

		switch actionType {
		case platform.ActionRemoveDish:
			result, execErr = ueAdapter.RemoveDish(ctx, req.DishMappingID)

		case platform.ActionRestoreDish:
			result, execErr = ueAdapter.RestoreDish(ctx, req.DishMappingID)

		case platform.ActionChangePrice:
			newPriceCents := int64(0)
			if p, ok := req.Params["new_price_cents"]; ok {
				switch v := p.(type) {
				case float64:
					newPriceCents = int64(v)
				case int64:
					newPriceCents = v
				}
			}
			if newPriceCents <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "new_price_cents must be > 0"})
				return
			}
			result, execErr = ueAdapter.ChangePrice(ctx, req.DishMappingID, newPriceCents)

		case platform.ActionRunPromo:
			promoConfig := buildPromoConfig(req.Params)
			result, execErr = ueAdapter.RunPromo(ctx, req.DishMappingID, promoConfig)

		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown action_type: " + req.ActionType})
			return
		}

		// Build platform result.
		platResult := models.PlatformActionResult{Platform: "uber_eats"}
		finalStatus := models.ActionStatusFailed

		if execErr != nil {
			platResult.Status = models.PlatformActionFailed
			platResult.Message = execErr.Error()
		} else if result != nil {
			platResult.Status = result.Status
			platResult.Message = result.Message
			platResult.Metadata = result.Metadata
			if result.Status == "success" {
				finalStatus = models.ActionStatusComplete
				t := time.Now()
				platResult.CompletedAt = &t
			}
		}

		// Update action record with result.
		canRollback := finalStatus == models.ActionStatusComplete &&
			actionType != platform.ActionRevokePromo

		_, _ = db.Col("actions").UpdateOne(ctx,
			bson.M{"_id": actionOID},
			bson.M{"$set": bson.M{
				"status":       finalStatus,
				"results":      []models.PlatformActionResult{platResult},
				"can_rollback": canRollback,
				"updated_at":   time.Now(),
			}},
		)

		c.JSON(http.StatusOK, gin.H{
			"action_id": actionOID.Hex(),
			"status":    finalStatus,
			"result":    platResult,
		})
	}
}

// GetAction returns a single action by ID.
func GetAction(c *gin.Context) {
	actionID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid action id"})
		return
	}
	restaurantID := c.GetString("restaurant_id")
	restID, _ := primitive.ObjectIDFromHex(restaurantID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var action models.Action
	err = db.Col("actions").FindOne(ctx, bson.M{
		"_id":           actionID,
		"restaurant_id": restID,
	}).Decode(&action)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "action not found"})
		return
	}
	c.JSON(http.StatusOK, action)
}

// ListActions returns paginated action history for the restaurant.
func ListActions(c *gin.Context) {
	restaurantID := c.GetString("restaurant_id")
	restID, _ := primitive.ObjectIDFromHex(restaurantID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"restaurant_id": restID}
	if at := c.Query("action_type"); at != "" {
		filter["action_type"] = at
	}
	if status := c.Query("status"); status != "" {
		filter["status"] = status
	}

	cursor, err := db.Col("actions").Find(ctx, filter,
		options.Find().
			SetSort(bson.D{{Key: "created_at", Value: -1}}).
			SetLimit(50),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var actions []models.Action
	if err := cursor.All(ctx, &actions); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if actions == nil {
		actions = []models.Action{}
	}
	c.JSON(http.StatusOK, gin.H{"actions": actions})
}

// RollbackAction reverses the last write action using the stored snapshot.
func RollbackAction(ueAdapter *ubereats.Adapter) gin.HandlerFunc {
	return func(c *gin.Context) {
		actionID, err := primitive.ObjectIDFromHex(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid action id"})
			return
		}
		restaurantID := c.GetString("restaurant_id")
		restID, _ := primitive.ObjectIDFromHex(restaurantID)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Verify action belongs to restaurant and is rollback-eligible.
		var action models.Action
		if err := db.Col("actions").FindOne(ctx, bson.M{
			"_id":           actionID,
			"restaurant_id": restID,
		}).Decode(&action); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "action not found"})
			return
		}
		if !action.CanRollback || action.RolledBack {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is not rollback-eligible"})
			return
		}

		// Load pre-change snapshot.
		var snap models.ExecutionSnapshot
		if err := db.Col("execution_snapshots").FindOne(ctx, bson.M{
			"action_id": actionID,
			"platform":  "uber_eats",
		}).Decode(&snap); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "rollback snapshot not found"})
			return
		}

		result, err := ueAdapter.Rollback(ctx, actionID.Hex(), snap.PreChangeState)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}

		// Mark action as rolled back.
		_, _ = db.Col("actions").UpdateOne(ctx,
			bson.M{"_id": actionID},
			bson.M{"$set": bson.M{
				"rolled_back": true,
				"can_rollback": false,
				"updated_at":   time.Now(),
			}},
		)

		c.JSON(http.StatusOK, gin.H{
			"status": result.Status,
			"message": result.Message,
		})
	}
}

// buildPromoConfig converts request params map to a platform.PromoConfig.
func buildPromoConfig(params map[string]interface{}) platform.PromoConfig {
	cfg := platform.PromoConfig{
		UserGroup:       "ALL_CUSTOMERS",
		UnlimitedBudget: true,
		StartTime:       time.Now(),
		EndTime:         time.Now().Add(7 * 24 * time.Hour),
	}

	if pt, ok := params["promo_type"].(string); ok {
		cfg.PromoType = platform.PromoType(pt)
	} else {
		cfg.PromoType = platform.PromoMenuItemDiscount
	}

	if dp, ok := params["discount_percent"].(float64); ok {
		cfg.DiscountPercent = int(dp)
	}
	if da, ok := params["discount_amount_cents"].(float64); ok {
		cfg.DiscountAmountCents = int64(da)
	}
	if ug, ok := params["user_group"].(string); ok {
		cfg.UserGroup = ug
	}
	if ub, ok := params["unlimited_budget"].(bool); ok {
		cfg.UnlimitedBudget = ub
	}
	if bc, ok := params["budget_cents"].(float64); ok {
		cfg.BudgetCents = int64(bc)
		cfg.UnlimitedBudget = false
	}
	if extID, ok := params["external_promo_id"].(string); ok {
		cfg.ExternalPromoID = extID
	}

	return cfg
}
