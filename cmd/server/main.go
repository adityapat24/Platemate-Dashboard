package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/adityapat24/platemate-agentic/internal/api"
	"github.com/adityapat24/platemate-agentic/internal/db"
	"github.com/adityapat24/platemate-agentic/internal/platform/toast"
	"github.com/adityapat24/platemate-agentic/internal/platform/ubereats"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file (ignore error in production where env vars are set externally).
	_ = godotenv.Load()

	mongoURI := getEnv("MONGO_URI", "mongodb://localhost:27017")
	mongoDB := getEnv("MONGO_DB", "platemate")
	port := getEnv("PORT", "8080")

	// Connect to MongoDB.
	if err := db.Connect(mongoURI, mongoDB); err != nil {
		log.Fatalf("MongoDB connect: %v", err)
	}
	defer db.Disconnect()

	// Ensure indexes.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := db.EnsureIndexes(ctx); err != nil {
		log.Printf("Warning: ensure indexes: %v", err)
	}
	cancel()

	// Seed default restaurant for demo.
	api.SeedDefaultRestaurant()

	// Build the Uber Eats adapter.
	ueCfg := ubereats.Config{
		ClientID:     getEnv("UBER_EATS_CLIENT_ID", ""),
		ClientSecret: getEnv("UBER_EATS_CLIENT_SECRET", ""),
		RedirectURI:  getEnv("UBER_EATS_REDIRECT_URI", "http://localhost:3000/auth/uber/callback"),
		APIBase:      getEnv("UBER_EATS_API_BASE", "https://test-api.uber.com"),
		AuthBase:     getEnv("UBER_EATS_AUTH_BASE", "https://sandbox-login.uber.com"),
	}
	if ueCfg.ClientID == "" || ueCfg.ClientSecret == "" {
		log.Println("Warning: UBER_EATS_CLIENT_ID / UBER_EATS_CLIENT_SECRET not set — Uber Eats API calls will fail")
	}
	ueAdapter := ubereats.NewAdapter(ueCfg, db.Database)

	// Build the Toast adapter (credentials are optional — adapter is nil until .env is populated).
	var toastAdapter *toast.Adapter
	toastClientID := getEnv("TOAST_CLIENT_ID", "")
	toastClientSecret := getEnv("TOAST_CLIENT_SECRET", "")
	if toastClientID == "" || toastClientSecret == "" {
		log.Println("Warning: TOAST_CLIENT_ID / TOAST_CLIENT_SECRET not set — Toast adapter disabled until credentials arrive")
	} else {
		toastCfg := toast.Config{
			ClientID:     toastClientID,
			ClientSecret: toastClientSecret,
			APIBase:      getEnv("TOAST_API_BASE", "https://ws-sandbox.toasttab.com"),
			AuthBase:     getEnv("TOAST_AUTH_BASE", "https://ws-sandbox.toasttab.com"),
		}
		toastAdapter = toast.NewAdapter(toastCfg, db.Database)
		log.Println("Toast adapter initialized (sandbox mode)")
	}

	// Build and run the HTTP server.
	router := api.NewRouter(ueAdapter, toastAdapter)
	log.Printf("PlateMate API listening on :%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Server: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
