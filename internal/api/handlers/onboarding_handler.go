package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/adityapat24/platemate-agentic/internal/db"
	"github.com/adityapat24/platemate-agentic/internal/models"
	"github.com/adityapat24/platemate-agentic/internal/platform/ubereats"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// OnboardingHandlers groups all onboarding endpoints.
type OnboardingHandlers struct {
	ueAdapter *ubereats.Adapter
	// pending OAuth states: state → restaurant_id (in-memory for MVP)
	pendingStates map[string]string
}

func NewOnboardingHandlers(ueAdapter *ubereats.Adapter) *OnboardingHandlers {
	return &OnboardingHandlers{
		ueAdapter:     ueAdapter,
		pendingStates: make(map[string]string),
	}
}

// GetUberAuthURL generates the OAuth redirect URL for the merchant.
// GET /api/onboarding/ubereats/auth-url
func (h *OnboardingHandlers) GetUberAuthURL(c *gin.Context) {
	restaurantID := c.GetString("restaurant_id")

	// Generate a random state param for CSRF protection.
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "generate state"})
		return
	}
	state := hex.EncodeToString(b)
	h.pendingStates[state] = restaurantID

	authURL := h.ueAdapter.TokenManager().GenerateAuthURL(state)
	c.JSON(http.StatusOK, gin.H{"auth_url": authURL, "state": state})
}

// UberCallback handles the OAuth callback after merchant authorization.
// GET /api/onboarding/ubereats/callback
func (h *OnboardingHandlers) UberCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	errParam := c.Query("error")

	if errParam != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uber oauth denied: " + errParam})
		return
	}
	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing code or state"})
		return
	}

	restaurantID, ok := h.pendingStates[state]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired state"})
		return
	}
	delete(h.pendingStates, state)

	// Exchange code for auth-code token (stored in TokenManager keyed by state).
	_, err := h.ueAdapter.TokenManager().ExchangeCodeForToken(code, restaurantID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "token exchange: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Authorization successful",
		"restaurant_id": restaurantID,
	})
}

// ListMerchantStores lists all Uber Eats stores owned by the merchant.
// GET /api/onboarding/ubereats/stores
func (h *OnboardingHandlers) ListMerchantStores(c *gin.Context) {
	restaurantID := c.GetString("restaurant_id")

	userToken, ok := h.ueAdapter.TokenManager().GetAuthCodeToken(restaurantID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no authorization token — complete OAuth first"})
		return
	}

	stores, err := h.ueAdapter.StoreClient().ListMerchantStores(userToken)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"stores": stores.Stores})
}

// ProvisionStores activates PlateMate on selected Uber Eats stores.
// POST /api/onboarding/ubereats/provision
func (h *OnboardingHandlers) ProvisionStores(c *gin.Context) {
	restaurantID := c.GetString("restaurant_id")
	restID, _ := primitive.ObjectIDFromHex(restaurantID)

	var req struct {
		StoreIDs   []string `json:"store_ids" binding:"required"`
		WebhookURL string   `json:"webhook_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userToken, ok := h.ueAdapter.TokenManager().GetAuthCodeToken(restaurantID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no authorization token — complete OAuth first"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	provisioned := []string{}
	failed := []string{}

	for _, storeID := range req.StoreIDs {
		// Get store details.
		store, err := h.ueAdapter.StoreClient().GetStore(storeID)
		if err != nil {
			// Try to provision without details if get fails (sandbox may not support it).
			store = &ubereats.Store{ID: storeID, Name: storeID}
		}

		if err := h.ueAdapter.StoreClient().ProvisionStore(storeID, userToken, req.WebhookURL); err != nil {
			failed = append(failed, storeID)
			continue
		}

		// Upsert platform_connection record.
		now := time.Now()
		filter := bson.M{"restaurant_id": restID, "platform": "uber_eats"}
		update := bson.M{
			"$set": bson.M{
				"status":     models.ConnectionStatusConnected,
				"store_id":   storeID,
				"store_name": store.Name,
				"last_sync":  now,
				"updated_at": now,
			},
			"$setOnInsert": bson.M{
				"restaurant_id": restID,
				"platform":      "uber_eats",
				"created_at":    now,
			},
		}
		_, _ = db.Col("platform_connections").UpdateOne(ctx, filter, update,
			options.Update().SetUpsert(true))

		provisioned = append(provisioned, storeID)
	}

	// Clean up the auth-code token — no longer needed.
	h.ueAdapter.TokenManager().DeleteAuthCodeToken(restaurantID)

	c.JSON(http.StatusOK, gin.H{
		"provisioned": provisioned,
		"failed":      failed,
		"message":     "Provisioning complete — client_credentials token now active",
	})
}

// GetConnectionStatus returns the Uber Eats connection status for the restaurant.
// GET /api/onboarding/ubereats/status
func (h *OnboardingHandlers) GetConnectionStatus(c *gin.Context) {
	restaurantID := c.GetString("restaurant_id")
	restID, _ := primitive.ObjectIDFromHex(restaurantID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var conn models.PlatformConnection
	err := db.Col("platform_connections").FindOne(ctx, bson.M{
		"restaurant_id": restID,
		"platform":      "uber_eats",
	}).Decode(&conn)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":   "not_connected",
			"platform": "uber_eats",
		})
		return
	}
	c.JSON(http.StatusOK, conn)
}

// GetAllConnections returns all platform connection statuses.
// GET /api/onboarding/status
func GetAllConnections(c *gin.Context) {
	restaurantID := c.GetString("restaurant_id")
	restID, _ := primitive.ObjectIDFromHex(restaurantID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := db.Col("platform_connections").Find(ctx,
		bson.M{"restaurant_id": restID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var connections []models.PlatformConnection
	if err := cursor.All(ctx, &connections); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Ensure all known platforms appear in the response.
	knownPlatforms := []string{"uber_eats", "toast", "r365", "google"}
	connected := make(map[string]bool)
	for _, conn := range connections {
		connected[conn.Platform] = true
	}
	for _, p := range knownPlatforms {
		if !connected[p] {
			connections = append(connections, models.PlatformConnection{
				RestaurantID: restID,
				Platform:     p,
				Status:       models.ConnectionStatusDisconnected,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"connections": connections})
}
