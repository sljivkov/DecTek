// Package pricefeed provides price data structures and interfaces for the DecTek service
package pricefeed

// Price represents a token's price data
type Price struct {
	Symbol string  // Token symbol (e.g., "BTC", "ETH")
	USD    float64 // Price in USD
}

// PriceProvider defines the interface for services that provide price updates
type PriceProvider interface {
	// UpdatePriceFromApi continuously fetches and sends price updates to the provided channel
	UpdatePriceFromApi(priceCh chan<- []Price)
}
