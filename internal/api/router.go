package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/adityapat24/platemate-agentic/internal/api/handlers"
	"github.com/adityapat24/platemate-agentic/internal/db"
	"github.com/adityapat24/platemate-agentic/internal/models"
	"github.com/adityapat24/platemate-agentic/internal/platform/ubereats"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// NewRouter builds and returns the Gin engine with all routes registered.
func NewRouter(ueAdapter *ubereats.Adapter) *gin.Engine {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Restaurant-ID", "X-API-Key"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Health check (unauthenticated).
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now()})
	})

	// Uber Eats webhook — public route, no auth.
	wh := handlers.NewWebhookHandlers(ueAdapter)
	r.POST("/api/webhooks/ubereats", wh.HandleUberEatsWebhook)

	// All authenticated routes.
	api := r.Group("/api", authMiddleware())
	{
		// Dishes
		api.GET("/dishes", handlers.ListDishes)
		api.GET("/dishes/:id", handlers.GetDish)
		api.POST("/dishes/sync", handlers.SyncMenu(ueAdapter))

		// Actions
		api.POST("/actions/execute", handlers.ExecuteAction(ueAdapter))
		api.GET("/actions/:id", handlers.GetAction)
		api.GET("/actions", handlers.ListActions)
		api.POST("/actions/:id/rollback", handlers.RollbackAction(ueAdapter))

		// Onboarding
		ob := handlers.NewOnboardingHandlers(ueAdapter)
		api.GET("/onboarding/ubereats/auth-url", ob.GetUberAuthURL)
		api.GET("/onboarding/ubereats/callback", ob.UberCallback)
		api.GET("/onboarding/ubereats/stores", ob.ListMerchantStores)
		api.POST("/onboarding/ubereats/provision", ob.ProvisionStores)
		api.GET("/onboarding/ubereats/status", ob.GetConnectionStatus)
		api.GET("/onboarding/status", handlers.GetAllConnections)
	}

	return r
}

// authMiddleware reads X-Restaurant-ID (and optionally X-API-Key) from the
// request headers and sets restaurant_id in the Gin context.
// For the demo, if no restaurant is found it auto-creates one.
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		restaurantIDHeader := c.GetHeader("X-Restaurant-ID")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var restaurant models.Restaurant

		if apiKey != "" {
			// Authenticate by API key.
			err := db.Col("restaurants").FindOne(ctx, bson.M{"api_key": apiKey}).Decode(&restaurant)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
				return
			}
		} else if restaurantIDHeader != "" {
			// Authenticate by restaurant ID (for dev/demo).
			restID, err := primitive.ObjectIDFromHex(restaurantIDHeader)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid X-Restaurant-ID"})
				return
			}
			if err := db.Col("restaurants").FindOne(ctx, bson.M{"_id": restID}).Decode(&restaurant); err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "restaurant not found"})
				return
			}
		} else {
			// No credentials — use default demo restaurant (auto-seeded).
			if err := db.Col("restaurants").FindOne(ctx, bson.M{"api_key": "platemate-demo-key-2024"}).Decode(&restaurant); err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "no credentials provided"})
				return
			}
		}

		c.Set("restaurant_id", restaurant.ID.Hex())
		c.Set("restaurant", restaurant)
		c.Next()
	}
}

// SeedDefaultRestaurant creates the demo restaurant if it doesn't exist.
func SeedDefaultRestaurant() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var existing models.Restaurant
	err := db.Col("restaurants").FindOne(ctx, bson.M{"api_key": "platemate-demo-key-2024"}).Decode(&existing)
	if err == nil {
		log.Printf("Demo restaurant already exists: %s (%s)", existing.Name, existing.ID.Hex())
		return
	}

	now := time.Now()
	restaurant := models.Restaurant{
		Name:      "Demo Restaurant",
		APIKey:    "platemate-demo-key-2024",
		CreatedAt: now,
		UpdatedAt: now,
	}
	res, err := db.Col("restaurants").InsertOne(ctx, restaurant,
		options.InsertOne())
	if err != nil {
		log.Printf("Seed default restaurant: %v", err)
		return
	}
	log.Printf("Seeded demo restaurant: %s", res.InsertedID.(primitive.ObjectID).Hex())
}
