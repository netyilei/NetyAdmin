package utils_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"NetyAdmin/internal/pkg/utils"
)

const testAESKey = "0123456789abcdef0123456789abcdef" // 32 bytes for AES-256

func TestEncryptDecrypt(t *testing.T) {
	t.Run("round trip returns original", func(t *testing.T) {
		plaintext := "Hello, World!"
		ciphertext, err := utils.Encrypt(plaintext, testAESKey)
		require.NoError(t, err)
		require.NotEmpty(t, ciphertext)

		decrypted, err := utils.Decrypt(ciphertext, testAESKey)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})

	t.Run("empty plaintext", func(t *testing.T) {
		ciphertext, err := utils.Encrypt("", testAESKey)
		require.NoError(t, err)

		decrypted, err := utils.Decrypt(ciphertext, testAESKey)
		require.NoError(t, err)
		assert.Equal(t, "", decrypted)
	})

	t.Run("unicode plaintext", func(t *testing.T) {
		plaintext := "你好世界🔐"
		ciphertext, err := utils.Encrypt(plaintext, testAESKey)
		require.NoError(t, err)

		decrypted, err := utils.Decrypt(ciphertext, testAESKey)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})

	t.Run("different encryptions produce different ciphertexts", func(t *testing.T) {
		plaintext := "same text"
		ct1, _ := utils.Encrypt(plaintext, testAESKey)
		ct2, _ := utils.Encrypt(plaintext, testAESKey)
		assert.NotEqual(t, ct1, ct2)
	})
}

func TestDecrypt_Errors(t *testing.T) {
	t.Run("wrong key fails decryption", func(t *testing.T) {
		plaintext := "secret data"
		ciphertext, err := utils.Encrypt(plaintext, testAESKey)
		require.NoError(t, err)

		wrongKey := "abcdef0123456789abcdef0123456789"
		_, err = utils.Decrypt(ciphertext, wrongKey)
		assert.Error(t, err)
	})

	t.Run("invalid base64 fails", func(t *testing.T) {
		_, err := utils.Decrypt("!!!not-base64!!!", testAESKey)
		assert.Error(t, err)
	})

	t.Run("too short ciphertext fails", func(t *testing.T) {
		// base64 of 5 bytes, less than nonce size (12 for GCM)
		shortData := "aGVsbG8="
		_, err := utils.Decrypt(shortData, testAESKey)
		assert.Error(t, err)
	})
}

func TestEncrypt_KeyValidation(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		keyLen int
		valid  bool
	}{
		{"AES-128 (16 bytes)", "0123456789abcdef", 16, true},
		{"AES-192 (24 bytes)", "0123456789abcdef01234567", 24, true},
		{"AES-256 (32 bytes)", testAESKey, 32, true},
		{"invalid (15 bytes)", "0123456789abcde", 15, false},
		{"invalid (17 bytes)", "0123456789abcdefg", 17, false},
		{"invalid (empty)", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := utils.Encrypt("test", tt.key)
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}

	// Verify the key length strings are correct
	assert.True(t, len(testAESKey) == 32)
	assert.True(t, strings.Repeat("a", 16) != testAESKey)
}
