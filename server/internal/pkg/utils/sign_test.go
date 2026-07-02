package utils_test

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"NetyAdmin/internal/pkg/utils"
)

const testSignKey = "test-secret-key-for-hmac"

func TestHMACSHA256Hex(t *testing.T) {
	t.Run("returns correct hex length", func(t *testing.T) {
		result := utils.HMACSHA256Hex(testSignKey, "test data")
		// SHA-256 produces 32 bytes = 64 hex chars
		assert.Len(t, result, 64)
	})

	t.Run("deterministic for same inputs", func(t *testing.T) {
		r1 := utils.HMACSHA256Hex(testSignKey, "same data")
		r2 := utils.HMACSHA256Hex(testSignKey, "same data")
		assert.Equal(t, r1, r2)
	})

	t.Run("different key produces different result", func(t *testing.T) {
		r1 := utils.HMACSHA256Hex("key1", "data")
		r2 := utils.HMACSHA256Hex("key2", "data")
		assert.NotEqual(t, r1, r2)
	})

	t.Run("different data produces different result", func(t *testing.T) {
		r1 := utils.HMACSHA256Hex(testSignKey, "data1")
		r2 := utils.HMACSHA256Hex(testSignKey, "data2")
		assert.NotEqual(t, r1, r2)
	})

	t.Run("valid hex encoding", func(t *testing.T) {
		result := utils.HMACSHA256Hex(testSignKey, "test")
		_, err := hex.DecodeString(result)
		assert.NoError(t, err)
	})
}

func TestSignUploadRecord(t *testing.T) {
	t.Run("deterministic for same inputs", func(t *testing.T) {
		sig1 := utils.SignUploadRecord(testSignKey, 1, "obj/key", "article", "src123", 1700000000)
		sig2 := utils.SignUploadRecord(testSignKey, 1, "obj/key", "article", "src123", 1700000000)
		assert.Equal(t, sig1, sig2)
	})

	t.Run("different recordID produces different signature", func(t *testing.T) {
		sig1 := utils.SignUploadRecord(testSignKey, 1, "obj/key", "article", "src123", 1700000000)
		sig2 := utils.SignUploadRecord(testSignKey, 2, "obj/key", "article", "src123", 1700000000)
		assert.NotEqual(t, sig1, sig2)
	})

	t.Run("different objectKey produces different signature", func(t *testing.T) {
		sig1 := utils.SignUploadRecord(testSignKey, 1, "obj/key1", "article", "src123", 1700000000)
		sig2 := utils.SignUploadRecord(testSignKey, 1, "obj/key2", "article", "src123", 1700000000)
		assert.NotEqual(t, sig1, sig2)
	})

	t.Run("different expiry produces different signature", func(t *testing.T) {
		sig1 := utils.SignUploadRecord(testSignKey, 1, "obj/key", "article", "src123", 1700000000)
		sig2 := utils.SignUploadRecord(testSignKey, 1, "obj/key", "article", "src123", 1800000000)
		assert.NotEqual(t, sig1, sig2)
	})
}

func TestVerifyUploadRecord(t *testing.T) {
	t.Run("valid signature returns true", func(t *testing.T) {
		sig := utils.SignUploadRecord(testSignKey, 42, "path/to/file", "article", "art-1", 1700000000)
		valid := utils.VerifyUploadRecord(testSignKey, 42, "path/to/file", "article", "art-1", 1700000000, sig)
		assert.True(t, valid)
	})

	t.Run("tampered recordID returns false", func(t *testing.T) {
		sig := utils.SignUploadRecord(testSignKey, 42, "path/to/file", "article", "art-1", 1700000000)
		valid := utils.VerifyUploadRecord(testSignKey, 99, "path/to/file", "article", "art-1", 1700000000, sig)
		assert.False(t, valid)
	})

	t.Run("tampered objectKey returns false", func(t *testing.T) {
		sig := utils.SignUploadRecord(testSignKey, 42, "path/to/file", "article", "art-1", 1700000000)
		valid := utils.VerifyUploadRecord(testSignKey, 42, "path/tampered", "article", "art-1", 1700000000, sig)
		assert.False(t, valid)
	})

	t.Run("wrong key returns false", func(t *testing.T) {
		sig := utils.SignUploadRecord(testSignKey, 42, "path/to/file", "article", "art-1", 1700000000)
		valid := utils.VerifyUploadRecord("wrong-key", 42, "path/to/file", "article", "art-1", 1700000000, sig)
		assert.False(t, valid)
	})

	t.Run("empty signature returns false", func(t *testing.T) {
		valid := utils.VerifyUploadRecord(testSignKey, 42, "path/to/file", "article", "art-1", 1700000000, "")
		// hmac.Equal on empty vs non-empty returns false
		require.False(t, valid)
	})
}
