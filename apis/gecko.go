// Package apis provides external price feed integrations
package apis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/sljivkov/dectek/config"
	"github.com/sljivkov/dectek/pricefeed"
)

// CoinGecko implements a price feed using the CoinGecko API
type CoinGecko struct {
	cfg       config.Config
	apiPrices map[string]float64
	client    *http.Client
}

// CurrencyPrice represents the price response structure from CoinGecko
type CurrencyPrice struct {
	USD float64 `json:"usd"`
}

// NewCoinGecko creates a new CoinGecko price feed instance
func NewCoinGecko(cfg config.Config) *CoinGecko {
	return &CoinGecko{
		cfg:       cfg,
		apiPrices: make(map[string]float64),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// getPrices fetches current prices from the CoinGecko API
func (g *CoinGecko) getPrices() ([]pricefeed.Price, error) {
	params := url.Values{}
	params.Add("ids", g.cfg.Tokens)
	params.Add("vs_currencies", "usd")
	params.Add("precision", g.cfg.Precision)
	params.Add("include_last_update_at", "true")

	fullURL := fmt.Sprintf("%s?%s", g.cfg.Url, params.Encode())

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch prices: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned non-200 status: %d", resp.StatusCode)
	}

	var raw map[string]CurrencyPrice
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	prices := make([]pricefeed.Price, 0, len(raw))

	for symbol, data := range raw {
		g.apiPrices[symbol] = data.USD
		prices = append(prices, pricefeed.Price{
			Symbol: symbol,
			USD:    data.USD,
		})
	}

	return prices, nil
}

// ApiPrices returns the current cached API prices
func (g *CoinGecko) ApiPrices() map[string]float64 {
	return g.apiPrices
}

// UpdatePriceFromApi continuously updates prices from the CoinGecko API
func (g *CoinGecko) UpdatePriceFromApi(priceCh chan<- []pricefeed.Price) {
	log.Println("📡 Starting CoinGecko price update service")

	for {
		data, err := g.getPrices()
		if err != nil {
			log.Printf("❌ Error fetching prices: %v", err)
		} else {
			log.Printf("✅ Successfully fetched %d prices from CoinGecko", len(data))

			priceCh <- data
		}

		// Rate limit as per CoinGecko's requirements
		time.Sleep(61 * time.Second)
	}
}
