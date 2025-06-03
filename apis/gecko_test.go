package apis

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sljivkov/dectek/config"
	"github.com/sljivkov/dectek/pricefeed"
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
}

func TestGetPrices(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify query parameters
		query := r.URL.Query()
		assert.Equal(t, "bitcoin,ethereum", query.Get("ids"))
		assert.Equal(t, "usd", query.Get("vs_currencies"))
		assert.Equal(t, "2", query.Get("precision"))

		// Return mock response
		response := map[string]CurrencyPrice{
			"bitcoin":  {USD: 30000.00},
			"ethereum": {USD: 2000.00},
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
	assert.Len(t, prices, 2)

	// Verify prices
	expectedPrices := map[string]float64{
		"bitcoin":  30000.00,
		"ethereum": 2000.00,
	}

	for _, price := range prices {
		assert.Equal(t, expectedPrices[price.Symbol], price.USD)
	}
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
			"bitcoin": {USD: 30000.00},
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
	priceCh := make(chan []pricefeed.Price, 1)

	// Start UpdatePriceFromApi in a goroutine
	go func() {
		gecko.UpdatePriceFromApi(priceCh)
	}()

	// Wait for the first price update
	select {
	case prices := <-priceCh:
		assert.Len(t, prices, 1)
		assert.Equal(t, "bitcoin", prices[0].Symbol)
		assert.Equal(t, 30000.00, prices[0].USD)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for price update")
	}
}
