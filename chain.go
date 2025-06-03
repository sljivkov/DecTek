package main

import (
	"context"
	"encoding/json"
	"net/http"
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

func pricesHandler(w http.ResponseWriter, r *http.Request) {
	// Wait until prices are ready
	select {
	case <-readyCh:
	case <-time.After(3 * time.Second): // fallback timeout
		http.Error(w, "prices not ready", http.StatusServiceUnavailable)

		return
	}

	mu.RLock()
	defer mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(apiPrices); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}
}
