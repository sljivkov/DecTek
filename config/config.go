// Package config provides configuration management for the DecTek service
package config

import (
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// Config holds the application configuration loaded from environment variables
type Config struct {
	Precision  string `env:"PRECISION" envDefault:"6"`   // Decimal precision for price values
	Tokens     string `env:"TOKENS" required:"true"`     // Comma-separated list of token symbols
	Url        string `env:"URL" required:"true"`        // CoinGecko API URL
	Alchemy    string `env:"ALCHEMY" required:"true"`    // Alchemy RPC URL
	Contract   string `env:"CONTRACT" required:"true"`   // Smart contract address
	PrivateKey string `env:"PRIVATEKEY" required:"true"` // Private key for transactions
}

// NewConfig creates a new Config instance from environment variables
func NewConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Printf("⚠️ Warning: failed to load .env file: %v", err)
	}

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to process config: %w", err)
	}

	return &cfg, nil
}
