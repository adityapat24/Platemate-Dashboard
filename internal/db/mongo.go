package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var Client *mongo.Client
var Database *mongo.Database

func Connect(uri, dbName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOpts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return fmt.Errorf("mongo connect: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("mongo ping: %w", err)
	}

	Client = client
	Database = client.Database(dbName)
	log.Printf("Connected to MongoDB: %s/%s", uri, dbName)
	return nil
}

func Disconnect() {
	if Client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = Client.Disconnect(ctx)
	}
}

func Col(name string) *mongo.Collection {
	return Database.Collection(name)
}

// EnsureIndexes creates all necessary MongoDB indexes.
func EnsureIndexes(ctx context.Context) error {
	indexes := map[string][]mongo.IndexModel{
		"restaurants": {
			{Keys: bson.D{{Key: "api_key", Value: 1}}, Options: options.Index().SetUnique(true)},
		},
		"dish_mappings": {
			{Keys: bson.D{{Key: "restaurant_id", Value: 1}}},
			{Keys: bson.D{{Key: "restaurant_id", Value: 1}, {Key: "canonical_name", Value: 1}}, Options: options.Index().SetUnique(true)},
		},
		"platform_connections": {
			{Keys: bson.D{{Key: "restaurant_id", Value: 1}, {Key: "platform", Value: 1}}, Options: options.Index().SetUnique(true)},
		},
		"actions": {
			{Keys: bson.D{{Key: "restaurant_id", Value: 1}}},
			{Keys: bson.D{{Key: "restaurant_id", Value: 1}, {Key: "created_at", Value: -1}}},
		},
		"execution_snapshots": {
			{Keys: bson.D{{Key: "action_id", Value: 1}}},
		},
		"uber_eats_orders": {
			{Keys: bson.D{{Key: "restaurant_id", Value: 1}}},
			{Keys: bson.D{{Key: "order_id", Value: 1}}},
		},
	}

	for col, idxs := range indexes {
		if _, err := Database.Collection(col).Indexes().CreateMany(ctx, idxs); err != nil {
			return fmt.Errorf("index %s: %w", col, err)
		}
	}
	return nil
}
