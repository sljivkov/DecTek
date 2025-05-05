package chains

import (
	"context"
	"log"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/sljivkov/dectak/contract"
	"github.com/sljivkov/dectak/pricefeed"
)

type SepoliaPriceFeed struct {
	client          *ethclient.Client
	contract        *contract.Contract
	auth            *bind.TransactOpts
	contractAddress common.Address
	onChainPrices   map[string]float64
}

// Minimal ABI for Chainlink AggregatorV3Interface
const chainlinkABI = `[{"inputs":[],"name":"latestRoundData","outputs":[{"internalType":"uint80","name":"roundId","type":"uint80"},{"internalType":"int256","name":"answer","type":"int256"},{"internalType":"uint256","name":"startedAt","type":"uint256"},{"internalType":"uint256","name":"updatedAt","type":"uint256"},{"internalType":"uint80","name":"answeredInRound","type":"uint80"}],"stateMutability":"view","type":"function"}]`

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

	return &SepoliaPriceFeed{
		client:          client,
		contract:        contract,
		auth:            auth,
		contractAddress: addr,
		onChainPrices:   make(map[string]float64),
	}, nil
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
		defer close(out) // Optional: close the output channel when done

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
				// Send the result to the output channel
				out <- pricefeed.Price{
					Symbol: event.Symbol,
					USD:    priceFloat,
				}
			case <-ctx.Done():
				log.Println("🛑 Context cancelled, stopping listener")
				sub.Unsubscribe()
				return
			}
		}
	}()
}

func (s *SepoliaPriceFeed) updatePriceOnChain(_ context.Context, symbol string, price *big.Int) error {
	_, err := s.contract.Set(s.auth, symbol, price)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil

}

func (s *SepoliaPriceFeed) WritePricesToChain(ctx context.Context, in <-chan []pricefeed.Price) {
	for {
		select {
		case prices := <-in:
			log.Printf("📨 Incoming prices to chain writer: %+v\n", prices)
			for _, p := range prices {
				// Convert price to on-chain representation (e.g. scaled by 100)
				scaled := new(big.Int).Mul(
					big.NewInt(int64(p.USD*100)),
					big.NewInt(1),
				)

				log.Printf("📝 Writing %s: %.2f (scaled: %s) to chain\n", p.Symbol, p.USD, scaled.String())
				err := s.updatePriceOnChain(ctx, p.Symbol, scaled)

				if err != nil {

				}
			}
		case <-ctx.Done():
			log.Println("⛔ Price writing stopped due to context cancellation")
			return
		}
	}
}

func (s *SepoliaPriceFeed) GetChainlinkPrice(symbol string) (int64, error) {
	addresses := map[string]string{
		"bitcoin":  "0xA39434A63A52E749F02807ae27335515BA4b07F7",
		"ethereum": "0xD4a33860578De61DBAbDc8BFdb98FD742fA7028e",
	}

	addr := common.HexToAddress(addresses[symbol])
	parsedABI, err := abi.JSON(strings.NewReader(chainlinkABI))
	if err != nil {
		return 0, err
	}

	contract := bind.NewBoundContract(addr, parsedABI, s.client, s.client, s.client)

	var out []interface{}
	err = contract.Call(nil, &out, "latestRoundData")
	if err != nil {
		return 0, err
	}

	answer := out[1].(*big.Int)
	price := new(big.Int).Div(answer, big.NewInt(1e6)).Int64()

	log.Printf("🔗 Chainlink %s: %d", symbol, price)
	return price, nil
}

func (s *SepoliaPriceFeed) ValidateAndWrite(ctx context.Context, prices []pricefeed.Price) {
	for _, p := range prices {
		symbol := strings.ToLower(p.Symbol)
		newPrice := int64(p.USD * 100)

		chainlinkPrice, err := s.GetChainlinkPrice(symbol)
		if err != nil {
			log.Printf("⚠️ Chainlink fetch failed for %s: %v", symbol, err)
			continue
		}

		contractPrice := int64(s.onChainPrices[symbol] * 100)

		contractUp := int64(float64(contractPrice) * 1.02)
		contractDown := int64(float64(contractPrice) * 0.98)
		chainUp := int64(float64(chainlinkPrice) * 1.2)
		chainDown := int64(float64(chainlinkPrice) * 0.8)

		if (newPrice <= contractDown || newPrice >= contractUp || contractPrice == 0) &&
			(newPrice >= chainDown && newPrice <= chainUp) {

			log.Printf("✅ Writing %s to chain: %d", symbol, newPrice)
			// TODO: handle error reverting
			s.updatePriceOnChain(ctx, p.Symbol, big.NewInt(newPrice))

			// ✅ Update cache after successful write
			s.onChainPrices[symbol] = float64(newPrice) / 100

		} else {
			log.Printf("⛔ %s not valid for write. Skipped.", symbol)
		}
	}
}

func (sp *SepoliaPriceFeed) OnChainPrices() map[string]float64 {
	return sp.onChainPrices
}
