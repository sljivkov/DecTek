// Package main provides the entry point for the DecTek price feed service
package main

import (
	"context"
	_ "embed"
	"log"
	"net/http"
	"sync"

	"github.com/sljivkov/dectek/apis"
	"github.com/sljivkov/dectek/chains"
	"github.com/sljivkov/dectek/config"
	"github.com/sljivkov/dectek/domain"
)

// Global state variables for price management
var (
	apiPrices = make(map[string]float64)
	mu        sync.RWMutex
	readyCh   = make(chan struct{}) // Signals when initial prices are loaded
)

func main() {
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("❌ Failed to initialize config: %v", err)
	}

	sepoliaFeed, err := chains.NewSepoliaPriceFeed(cfg.PrivateKey, cfg.Alchemy, cfg.Contract)
	if err != nil {
		log.Fatalf("❌ Failed to initialize Sepolia feed: %v", err)

		return
	}

	// Initialize chainlink pricer
	chainlinkPricer, err := chains.NewRealChainlinkPricer(sepoliaFeed.Client())
	if err != nil {
		log.Fatalf("❌ Failed to initialize Chainlink pricer: %v", err)

		return
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
			log.Printf("📥 Received on-chain price update: %s = %.2f", data.Symbol, data.USD)
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

	// Main price update loop
	log.Println("✨ Starting main price update loop")

	for data := range priceCh {
		mu.Lock()

		for _, coin := range data {
			apiPrices[coin.Symbol] = coin.USD

			log.Printf("💰 Updated %s price: %.2f", coin.Symbol, coin.USD)
		}

		mu.Unlock()
	}
}
