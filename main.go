package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"

	"github.com/sljivkov/dectak/apis"
	"github.com/sljivkov/dectak/chains"
	"github.com/sljivkov/dectak/config"
	"github.com/sljivkov/dectak/pricefeed"
)

var (
	apiPrices = make(map[string]float64)
	mu        sync.RWMutex
	readyCh   = make(chan struct{}) // closed once prices are loaded
	once      sync.Once
)

// embed:go Dectek.abi
var contractAbi []byte

func main() {
	// load environment variables
	if err := godotenv.Load(); err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	var cfg config.Config
	if err := envconfig.Process("", &cfg); err != nil {
		log.Fatalf("failed to process cfg: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		out     = make(chan pricefeed.Price)
		priceCh = make(chan []pricefeed.Price)
	)

	sepoliaFeed, _ := chains.NewSepoliaPriceFeed(cfg.PrivateKey, cfg.Alchemy, cfg.Contract)

	// change this to api not gecko
	geckoFeed := apis.NewCoinGecko(cfg)

	allFeed := NewAllFeed(geckoFeed, sepoliaFeed)

	go allFeed.ListenOnChainPriceUpdate(ctx, out)

	// Listen to processed data
	go func() {
		for data := range out {
			log.Printf("📥 Received on-chain price update: %s = %.2f", data.Symbol, data.USD)
		}
	}()

	go allFeed.UpdatePriceFromApi(priceCh)

	go allFeed.WritePricesToChain(ctx, priceCh)

	// TODO add api calls for reverts and implement revert logic

	// Start web server
	http.HandleFunc("/prices", pricesHandler)
	go func() {
		log.Println("Starting server on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Main loop updates shared map
	for data := range priceCh {
		mu.Lock()
		for _, coin := range data {
			apiPrices[coin.Symbol] = coin.USD
			fmt.Printf("%s %f \n", coin.Symbol, coin.USD)
		}
		mu.Unlock()
	}
}
