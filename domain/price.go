// Package domain defines core interfaces and types for the DecTek service
package domain

import (
	"context"

	"github.com/ethereum/go-ethereum/ethclient"
)

// Price represents a token's price data
type Price struct {
	Symbol string  // Token symbol (e.g., "BTC", "ETH")
	Amount float64 // Amount of the token
	Type   string  // Type of currency (e.g., "USD", "EUR")
}

// PriceProvider defines the interface for services that provide price updates
type PriceProvider interface {
	// UpdatePriceFromApi continuously fetches and sends price updates to the provided channel
	UpdatePriceFromApi(priceCh chan<- []Price)
}

// ChainlinkPricer allows fetching price data from Chainlink feeds
type ChainlinkPricer interface {
	// GetChainlinkPrice fetches the latest price for a given token from Chainlink price feeds
	GetChainlinkPrice(symbol string) (int64, error)
}

// PriceFeed defines the interface for blockchain price feed operations
type PriceFeed interface {
	// OnChainPrices returns the current prices stored on the blockchain
	OnChainPrices() map[string]float64

	// ListenOnChainPriceUpdate listens for price updates on the blockchain and
	// sends them to the provided channel
	ListenOnChainPriceUpdate(ctx context.Context, out chan<- Price)

	// WritePricesToChain writes price updates received from the channel to the blockchain
	WritePricesToChain(ctx context.Context, in <-chan []Price)

	// SetChainlinkPricer sets the ChainlinkPricer for price validation
	SetChainlinkPricer(pricer ChainlinkPricer)

	// Client returns the underlying Ethereum client
	Client() *ethclient.Client
}
