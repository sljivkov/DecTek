package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConfig(t *testing.T) {
	// Test case 1: Test with environment variables
	t.Run("with environment variables", func(t *testing.T) {
		// Set up test environment variables
		t.Setenv("PRECISION", "2")
		t.Setenv("TOKENS", "bitcoin,ethereum")
		t.Setenv("URL", "http://test.com")
		t.Setenv("ALCHEMY", "test-alchemy")
		t.Setenv("CONTRACT", "0x123")
		t.Setenv("PRIVATEKEY", "test-key")

		cfg, err := NewConfig()
		assert.NoError(t, err)
		assert.NotNil(t, cfg)

		// Verify all fields
		assert.Equal(t, "2", cfg.Precision)
		assert.Equal(t, "bitcoin,ethereum", cfg.Tokens)
		assert.Equal(t, "http://test.com", cfg.Url)
		assert.Equal(t, "test-alchemy", cfg.Alchemy)
		assert.Equal(t, "0x123", cfg.Contract)
		assert.Equal(t, "test-key", cfg.PrivateKey)
	})

	// Test case 2: Test with missing environment variables
	t.Run("with missing environment variables", func(t *testing.T) {
		// Set empty environment variables
		t.Setenv("PRECISION", "")
		t.Setenv("TOKENS", "")
		t.Setenv("URL", "")
		t.Setenv("ALCHEMY", "")
		t.Setenv("CONTRACT", "")
		t.Setenv("PRIVATEKEY", "")

		cfg, err := NewConfig()
		assert.NoError(t, err) // Should not error even with missing vars
		assert.NotNil(t, cfg)  // Should return empty config
	})
}
