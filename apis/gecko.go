// Package apis provides external price feed integrations
package apis

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sljivkov/dectek/config"
	"github.com/sljivkov/dectek/domain"
)

// CoinGecko implements domain.PriceProvider interface using the CoinGecko API
type CoinGecko struct {
	cfg       config.Config
	apiPrices map[string]float64
	client    *http.Client
}

// CurrencyPrice represents the price response structure from CoinGecko
type CurrencyPrice map[string]float64 // currency type to amount mapping

// NewCoinGecko creates a new CoinGecko price feed instance
func NewCoinGecko(cfg config.Config) *CoinGecko {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}

	return &CoinGecko{
		cfg:       cfg,
		apiPrices: make(map[string]float64),
		client: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
		},
	}
}

// getPrices fetches current prices from the CoinGecko API
func (g *CoinGecko) getPrices() ([]domain.Price, error) {
	params := url.Values{}
	params.Add("ids", g.cfg.Tokens)
	params.Add("vs_currencies", "usd,eur") // Request multiple currencies
	params.Add("precision", g.cfg.Precision)
	params.Add("include_last_update_at", "true")

	fullURL := fmt.Sprintf("%s?%s", g.cfg.Url, params.Encode())

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add compression and caching headers
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Cache-Control", "max-age=30") // Allow 30s cache
	req.Header.Set("Accept", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch prices: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned non-200 status: %d", resp.StatusCode)
	}

	var reader io.ReadCloser = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		var err error
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer reader.Close()
	}

	var raw map[string]CurrencyPrice
	if err := json.NewDecoder(reader).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	prices := make([]domain.Price, 0)

	for symbol, currencies := range raw {
		for currencyType, amount := range currencies {
			prices = append(prices, domain.Price{
				Symbol: symbol,
				Amount: amount,
				Type:   strings.ToUpper(currencyType),
			})
		}
		// Store USD price for backward compatibility
		if usdPrice, ok := currencies["usd"]; ok {
			g.apiPrices[symbol] = usdPrice
		}
	}

	return prices, nil
}

// ApiPrices returns the current cached API prices
func (g *CoinGecko) ApiPrices() map[string]float64 {
	return g.apiPrices
}

// UpdatePriceFromApi continuously updates prices from the CoinGecko API
func (g *CoinGecko) UpdatePriceFromApi(priceCh chan<- []domain.Price) {
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
