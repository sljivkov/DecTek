// Package main provides the entry point for the DecTek price feed service
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/sljivkov/dectek/pricefeed"
)

type AllFeed struct {
	apiFeed   pricefeed.PriceProvider
	chainFeed pricefeed.PriceFeed
}

func (af *AllFeed) ListenOnChainPriceUpdate(ctx context.Context, out chan<- pricefeed.Price) {
	af.chainFeed.ListenOnChainPriceUpdate(ctx, out)
}

func (af *AllFeed) WritePricesToChain(ctx context.Context, in <-chan []pricefeed.Price) {
	af.chainFeed.WritePricesToChain(ctx, in)
}

func (af *AllFeed) UpdatePriceFromApi(priceCh chan<- []pricefeed.Price) {
	af.apiFeed.UpdatePriceFromApi(priceCh)
}

func NewAllFeed(apiFeed pricefeed.PriceProvider, chainFeed pricefeed.PriceFeed) *AllFeed {
	return &AllFeed{
		apiFeed:   apiFeed,
		chainFeed: chainFeed,
	}
}

type priceCache struct {
	data    []byte
	expires time.Time
}

var cache = struct {
	sync.RWMutex
	prices priceCache
}{}

func pricesHandler(w http.ResponseWriter, r *http.Request) {
	// Wait until prices are ready
	select {
	case <-readyCh:
	case <-time.After(3 * time.Second): // fallback timeout
		http.Error(w, "prices not ready", http.StatusServiceUnavailable)

		return
	}

	// Check cache first
	cache.RLock()
	if !cache.prices.expires.IsZero() && time.Now().Before(cache.prices.expires) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(cache.prices.data)
		cache.RUnlock()

		return
	}
	cache.RUnlock()

	// Generate new response
	mu.RLock()
	data, err := json.Marshal(apiPrices)
	mu.RUnlock()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	// Update cache
	cache.Lock()
	cache.prices = priceCache{
		data:    data,
		expires: time.Now().Add(2 * time.Second), // Cache for 2 seconds
	}
	cache.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
