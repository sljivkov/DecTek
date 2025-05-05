package apis

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/sljivkov/dectak/config"
	"github.com/sljivkov/dectak/pricefeed"
)

type CoinGecko struct {
	cfg       config.Config
	apiPrices map[string]float64
}

func NewCoinGecko(cfg config.Config) *CoinGecko {
	return &CoinGecko{
		cfg:       cfg,
		apiPrices: make(map[string]float64),
	}
}

type CurrencyPrice struct {
	USD float64 `json:"usd"`
}

func (g *CoinGecko) getPrices() ([]pricefeed.Price, error) {
	params := url.Values{}
	params.Add("ids", g.cfg.Tokens) // e.g., "bitcoin,ethereum"
	params.Add("vs_currencies", "usd")
	params.Add("precision", g.cfg.Precision)
	params.Add("include_last_update_at", "true")

	fullURL := fmt.Sprintf("%s?%s", g.cfg.Url, params.Encode())
	resp, err := http.Get(fullURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Use map since token list is dynamic
	var raw map[string]CurrencyPrice
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	var prices []pricefeed.Price
	for symbol, data := range raw {
		g.apiPrices[symbol] = data.USD
		prices = append(prices, pricefeed.Price{
			Symbol: symbol,
			USD:    data.USD,
		})
	}

	return prices, nil
}

func (g *CoinGecko) ApiPrices() map[string]float64 {
	return g.apiPrices
}

func (g *CoinGecko) UpdatePriceFromApi(priceCh chan<- []pricefeed.Price) {
	for {
		data, err := g.getPrices()
		if err != nil {
			log.Println("Error fetching prices:", err)
		} else {
			priceCh <- data
		}
		time.Sleep(61 * time.Second)
	}
}
