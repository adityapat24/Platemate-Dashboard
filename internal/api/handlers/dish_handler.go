package handlers

import (
	"context"
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

// ListDishes returns all dish_mappings for the authenticated restaurant.
func ListDishes(c *gin.Context) {
	restaurantID := c.GetString("restaurant_id")
	restID, err := primitive.ObjectIDFromHex(restaurantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid restaurant id"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := db.Col("dish_mappings").Find(ctx,
		bson.M{"restaurant_id": restID},
		options.Find().SetSort(bson.D{{Key: "canonical_name", Value: 1}}),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var dishes []models.DishMapping
	if err := cursor.All(ctx, &dishes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if dishes == nil {
		dishes = []models.DishMapping{}
	}
	c.JSON(http.StatusOK, gin.H{"dishes": dishes})
}

// GetDish returns a single dish_mapping by ID.
func GetDish(c *gin.Context) {
	dishID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dish id"})
		return
	}

	restaurantID := c.GetString("restaurant_id")
	restID, _ := primitive.ObjectIDFromHex(restaurantID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var dish models.DishMapping
	err = db.Col("dish_mappings").FindOne(ctx, bson.M{
		"_id":           dishID,
		"restaurant_id": restID,
	}).Decode(&dish)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "dish not found"})
		return
	}
	c.JSON(http.StatusOK, dish)
}

// SyncMenu triggers an Uber Eats menu sync for the restaurant.
func SyncMenu(ueAdapter *ubereats.Adapter) gin.HandlerFunc {
	return func(c *gin.Context) {
		restaurantID := c.GetString("restaurant_id")

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		results, err := ueAdapter.SyncMenu(ctx, restaurantID)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"synced":  len(results),
			"message": "Menu sync complete",
		})
	}
}
