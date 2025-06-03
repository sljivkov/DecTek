// Package chains provides blockchain interaction implementations
package chains

import (
	_ "embed"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

//go:embed abi/chainlink.abi
var chainlinkABI string

// RealChainlinkPricer implements ChainlinkPricer interface using real Chainlink price feeds
type RealChainlinkPricer struct {
	client    *ethclient.Client
	parsedABI abi.ABI
	addresses map[string]string
}

// NewRealChainlinkPricer creates a new instance of RealChainlinkPricer
func NewRealChainlinkPricer(client *ethclient.Client) (*RealChainlinkPricer, error) {
	parsedABI, err := abi.JSON(strings.NewReader(chainlinkABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse Chainlink ABI: %w", err)
	}

	// Initialize with default Sepolia addresses
	addresses := map[string]string{
		"bitcoin":  "0xA39434A63A52E749F02807ae27335515BA4b07F7",
		"ethereum": "0xD4a33860578De61DBAbDc8BFdb98FD742fA7028e",
	}

	return &RealChainlinkPricer{
		client:    client,
		parsedABI: parsedABI,
		addresses: addresses,
	}, nil
}

// SetAddresses allows updating the Chainlink price feed addresses
func (r *RealChainlinkPricer) SetAddresses(addresses map[string]string) {
	r.addresses = addresses
}

// getChainlinkPrice fetches the latest price for a given token from Chainlink price feeds
func (r *RealChainlinkPricer) getChainlinkPrice(symbol string) (int64, error) {
	addr, ok := r.addresses[symbol]
	if !ok {
		return 0, fmt.Errorf("no Chainlink price feed available for %s", symbol)
	}

	contractAddr := common.HexToAddress(addr)
	contract := bind.NewBoundContract(contractAddr, r.parsedABI, r.client, r.client, r.client)

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
