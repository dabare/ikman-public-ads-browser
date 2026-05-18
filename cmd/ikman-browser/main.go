package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"ikmanbrowser/internal/calls"
	"ikmanbrowser/internal/ikman"
	"ikmanbrowser/internal/web"
)

func main() {
	port := getenv("PORT", "8080")
	baseURL := getenv("IKMAN_BASE_URL", "https://ikman.lk")
	interval := getDuration("IKMAN_REQUEST_INTERVAL", 200*time.Millisecond)

	client := ikman.NewClient(ikman.Config{
		BaseURL:         baseURL,
		RequestInterval: interval,
		UserAgent:       "ikmanbrowser/0.1 (+local user-driven viewer)",
	})
	callStore, err := calls.Open(getenv("IKMAN_CALL_DB", filepath.Join("data", "calls.json")))
	if err != nil {
		log.Fatalf("open call database: %v", err)
	}
	server := web.NewServer(client, web.Config{
		LoadPhonesByDefault: getBool("IKMAN_LOAD_PHONES", true),
		CallStore:           callStore,
	})

	addr := ":" + port
	log.Printf("listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, server.Routes()))
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	switch os.Getenv(key) {
	case "1", "true", "TRUE", "yes", "YES", "on", "ON":
		return true
	case "0", "false", "FALSE", "no", "NO", "off", "OFF":
		return false
	default:
		return fallback
	}
}

func getDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		log.Printf("invalid %s=%q, using %s", key, raw, fallback)
		return fallback
	}
	return value
}
