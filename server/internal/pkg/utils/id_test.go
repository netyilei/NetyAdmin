package utils_test

import (
	"regexp"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"NetyAdmin/internal/pkg/utils"
)

func TestNewULID(t *testing.T) {
	t.Run("returns non-empty string", func(t *testing.T) {
		id := utils.NewULID()
		require.NotEmpty(t, id)
	})

	t.Run("returns 26-character string", func(t *testing.T) {
		id := utils.NewULID()
		// ULID spec: 128 bits = 26 chars in Crockford base32
		assert.Len(t, id, 26)
	})

	t.Run("matches ULID format", func(t *testing.T) {
		id := utils.NewULID()
		// Crockford base32: 0-9, A-Z excluding I, L, O, U
		matched, err := regexp.MatchString(`^[0-9A-HJKMNP-TV-Z]{26}$`, id)
		require.NoError(t, err)
		assert.True(t, matched, "ULID %q does not match expected format", id)
	})

	t.Run("generates unique IDs", func(t *testing.T) {
		ids := make(map[string]bool, 1000)
		for i := 0; i < 1000; i++ {
			id := utils.NewULID()
			assert.False(t, ids[id], "duplicate ULID generated: %s", id)
			ids[id] = true
		}
		assert.Len(t, ids, 1000)
	})

	t.Run("monotonic ordering within same millisecond", func(t *testing.T) {
		// Generate multiple ULIDs rapidly — they should be lexicographically
		// sortable due to the monotonic entropy source
		var ids []string
		for i := 0; i < 100; i++ {
			ids = append(ids, utils.NewULID())
		}

		// All ULIDs should be in ascending order (monotonic)
		for i := 1; i < len(ids); i++ {
			assert.True(t, ids[i] > ids[i-1],
				"ULID at index %d (%s) should be >= previous (%s)",
				i, ids[i], ids[i-1])
		}
	})

	t.Run("timestamp component reflects current time", func(t *testing.T) {
		before := time.Now().Add(-10 * time.Millisecond)
		id := utils.NewULID()
		after := time.Now().Add(10 * time.Millisecond)

		// ULID encodes Unix millisecond timestamp in first 10 chars (48 bits).
		// Parse it back via the oklog/ulid library to verify the timestamp
		// falls within the [before, after] window.
		parsed, err := ulid.Parse(id)
		require.NoError(t, err)

		idTime := ulid.Time(parsed.Time())
		assert.False(t, idTime.Before(before), "ULID timestamp %v is before expected window start %v", idTime, before)
		assert.False(t, idTime.After(after), "ULID timestamp %v is after expected window end %v", idTime, after)
	})

	t.Run("uppercase only", func(t *testing.T) {
		id := utils.NewULID()
		// ULID strings are uppercase
		assert.Equal(t, id, func() string {
			upper := ""
			for _, c := range id {
				if c >= 'a' && c <= 'z' {
					upper += string(c - 32)
				} else {
					upper += string(c)
				}
			}
			return upper
		}(), "ULID should be uppercase: %s", id)
	})
}
