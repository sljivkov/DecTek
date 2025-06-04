// Package main provides the entry point for the DecTek price feed service
package main

import (
	"context"

	"github.com/sljivkov/dectek/domain"
)

type AllFeed struct {
	apiFeed   domain.PriceProvider
	chainFeed domain.PriceFeed
}

func (af *AllFeed) ListenOnChainPriceUpdate(ctx context.Context, out chan<- domain.Price) {
	af.chainFeed.ListenOnChainPriceUpdate(ctx, out)
}

func (af *AllFeed) WritePricesToChain(ctx context.Context, in <-chan []domain.Price) {
	af.chainFeed.WritePricesToChain(ctx, in)
}

func (af *AllFeed) UpdatePriceFromApi(priceCh chan<- []domain.Price) {
	af.apiFeed.UpdatePriceFromApi(priceCh)
}

func NewAllFeed(apiFeed domain.PriceProvider, chainFeed domain.PriceFeed) *AllFeed {
	return &AllFeed{
		apiFeed:   apiFeed,
		chainFeed: chainFeed,
	}
}
