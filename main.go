// Package main provides the entry point for the DecTek price feed service
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/sljivkov/dectek/apis"
	"github.com/sljivkov/dectek/chains"
	"github.com/sljivkov/dectek/config"
	"github.com/sljivkov/dectek/domain"
)

// Global state variables for price management
var (
	apiPrices = make(map[string]map[string]float64) // symbol -> currency type -> amount
	mu        sync.RWMutex
)

func main() {
	// Create config with options
	cfg, err := config.NewConfig(
		config.WithEnvFile(".env"),
		config.WithPrecision("6"),
	)
	if err != nil {
		log.Fatalf("❌ Failed to initialize config: %v", err)
	}

	sepoliaFeed, err := chains.NewSepoliaPriceFeed(cfg.PrivateKey, cfg.Alchemy, cfg.Contract)
	if err != nil {
		log.Fatalf("❌ Failed to initialize Sepolia feed: %v", err)
	}

	// Initialize chainlink pricer
	chainlinkPricer, err := chains.NewRealChainlinkPricer(sepoliaFeed.Client())
	if err != nil {
		log.Fatalf("❌ Failed to initialize Chainlink pricer: %v", err)
	}

	sepoliaFeed.SetChainlinkPricer(chainlinkPricer)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		geckoFeed = apis.NewCoinGecko(*cfg)
		allFeed   = NewAllFeed(geckoFeed, sepoliaFeed)
		out       = make(chan domain.Price, 100)   // Buffer for on-chain price updates
		priceCh   = make(chan []domain.Price, 100) // Buffer for API price updates
	)

	// Start on-chain price listener
	go allFeed.ListenOnChainPriceUpdate(ctx, out)

	// Process on-chain price updates
	go func() {
		for data := range out {
			log.Printf("📥 Received on-chain price update: %s %s = %.2f", data.Symbol, data.Type, data.Amount)
		}
	}()

	// Start API price updater
	go allFeed.UpdatePriceFromApi(priceCh)

	// Start chain price writer
	go allFeed.WritePricesToChain(ctx, priceCh)

	// Start HTTP server
	go func() {
		log.Println("🚀 Starting server on :8080")

		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("❌ Server failed: %v", err)
		}
	}()

	// Register HTTP handlers
	http.HandleFunc("/prices", pricesHandler)
	http.HandleFunc("/set-price", setPriceHandler)

	// Main price update loop
	log.Println("✨ Starting main price update loop")

	for data := range priceCh {
		mu.Lock()
		for _, coin := range data {
			if _, exists := apiPrices[coin.Symbol]; !exists {
				apiPrices[coin.Symbol] = make(map[string]float64)
			}
			apiPrices[coin.Symbol][coin.Type] = coin.Amount
			log.Printf("💰 Updated %s %s price: %.2f", coin.Symbol, coin.Type, coin.Amount)
		}
		mu.Unlock()
	}
}

// setCORSHeaders sets the CORS headers for the response
func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// pricesHandler handles the /prices HTTP endpoint
func pricesHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	// Handle preflight OPTIONS request
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Set content type before writing response
	w.Header().Set("Content-Type", "application/json")

	// Convert prices map to slice
	var prices []domain.Price
	mu.RLock()
	for symbol, currencies := range apiPrices {
		for currencyType, amount := range currencies {
			prices = append(prices, domain.Price{
				Symbol: symbol,
				Amount: amount,
				Type:   currencyType,
			})
		}
	}
	mu.RUnlock()

	// Initialize empty array if no prices
	if prices == nil {
		prices = make([]domain.Price, 0)
	}

	// Respond with JSON
	if err := json.NewEncoder(w).Encode(prices); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// setPriceHandler accepts manual price updates
func setPriceHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	// Handle preflight OPTIONS request
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var update struct {
		Symbol string  `json:"symbol"`
		Amount float64 `json:"amount"`
		Type   string  `json:"type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if update.Symbol == "" || update.Amount <= 0 || update.Type == "" {
		http.Error(w, "Invalid price data", http.StatusBadRequest)
		return
	}

	// Validate currency type (only USD and EUR supported)
	if update.Type != "USD" && update.Type != "EUR" {
		http.Error(w, "Invalid currency type", http.StatusBadRequest)
		return
	}

	// Check if symbol exists in our token list
	validSymbols := map[string]bool{
		"bitcoin":  true,
		"ethereum": true,
	}
	if !validSymbols[strings.ToLower(update.Symbol)] {
		http.Error(w, "Unknown symbol", http.StatusNotFound)
		return
	}

	mu.Lock()
	if _, exists := apiPrices[update.Symbol]; !exists {
		apiPrices[update.Symbol] = make(map[string]float64)
	}
	apiPrices[update.Symbol][update.Type] = update.Amount
	mu.Unlock()

	log.Printf("🔧 Manually set %s %s price: %.2f", update.Symbol, update.Type, update.Amount)
	w.WriteHeader(http.StatusOK)
}
