// Package config provides configuration management for the DecTek service
package config

import (
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// Config holds the application configuration
type Config struct {
	Precision  string `env:"PRECISION" envDefault:"6"   validate:"numeric"`   // Decimal precision for price values
	Tokens     string `env:"TOKENS" required:"true"     validate:"required"`  // Comma-separated list of token symbols
	Url        string `env:"URL" required:"true"        validate:"url"`       // CoinGecko API URL
	Alchemy    string `env:"ALCHEMY" required:"true"    validate:"url"`       // Alchemy RPC URL
	Contract   string `env:"CONTRACT" required:"true"    validate:"eth_addr"` // Smart contract address
	PrivateKey string `env:"PRIVATEKEY" required:"true" validate:"eth_key"`   // Private key for transactions
}

// Option is a function that modifies Config
type Option func(*Config) error

// WithEnvFile loads configuration from a .env file
func WithEnvFile(path string) Option {
	return func(c *Config) error {
		if err := godotenv.Load(path); err != nil {
			return fmt.Errorf("failed to load env file: %w", err)
		}
		return nil
	}
}

// WithPrecision sets the precision value for price display
func WithPrecision(precision string) Option {
	return func(c *Config) error {
		c.Precision = precision
		return nil
	}
}

// validate performs validation on the config values
func (c *Config) validate() error {
	// Validate URLs
	for name, urlStr := range map[string]string{
		"CoinGecko": c.Url,
		"Alchemy":   c.Alchemy,
	} {
		if urlStr == "" {
			return fmt.Errorf("%s URL is required", name)
		}
		if _, err := url.ParseRequestURI(urlStr); err != nil {
			return fmt.Errorf("invalid %s URL: %s", name, urlStr)
		}
	}

	// Validate Ethereum address
	if !common.IsHexAddress(c.Contract) {
		return fmt.Errorf("invalid contract address: %s", c.Contract)
	}

	// Validate private key format (should be hex without 0x prefix)
	if len(c.PrivateKey) != 64 || !isHex(c.PrivateKey) {
		return fmt.Errorf("invalid private key format")
	}

	// Validate tokens
	if c.Tokens == "" {
		return fmt.Errorf("no tokens specified")
	}
	tokens := strings.Split(c.Tokens, ",")
	for _, token := range tokens {
		if token = strings.TrimSpace(token); token == "" {
			return fmt.Errorf("empty token in list")
		}
	}

	return nil
}

// isHex checks if a string is valid hexadecimal
func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// NewConfig creates a new validated Config instance
func NewConfig(opts ...Option) (*Config, error) {
	var cfg Config

	// Process environment variables first
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to process config: %w", err)
	}

	// Apply default options only if values are empty
	if cfg.Precision == "" {
		if err := WithPrecision("6")(&cfg); err != nil {
			log.Printf("⚠️ Warning: default option application failed: %v", err)
		}
	}

	// Apply user options last so they take precedence
	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			log.Printf("⚠️ Warning: option application failed: %v", err)
		}
	}

	// Validate the configuration
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// TokenList returns the list of tokens as a slice
func (c *Config) TokenList() []string {
	return strings.Split(c.Tokens, ",")
}
