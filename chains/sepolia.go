package chains

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"

	"github.com/sljivkov/dectek/contract"
	"github.com/sljivkov/dectek/pricefeed"
)

// ContractInterface defines the interface needed for price feed operations.
type ContractInterface interface {
	WatchPriceChanged(opts *bind.WatchOpts, sink chan<- *contract.ContractPriceChanged) (event.Subscription, error)
	Set(opts *bind.TransactOpts, symbol string, price *big.Int) (*types.Transaction, error)
	Get(opts *bind.CallOpts, symbol string) (*big.Int, error)
}

// ChainlinkPricer allows mocking getChainlinkPrice.
type ChainlinkPricer interface {
	getChainlinkPrice(symbol string) (int64, error)
}

type SepoliaPriceFeed struct {
	client          *ethclient.Client
	contract        ContractInterface
	auth            *bind.TransactOpts
	contractAddress common.Address
	onChainPrices   map[string]float64
	chainlinkPricer ChainlinkPricer
	// Add cache fields
	boundCache   map[string]boundsCacheEntry
	boundCacheMu sync.RWMutex
}

type boundsCacheEntry struct {
	up        int64
	down      int64
	timestamp time.Time
}

func NewSepoliaPriceFeed(privateKey string, rpcURL string, contractAddress string) (*SepoliaPriceFeed, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}

	ecdsaKey, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return nil, err
	}

	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		return nil, err
	}

	auth, err := bind.NewKeyedTransactorWithChainID(ecdsaKey, chainID)
	if err != nil {
		return nil, err
	}

	addr := common.HexToAddress(contractAddress)

	contract, err := contract.NewContract(addr, client)
	if err != nil {
		return nil, err
	}

	feed := &SepoliaPriceFeed{
		client:          client,
		contract:        contract,
		auth:            auth,
		contractAddress: addr,
		onChainPrices:   make(map[string]float64),
		boundCache:      make(map[string]boundsCacheEntry),
	}
	// chainlinkPricer will be set by the caller
	return feed, nil
}

func (s *SepoliaPriceFeed) ListenOnChainPriceUpdate(ctx context.Context, out chan<- pricefeed.Price) {
	logs := make(chan *contract.ContractPriceChanged)

	sub, err := s.contract.WatchPriceChanged(&bind.WatchOpts{
		Context: ctx,
	}, logs)
	if err != nil {
		log.Fatalf("Failed to subscribe to event: %v", err)
	}

	log.Println("📡 Listening for PriceChanged events...")

	go func() {
		defer sub.Unsubscribe() // Always cleanup subscription

		defer close(out) // Always close output channel when done

		for {
			select {
			case err := <-sub.Err():
				log.Printf("🔴 Subscription error: %v", err)

				return
			case event := <-logs:
				log.Printf("🔥 Event:\n  Symbol: %s\n  Price: %d\n  Timestamp: %d\n",
					event.Symbol, event.NewPrice, event.Timestamp.Uint64())

				priceFloat, _ := new(big.Float).SetInt(
					new(big.Int).Div(event.NewPrice, big.NewInt(100)),
				).Float64()

				s.onChainPrices[event.Symbol] = priceFloat

				out <- pricefeed.Price{
					Symbol: event.Symbol,
					USD:    priceFloat,
				}
			case <-ctx.Done():
				log.Println("🛑 Context canceled, stopping listener")

				return
			}
		}
	}()
}

func (s *SepoliaPriceFeed) getChainlinkBounds(symbol string, chainlinkPrice int64) (int64, int64, error) {
	s.boundCacheMu.RLock()
	if entry, exists := s.boundCache[symbol]; exists {
		if time.Since(entry.timestamp) < 5*time.Minute {
			s.boundCacheMu.RUnlock()
			return entry.up, entry.down, nil
		}
	}
	s.boundCacheMu.RUnlock()

	// Calculate new bounds
	chainlinkScaled := chainlinkPrice * 100
	up := int64(float64(chainlinkScaled) * 1.2)   // 20% up
	down := int64(float64(chainlinkScaled) * 0.8) // 20% down

	// Update cache
	s.boundCacheMu.Lock()
	s.boundCache[symbol] = boundsCacheEntry{
		up:        up,
		down:      down,
		timestamp: time.Now(),
	}
	s.boundCacheMu.Unlock()

	return up, down, nil
}

func (s *SepoliaPriceFeed) validatePrice(symbol string, newPrice int64) (bool, error) {
	chainlinkPrice, err := s.chainlinkPricer.getChainlinkPrice(symbol)
	if err != nil {
		return false, fmt.Errorf("chainlink fetch failed for %s: %w", symbol, err)
	}

	contractPrice := int64(s.onChainPrices[symbol] * 100)

	// If no contract price exists, only check chainlink bounds
	if contractPrice == 0 {
		chainUp, chainDown, err := s.getChainlinkBounds(symbol, chainlinkPrice)
		if err != nil {
			return false, err
		}
		return newPrice >= chainDown && newPrice <= chainUp, nil
	}

	// Check if price is within 2% of contract price
	contractUp := int64(float64(contractPrice) * 1.02)   // 2% up
	contractDown := int64(float64(contractPrice) * 0.98) // 2% down
	withinContractBounds := newPrice >= contractDown && newPrice <= contractUp

	// If price is within 2% bounds, don't write
	if withinContractBounds {
		return false, nil
	}

	// If price is outside contract bounds, verify it's within chainlink bounds
	chainUp, chainDown, err := s.getChainlinkBounds(symbol, chainlinkPrice)
	if err != nil {
		return false, err
	}
	withinChainlinkBounds := newPrice >= chainDown && newPrice <= chainUp

	return withinChainlinkBounds, nil
}

func (s *SepoliaPriceFeed) writeToChain(_ context.Context, symbol string, price float64) error {
	newPrice := big.NewInt(int64(price * 100))

	_, err := s.contract.Set(s.auth, symbol, newPrice)
	if err != nil {
		return fmt.Errorf("failed to write %s price: %w", symbol, err)
	}

	// Update cache after successful write
	s.onChainPrices[symbol] = price

	return nil
}

func (s *SepoliaPriceFeed) WritePricesToChain(ctx context.Context, in <-chan []pricefeed.Price) {
	for {
		select {
		case prices := <-in:
			log.Printf("📨 Incoming prices to chain writer: %+v\n", prices)

			// Pre-allocate slice with capacity matching input
			validPrices := make([]pricefeed.Price, 0, len(prices))

			for _, price := range prices {
				symbol := strings.ToLower(price.Symbol)
				newPrice := int64(price.USD * 100)

				shouldWrite, err := s.validatePrice(symbol, newPrice)
				if err != nil {
					log.Printf("⚠️ %v", err)
					continue
				}

				if shouldWrite {
					log.Printf("✅ Price validated for %s: %.2f", symbol, price.USD)
					validPrices = append(validPrices, price)
				} else {
					log.Printf("⛔ %s price invalid for write: %.2f", symbol, price.USD)
				}
			}

			// Batch write valid prices in a single transaction if possible
			for _, price := range validPrices {
				symbol := strings.ToLower(price.Symbol)
				if err := s.writeToChain(ctx, symbol, price.USD); err != nil {
					log.Printf("❌ %v", err)
					continue
				}
				log.Printf("📝 Successfully wrote %s price: %.2f", symbol, price.USD)
			}

		case <-ctx.Done():
			log.Println("⛔ Price writing stopped due to context cancellation")
			return
		}
	}
}

func (sp *SepoliaPriceFeed) OnChainPrices() map[string]float64 {
	return sp.onChainPrices
}
