package chains

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/sljivkov/dectek/contract"
	"github.com/sljivkov/dectek/pricefeed"
)

// MockContract implements the ContractInterface for testing
type MockContract struct {
	mock.Mock
}

//nolint:lll
func (m *MockContract) WatchPriceChanged(opts *bind.WatchOpts, sink chan<- *contract.ContractPriceChanged) (event.Subscription, error) {
	args := m.Called(opts, sink)

	return args.Get(0).(event.Subscription), args.Error(1)
}

func (m *MockContract) Set(opts *bind.TransactOpts, symbol string, price *big.Int) (*types.Transaction, error) {
	args := m.Called(opts, symbol, price)

	return args.Get(0).(*types.Transaction), args.Error(1)
}

func (m *MockContract) Get(opts *bind.CallOpts, symbol string) (*big.Int, error) {
	args := m.Called(opts, symbol)

	return args.Get(0).(*big.Int), args.Error(1)
}

// MockSubscription implements event.Subscription
type MockSubscription struct {
	mock.Mock
}

func (m *MockSubscription) Unsubscribe() {
	m.Called()
}

func (m *MockSubscription) Err() <-chan error {
	args := m.Called()

	return args.Get(0).(chan error)
}

func TestListenOnChainPriceUpdate(t *testing.T) {
	mockContract := new(MockContract)
	mockSub := new(MockSubscription)

	feed := &SepoliaPriceFeed{
		contract:      mockContract,
		onChainPrices: make(map[string]float64),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan pricefeed.Price)
	errChan := make(chan error)

	// Setup mock expectations
	mockSub.On("Err").Return(errChan)
	mockSub.On("Unsubscribe").Return()
	mockContract.On("WatchPriceChanged", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		sink := args.Get(1).(chan<- *contract.ContractPriceChanged)
		go func() {
			sink <- &contract.ContractPriceChanged{
				Symbol:    "bitcoin",
				NewPrice:  big.NewInt(3000000), // $30,000.00
				Timestamp: big.NewInt(time.Now().Unix()),
			}
		}()
	}).Return(mockSub, nil)

	// Start listening in a goroutine
	go feed.ListenOnChainPriceUpdate(ctx, out)

	// Check the result
	select {
	case price := <-out:
		assert.Equal(t, "bitcoin", price.Symbol)

		assert.Equal(t, 30000.00, price.USD)

	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for price update")
	}

	// Test cleanup
	cancel()
	time.Sleep(100 * time.Millisecond)
	mockSub.AssertExpectations(t)
	mockContract.AssertExpectations(t)
}

func TestListenOnChainPriceUpdate_Error(t *testing.T) {
	mockContract := new(MockContract)
	mockSub := new(MockSubscription)

	feed := &SepoliaPriceFeed{
		contract:      mockContract,
		onChainPrices: make(map[string]float64),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan pricefeed.Price)
	errChan := make(chan error, 1)

	// Setup mock expectations
	mockSub.On("Err").Return(errChan)
	mockSub.On("Unsubscribe").Return()
	mockContract.On("WatchPriceChanged", mock.Anything, mock.Anything).Return(mockSub, nil)

	// Start listening in a goroutine
	go feed.ListenOnChainPriceUpdate(ctx, out)

	// Send an error through the subscription
	errChan <- fmt.Errorf("test error")

	// Give some time for error processing
	time.Sleep(100 * time.Millisecond)

	// Test cleanup
	cancel()
	mockSub.AssertExpectations(t)
	mockContract.AssertExpectations(t)
}

// MockSepoliaPriceFeed embeds SepoliaPriceFeed and allows mocking getChainlinkPrice
type MockSepoliaPriceFeed struct {
	mock.Mock
	*SepoliaPriceFeed
}

// MockChainlinkPricer implements ChainlinkPricer for testing
type MockChainlinkPricer struct {
	mock.Mock
}

func (m *MockChainlinkPricer) getChainlinkPrice(symbol string) (int64, error) {
	args := m.Called(symbol)

	return args.Get(0).(int64), args.Error(1)
}

func TestValidatePrice(t *testing.T) {
	mockContract := new(MockContract)
	mockPricer := new(MockChainlinkPricer)
	feed := &SepoliaPriceFeed{
		contract: mockContract,
		onChainPrices: map[string]float64{
			"bitcoin": 30000.00,
		},
		chainlinkPricer: mockPricer,
		boundCache:      make(map[string]boundsCacheEntry), // Initialize the cache map
	}

	tests := []struct {
		name           string
		symbol         string
		price          int64
		chainlinkPrice int64
		want           bool
		wantErr        bool
	}{
		{
			name:           "price within bounds",
			symbol:         "bitcoin",
			price:          3050000, // $30,500.00
			chainlinkPrice: 30400,
			want:           false, // within 2% of contract price, should not write
			wantErr:        false,
		},
		{
			name:           "price outside contract bounds but within chainlink bounds",
			symbol:         "bitcoin",
			price:          3300000, // $33,000.00
			chainlinkPrice: 32000,
			want:           true, // outside 2% of contract price, within 20% of chainlink
			wantErr:        false,
		},
		{
			name:           "price outside all bounds",
			symbol:         "bitcoin",
			price:          4000000, // $40,000.00
			chainlinkPrice: 30000,
			want:           false, // outside 20% of chainlink price
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr {
				mockPricer.On("getChainlinkPrice", tt.symbol).Return(int64(0), fmt.Errorf("chainlink error")).Once()
			} else {
				mockPricer.On("getChainlinkPrice", tt.symbol).Return(tt.chainlinkPrice, nil).Once()
			}

			got, err := feed.validatePrice(tt.symbol, tt.price)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePrice() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if got != tt.want {
				t.Errorf("validatePrice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWriteToChain(t *testing.T) {
	mockContract := new(MockContract)
	feed := &SepoliaPriceFeed{
		contract:      mockContract,
		onChainPrices: make(map[string]float64),
		auth:          &bind.TransactOpts{},
	}

	tests := []struct {
		name    string
		symbol  string
		price   float64
		wantErr bool
	}{
		{
			name:    "successful write",
			symbol:  "bitcoin",
			price:   30000.00,
			wantErr: false,
		},
		{
			name:    "failed write",
			symbol:  "error_token",
			price:   1000.00,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			mockTx := types.NewTransaction(
				0,
				common.Address{},
				big.NewInt(0),
				0,
				big.NewInt(0),
				[]byte("mock transaction data"),
			)

			if tt.wantErr {
				mockContract.On("Set", mock.Anything, tt.symbol, big.NewInt(int64(tt.price*100))).
					Return(mockTx, fmt.Errorf("mock error")).Once()
			} else {
				mockContract.On("Set", mock.Anything, tt.symbol, big.NewInt(int64(tt.price*100))).
					Return(mockTx, nil).Once()
			}

			err := feed.writeToChain(ctx, tt.symbol, tt.price)
			if (err != nil) != tt.wantErr {
				t.Errorf("writeToChain() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr {
				// Verify price was cached
				assert.Equal(t, tt.price, feed.onChainPrices[tt.symbol])
			}
		})
	}

	mockContract.AssertExpectations(t)
}

func TestWritePricesToChain(t *testing.T) {
	mockContract := new(MockContract)
	mockPricer := new(MockChainlinkPricer)
	feed := &SepoliaPriceFeed{
		contract: mockContract,
		onChainPrices: map[string]float64{
			"bitcoin": 30000.00,
		},
		auth:            &bind.TransactOpts{},
		chainlinkPricer: mockPricer,
		boundCache:      make(map[string]boundsCacheEntry), // Initialize the boundCache map
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockTx := types.NewTransaction(
		0,
		common.Address{},
		big.NewInt(0),
		0,
		big.NewInt(0),
		[]byte("mock transaction data"),
	)

	// Set up mock expectations
	mockContract.On("Set", mock.Anything, "bitcoin", big.NewInt(3100000)). // $31,000.00
										Return(mockTx, nil)
	mockPricer.On("getChainlinkPrice", "bitcoin").Return(int64(31000), nil)

	in := make(chan []pricefeed.Price, 1)

	// Start WritePricesToChain in a goroutine
	go feed.WritePricesToChain(ctx, in)

	// Send test prices
	testPrices := []pricefeed.Price{
		{Symbol: "bitcoin", USD: 31000.00}, // Valid price
	}
	in <- testPrices

	// Give some time for processing
	time.Sleep(100 * time.Millisecond)

	// Clean up
	cancel()
	mockContract.AssertExpectations(t)
	mockPricer.AssertExpectations(t)

	// Verify the cache was updated for the valid price
	assert.Equal(t, 31000.00, feed.onChainPrices["bitcoin"])
}
