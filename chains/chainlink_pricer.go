// Package chains provides blockchain interaction implementations
package chains

import (
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// RealChainlinkPricer implements ChainlinkPricer interface using real Chainlink price feeds
type RealChainlinkPricer struct {
	client *ethclient.Client
}

// NewRealChainlinkPricer creates a new instance of RealChainlinkPricer
func NewRealChainlinkPricer(client *ethclient.Client) *RealChainlinkPricer {
	return &RealChainlinkPricer{client: client}
}

//nolint:lll
const chainlinkABI = `[{"inputs":[],"name":"latestRoundData","outputs":[{"internalType":"uint80","name":"roundId","type":"uint80"},{"internalType":"int256","name":"answer","type":"int256"},{"internalType":"uint256","name":"startedAt","type":"uint256"},{"internalType":"uint256","name":"updatedAt","type":"uint256"},{"internalType":"uint80","name":"answeredInRound","type":"uint80"}],"stateMutability":"view","type":"function"}]`

// getChainlinkPrice fetches the latest price for a given token from Chainlink price feeds
func (r *RealChainlinkPricer) getChainlinkPrice(symbol string) (int64, error) {
	addresses := map[string]string{
		"bitcoin":  "0xA39434A63A52E749F02807ae27335515BA4b07F7",
		"ethereum": "0xD4a33860578De61DBAbDc8BFdb98FD742fA7028e",
	}

	addr, ok := addresses[symbol]
	if !ok {
		return 0, fmt.Errorf("no Chainlink price feed available for %s", symbol)
	}

	contractAddr := common.HexToAddress(addr)

	parsedABI, err := abi.JSON(strings.NewReader(chainlinkABI))
	if err != nil {
		return 0, fmt.Errorf("failed to parse Chainlink ABI: %w", err)
	}

	contract := bind.NewBoundContract(contractAddr, parsedABI, r.client, r.client, r.client)

	var out []any
	if err := contract.Call(nil, &out, "latestRoundData"); err != nil {
		return 0, fmt.Errorf("failed to fetch Chainlink price data: %w", err)
	}

	answer, ok := out[1].(*big.Int)
	if !ok || answer == nil {
		return 0, fmt.Errorf("invalid price data received from Chainlink")
	}

	price := new(big.Int).Div(answer, big.NewInt(1e6)).Int64()
	log.Printf("🔗 Chainlink %s: %d", symbol, price)

	return price, nil
}
