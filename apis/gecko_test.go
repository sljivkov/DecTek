package apis

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sljivkov/dectek/config"
	"github.com/sljivkov/dectek/domain"
)

func TestNewCoinGecko(t *testing.T) {
	cfg := config.Config{
		Tokens:    "bitcoin,ethereum",
		Precision: "2",
		Url:       "http://test.com",
	}

	gecko := NewCoinGecko(cfg)
	assert.NotNil(t, gecko)
	assert.Equal(t, cfg, gecko.cfg)
	assert.NotNil(t, gecko.apiPrices)

	// Test HTTP client configuration
	transport, ok := gecko.client.Transport.(*http.Transport)
	assert.True(t, ok, "Expected *http.Transport")
	assert.Equal(t, 100, transport.MaxIdleConns)
	assert.Equal(t, 100, transport.MaxIdleConnsPerHost)
	assert.Equal(t, 90*time.Second, transport.IdleConnTimeout)
	assert.Equal(t, 10*time.Second, gecko.client.Timeout)
}

func TestGetPrices(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify query parameters
		query := r.URL.Query()
		assert.Equal(t, "bitcoin,ethereum", query.Get("ids"))
		assert.Equal(t, "usd,eur", query.Get("vs_currencies"))
		assert.Equal(t, "2", query.Get("precision"))

		// Return mock response
		response := map[string]CurrencyPrice{
			"bitcoin": {
				"usd": 30000.00,
				"eur": 28000.00,
			},
			"ethereum": {
				"usd": 2000.00,
				"eur": 1850.00,
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := config.Config{
		Tokens:    "bitcoin,ethereum",
		Precision: "2",
		Url:       server.URL,
	}

	gecko := NewCoinGecko(cfg)
	prices, err := gecko.getPrices()

	assert.NoError(t, err)
	assert.Len(t, prices, 4) // 2 coins * 2 currencies

	// Create map for easy lookup
	priceMap := make(map[string]map[string]float64)
	for _, price := range prices {
		if _, exists := priceMap[price.Symbol]; !exists {
			priceMap[price.Symbol] = make(map[string]float64)
		}
		priceMap[price.Symbol][price.Type] = price.Amount
	}

	// Verify prices
	assert.Equal(t, 30000.00, priceMap["bitcoin"]["USD"])
	assert.Equal(t, 28000.00, priceMap["bitcoin"]["EUR"])
	assert.Equal(t, 2000.00, priceMap["ethereum"]["USD"])
	assert.Equal(t, 1850.00, priceMap["ethereum"]["EUR"])

	// Verify USD prices are stored in apiPrices
	assert.Equal(t, 30000.00, gecko.apiPrices["bitcoin"])
	assert.Equal(t, 2000.00, gecko.apiPrices["ethereum"])
}

func TestApiPrices(t *testing.T) {
	cfg := config.Config{}
	gecko := NewCoinGecko(cfg)

	// Set some test prices
	gecko.apiPrices = map[string]float64{
		"bitcoin":  30000.00,
		"ethereum": 2000.00,
	}

	prices := gecko.ApiPrices()
	assert.Equal(t, gecko.apiPrices, prices)
}

func TestUpdatePriceFromApi(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]CurrencyPrice{
			"bitcoin": {
				"usd": 30000.00,
				"eur": 28000.00,
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := config.Config{
		Tokens:    "bitcoin",
		Precision: "2",
		Url:       server.URL,
	}

	gecko := NewCoinGecko(cfg)
	priceCh := make(chan []domain.Price, 1)

	// Start UpdatePriceFromApi in a goroutine
	go func() {
		gecko.UpdatePriceFromApi(priceCh)
	}()

	// Wait for the first price update
	select {
	case prices := <-priceCh:
		assert.Len(t, prices, 2) // Both USD and EUR
		foundUSD := false
		foundEUR := false
		for _, price := range prices {
			assert.Equal(t, "bitcoin", price.Symbol)
			switch price.Type {
			case "USD":
				assert.Equal(t, 30000.00, price.Amount)
				foundUSD = true
			case "EUR":
				assert.Equal(t, 28000.00, price.Amount)
				foundEUR = true
			}
		}
		assert.True(t, foundUSD, "USD price not found")
		assert.True(t, foundEUR, "EUR price not found")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for price update")
	}
}
