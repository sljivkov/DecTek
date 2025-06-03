// Package pricefeed provides price data structures and interfaces for the DecTek service
package pricefeed

import (
	"context"
)

// PriceFeed defines the interface for blockchain price feed operations
type PriceFeed interface {
	// OnChainPrices returns the current prices stored on the blockchain
	OnChainPrices() map[string]float64

	// ListenOnChainPriceUpdate listens for price updates on the blockchain and
	// sends them to the provided channel
	ListenOnChainPriceUpdate(ctx context.Context, out chan<- Price)

	// WritePricesToChain writes price updates received from the channel to the blockchain
	WritePricesToChain(ctx context.Context, in <-chan []Price)
}
