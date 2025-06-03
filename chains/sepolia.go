package chains

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"

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

func (s *SepoliaPriceFeed) validatePrice(symbol string, newPrice int64) (bool, error) {
	chainlinkPrice, err := s.chainlinkPricer.getChainlinkPrice(symbol)
	if err != nil {
		return false, fmt.Errorf("chainlink fetch failed for %s: %w", symbol, err)
	}

	contractPrice := int64(s.onChainPrices[symbol] * 100)
	chainlinkScaled := chainlinkPrice * 100

	// If no contract price exists, only check chainlink bounds
	if contractPrice == 0 {
		chainUp := int64(float64(chainlinkScaled) * 1.2)   // 20% up
		chainDown := int64(float64(chainlinkScaled) * 0.8) // 20% down

		return newPrice >= chainDown && newPrice <= chainUp, nil
	}

	// First check if price is within 2% of contract price
	contractUp := int64(float64(contractPrice) * 1.02)   // 2% up
	contractDown := int64(float64(contractPrice) * 0.98) // 2% down
	withinContractBounds := newPrice >= contractDown && newPrice <= contractUp

	// If price is within 2% bounds, don't write (avoid unnecessary updates)
	if withinContractBounds {
		return false, nil
	}

	// If price is outside contract bounds, verify it's within chainlink bounds
	chainUp := int64(float64(chainlinkScaled) * 1.2)   // 20% up
	chainDown := int64(float64(chainlinkScaled) * 0.8) // 20% down
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

			// First validate all prices
			validPrices := make([]pricefeed.Price, 0)

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

			// Then write valid prices
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
