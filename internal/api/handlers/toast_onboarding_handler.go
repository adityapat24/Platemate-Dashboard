package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/adityapat24/platemate-agentic/internal/db"
	"github.com/adityapat24/platemate-agentic/internal/models"
	"github.com/adityapat24/platemate-agentic/internal/platform/toast"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ToastOnboardingHandlers groups all Toast onboarding endpoints.
type ToastOnboardingHandlers struct {
	toastAdapter *toast.Adapter
}

func NewToastOnboardingHandlers(toastAdapter *toast.Adapter) *ToastOnboardingHandlers {
	return &ToastOnboardingHandlers{toastAdapter: toastAdapter}
}

// ConnectToast validates Toast credentials and fires an initial menu sync.
//
// POST /api/onboarding/toast/connect
//
// Request body:
//
//	{
//	  "client_id":               "...",   // Toast integration client ID
//	  "client_secret":           "...",   // Toast integration client secret
//	  "restaurant_external_id":  "..."    // Toast restaurant GUID
//	}
//
// Success (200):
//
//	{ "success": true, "restaurant_name": "...", "sync_status": "synced", "synced_items": 42 }
//
// Error (400/502):
//
//	{ "success": false, "error": "invalid credentials: ..." }
//
// Coordination note for Amine's wizard:
// This endpoint is the contract for Step 4 of the onboarding flow.
// The stub shape below is stable — Amine can build against it before live credentials land.
func (h *ToastOnboardingHandlers) ConnectToast(c *gin.Context) {
	restaurantID := c.GetString("restaurant_id")
	restID, _ := primitive.ObjectIDFromHex(restaurantID)

	var req struct {
		ClientID             string `json:"client_id" binding:"required"`
		ClientSecret         string `json:"client_secret" binding:"required"`
		RestaurantExternalID string `json:"restaurant_external_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "missing required fields: " + err.Error(),
		})
		return
	}

	// Step 1: Validate credentials by fetching a token.
	_, err := h.toastAdapter.TokenManager().GetTokenForCredentials(req.ClientID, req.ClientSecret)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"error":   "invalid credentials: " + err.Error(),
		})
		return
	}

	// Step 2: Validate restaurant GUID by hitting the restaurant endpoint.
	restaurant, err := h.toastAdapter.RestaurantClient().GetRestaurant(req.RestaurantExternalID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"error":   "restaurant not found or inaccessible: " + err.Error(),
		})
		return
	}

	// Step 3: Persist the connection.
	// store_id holds the Toast restaurant GUID (analogous to Uber Eats store_id).
	// TODO: encrypt client_id/client_secret before storing (Phase 3).
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	now := time.Now()
	filter := bson.M{"restaurant_id": restID, "platform": "toast"}
	update := bson.M{
		"$set": bson.M{
			"status":     models.ConnectionStatusConnected,
			"store_id":   req.RestaurantExternalID,
			"store_name": restaurant.Name,
			"last_sync":  now,
			"updated_at": now,
		},
		"$setOnInsert": bson.M{
			"restaurant_id": restID,
			"platform":      "toast",
			"created_at":    now,
		},
	}
	if _, err := db.Col("platform_connections").UpdateOne(ctx, filter, update,
		options.Update().SetUpsert(true)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "failed to persist connection: " + err.Error(),
		})
		return
	}

	// Step 4: Fire initial menu sync (best-effort; errors are surfaced but don't fail the connect).
	syncCtx, syncCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer syncCancel()

	syncedItems := 0
	syncStatus := "pending"
	syncErr := ""

	results, err := h.toastAdapter.SyncMenu(syncCtx, restaurantID)
	if err != nil {
		syncStatus = "failed"
		syncErr = err.Error()
	} else {
		syncedItems = len(results)
		syncStatus = "synced"
	}

	resp := gin.H{
		"success":         true,
		"restaurant_name": restaurant.Name,
		"restaurant_guid": req.RestaurantExternalID,
		"sync_status":     syncStatus,
		"synced_items":    syncedItems,
	}
	if syncErr != "" {
		resp["sync_error"] = syncErr
	}
	c.JSON(http.StatusOK, resp)
}

// GetToastConnectionStatus returns the current Toast connection status.
// GET /api/onboarding/toast/status
func (h *ToastOnboardingHandlers) GetToastConnectionStatus(c *gin.Context) {
	restaurantID := c.GetString("restaurant_id")
	restID, _ := primitive.ObjectIDFromHex(restaurantID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var conn models.PlatformConnection
	err := db.Col("platform_connections").FindOne(ctx, bson.M{
		"restaurant_id": restID,
		"platform":      "toast",
	}).Decode(&conn)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":   "not_connected",
			"platform": "toast",
		})
		return
	}
	c.JSON(http.StatusOK, conn)
}
