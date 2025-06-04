package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConfig(t *testing.T) {
	// Test case 1: Test with valid environment variables
	t.Run("with valid environment variables", func(t *testing.T) {
		t.Setenv("PRECISION", "2")
		t.Setenv("TOKENS", "bitcoin,ethereum")
		t.Setenv("URL", "http://test.com")
		t.Setenv("ALCHEMY", "http://alchemy.test")
		t.Setenv("CONTRACT", "0x123456789012345678901234567890123456789a")
		t.Setenv("PRIVATEKEY", "1234567890123456789012345678901234567890123456789012345678901234")

		cfg, err := NewConfig()
		assert.NoError(t, err)
		assert.NotNil(t, cfg)

		// Verify all fields
		assert.Equal(t, "2", cfg.Precision)
		assert.Equal(t, "bitcoin,ethereum", cfg.Tokens)
		assert.Equal(t, "http://test.com", cfg.Url)
		assert.Equal(t, "http://alchemy.test", cfg.Alchemy)
		assert.Equal(t, "0x123456789012345678901234567890123456789a", cfg.Contract)
		assert.Equal(t, "1234567890123456789012345678901234567890123456789012345678901234", cfg.PrivateKey)
	})

	// Test case 2: Test with invalid URL
	t.Run("with invalid URL", func(t *testing.T) {
		t.Setenv("PRECISION", "2")
		t.Setenv("TOKENS", "bitcoin,ethereum")
		t.Setenv("URL", "invalid-url")
		t.Setenv("ALCHEMY", "http://alchemy.test")
		t.Setenv("CONTRACT", "0x123456789012345678901234567890123456789a")
		t.Setenv("PRIVATEKEY", "1234567890123456789012345678901234567890123456789012345678901234")

		_, err := NewConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid CoinGecko URL")
	})

	// Test case 3: Test with invalid contract address
	t.Run("with invalid contract address", func(t *testing.T) {
		t.Setenv("PRECISION", "2")
		t.Setenv("TOKENS", "bitcoin,ethereum")
		t.Setenv("URL", "http://test.com")
		t.Setenv("ALCHEMY", "http://alchemy.test")
		t.Setenv("CONTRACT", "invalid-address")
		t.Setenv("PRIVATEKEY", "1234567890123456789012345678901234567890123456789012345678901234")

		_, err := NewConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid contract address")
	})

	// Test case 4: Test with invalid private key
	t.Run("with invalid private key", func(t *testing.T) {
		t.Setenv("PRECISION", "2")
		t.Setenv("TOKENS", "bitcoin,ethereum")
		t.Setenv("URL", "http://test.com")
		t.Setenv("ALCHEMY", "http://alchemy.test")
		t.Setenv("CONTRACT", "0x123456789012345678901234567890123456789a")
		t.Setenv("PRIVATEKEY", "invalid-key")

		_, err := NewConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid private key format")
	})

	// Test case 5: Test with empty token list
	t.Run("with empty token list", func(t *testing.T) {
		t.Setenv("PRECISION", "2")
		t.Setenv("TOKENS", "")
		t.Setenv("URL", "http://test.com")
		t.Setenv("ALCHEMY", "http://alchemy.test")
		t.Setenv("CONTRACT", "0x123456789012345678901234567890123456789a")
		t.Setenv("PRIVATEKEY", "1234567890123456789012345678901234567890123456789012345678901234")

		_, err := NewConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no tokens specified")
	})
}

func TestConfigOptions(t *testing.T) {
	// Test WithPrecision option
	t.Run("with precision", func(t *testing.T) {
		t.Setenv("TOKENS", "bitcoin,ethereum")
		t.Setenv("URL", "http://test.com")
		t.Setenv("ALCHEMY", "http://alchemy.test")
		t.Setenv("CONTRACT", "0x123456789012345678901234567890123456789a")
		t.Setenv("PRIVATEKEY", "1234567890123456789012345678901234567890123456789012345678901234")

		cfg, err := NewConfig(WithPrecision("8"))
		assert.NoError(t, err)
		assert.Equal(t, "8", cfg.Precision)
	})

	// Test TokenList helper
	t.Run("token list helper", func(t *testing.T) {
		t.Setenv("PRECISION", "2")
		t.Setenv("TOKENS", "bitcoin,ethereum")
		t.Setenv("URL", "http://test.com")
		t.Setenv("ALCHEMY", "http://alchemy.test")
		t.Setenv("CONTRACT", "0x123456789012345678901234567890123456789a")
		t.Setenv("PRIVATEKEY", "1234567890123456789012345678901234567890123456789012345678901234")

		cfg, err := NewConfig()
		assert.NoError(t, err)
		tokens := cfg.TokenList()
		assert.Equal(t, []string{"bitcoin", "ethereum"}, tokens)
	})
}
