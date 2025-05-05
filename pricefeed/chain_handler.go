package pricefeed

import (
	"context"
)

type PriceFeed interface {
	OnChainPrices() map[string]float64
	ListenOnChainPriceUpdate(ctx context.Context, out chan<- Price)
	WritePricesToChain(ctx context.Context, in <-chan []Price)
}
