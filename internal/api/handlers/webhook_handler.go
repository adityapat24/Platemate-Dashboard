package handlers

import (
	"context"
	"log"
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

// WebhookHandlers groups all webhook endpoints.
type WebhookHandlers struct {
	ueAdapter *ubereats.Adapter
}

func NewWebhookHandlers(ueAdapter *ubereats.Adapter) *WebhookHandlers {
	return &WebhookHandlers{ueAdapter: ueAdapter}
}

// HandleUberEatsWebhook handles all incoming Uber Eats webhook events.
// POST /api/webhooks/ubereats
// Always returns 200 immediately per Uber's requirements.
func (h *WebhookHandlers) HandleUberEatsWebhook(c *gin.Context) {
	var event ubereats.WebhookEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		// Return 200 even on parse error to prevent retries.
		log.Printf("[webhook] uber eats: failed to parse event: %v", err)
		c.Status(http.StatusOK)
		return
	}

	log.Printf("[webhook] uber eats event: %s (id: %s)", event.EventType, event.EventID)

	// Process event asynchronously so we return 200 immediately.
	go h.processUberEvent(event)

	c.Status(http.StatusOK)
}

func (h *WebhookHandlers) processUberEvent(event ubereats.WebhookEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	switch event.EventType {
	case "store.provisioned":
		h.handleStoreProvisioned(ctx, event)

	case "store.deprovisioned":
		h.handleStoreDeprovisioned(ctx, event)

	case "store.menu_refresh_request":
		h.handleMenuRefreshRequest(ctx, event)

	case "orders.notification", "orders.scheduled.notification",
		"orders.release", "orders.failure", "orders.fulfillment_issues.resolved":
		h.handleOrderEvent(ctx, event)

	case "eats.report.success":
		h.handleReportSuccess(ctx, event)

	default:
		log.Printf("[webhook] uber eats: unhandled event type: %s", event.EventType)
	}
}

func (h *WebhookHandlers) handleStoreProvisioned(ctx context.Context, event ubereats.WebhookEvent) {
	storeID, _ := event.Meta["store_id"].(string)
	if storeID == "" {
		log.Printf("[webhook] store.provisioned: missing store_id")
		return
	}

	// Find and update the platform_connection for this store.
	now := time.Now()
	res, err := db.Col("platform_connections").UpdateOne(ctx,
		bson.M{"store_id": storeID, "platform": "uber_eats"},
		bson.M{"$set": bson.M{
			"status":     models.ConnectionStatusConnected,
			"updated_at": now,
		}},
		options.Update().SetUpsert(false),
	)
	if err != nil {
		log.Printf("[webhook] store.provisioned: update connection: %v", err)
		return
	}
	log.Printf("[webhook] store.provisioned: updated %d connection(s) for store %s", res.ModifiedCount, storeID)
}

func (h *WebhookHandlers) handleStoreDeprovisioned(ctx context.Context, event ubereats.WebhookEvent) {
	storeID, _ := event.Meta["store_id"].(string)
	now := time.Now()
	_, err := db.Col("platform_connections").UpdateOne(ctx,
		bson.M{"store_id": storeID, "platform": "uber_eats"},
		bson.M{"$set": bson.M{
			"status":     models.ConnectionStatusDisconnected,
			"updated_at": now,
		}},
	)
	if err != nil {
		log.Printf("[webhook] store.deprovisioned: update connection: %v", err)
	}
	log.Printf("[webhook] store.deprovisioned: store %s", storeID)
}

func (h *WebhookHandlers) handleMenuRefreshRequest(ctx context.Context, event ubereats.WebhookEvent) {
	storeID, _ := event.Meta["store_id"].(string)
	log.Printf("[webhook] store.menu_refresh_request for store %s — queuing sync", storeID)

	// Find which restaurant owns this store and trigger a menu sync.
	var conn struct {
		RestaurantID primitive.ObjectID `bson:"restaurant_id"`
	}
	err := db.Col("platform_connections").FindOne(ctx,
		bson.M{"store_id": storeID, "platform": "uber_eats"},
	).Decode(&conn)
	if err != nil {
		log.Printf("[webhook] menu_refresh: connection not found for store %s", storeID)
		return
	}

	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if _, err := h.ueAdapter.SyncMenu(syncCtx, conn.RestaurantID.Hex()); err != nil {
			log.Printf("[webhook] menu_refresh sync error: %v", err)
		}
	}()
}

func (h *WebhookHandlers) handleOrderEvent(ctx context.Context, event ubereats.WebhookEvent) {
	orderID, _ := event.Meta["order_id"].(string)
	storeID, _ := event.Meta["store_id"].(string)

	// Find the restaurant for this store.
	var conn struct {
		RestaurantID primitive.ObjectID `bson:"restaurant_id"`
	}
	if err := db.Col("platform_connections").FindOne(ctx,
		bson.M{"store_id": storeID, "platform": "uber_eats"},
	).Decode(&conn); err != nil {
		// Store not found — still log the event.
		log.Printf("[webhook] order event %s: unknown store %s", event.EventType, storeID)
	}

	orderEvent := models.UberEatsOrderEvent{
		RestaurantID: conn.RestaurantID,
		EventType:    event.EventType,
		OrderID:      orderID,
		Payload:      event.Meta,
		ReceivedAt:   time.Now(),
	}
	if _, err := db.Col("uber_eats_orders").InsertOne(ctx, orderEvent); err != nil {
		log.Printf("[webhook] order event insert: %v", err)
	}
}

func (h *WebhookHandlers) handleReportSuccess(ctx context.Context, event ubereats.WebhookEvent) {
	log.Printf("[webhook] eats.report.success: %+v", event.Meta)
	// Store for async processing — download URL is in meta.sections[].download_url
	if _, err := db.Col("uber_eats_orders").InsertOne(ctx, models.UberEatsOrderEvent{
		EventType:  "eats.report.success",
		Payload:    event.Meta,
		ReceivedAt: time.Now(),
	}); err != nil {
		log.Printf("[webhook] report success insert: %v", err)
	}
}
