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
	"github.com/sljivkov/dectek/pricefeed"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	geckoFeed := apis.NewCoinGecko(*cfg)

	allFeed := NewAllFeed(geckoFeed, sepoliaFeed)

	// Initialize channels for price data flow
	var (
		out     = make(chan pricefeed.Price)
		priceCh = make(chan []pricefeed.Price)
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
