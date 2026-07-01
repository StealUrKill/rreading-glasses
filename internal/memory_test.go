package internal

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCacheSizing(t *testing.T) {
	const MiB = 1 << 20

	t.Run("small container sizes down to fit the budget", func(t *testing.T) {
		// ~57.6MiB GOMEMLIMIT (0.45 * 128MiB), as seen on the 128m QNAP container.
		maxCost, numCounters := cacheSizing(60397977)

		// Values budgeted well under GOMEMLIMIT to leave room for ristretto's
		// ~2x bookkeeping overhead.
		assert.Less(t, maxCost, int64(11*MiB))
		assert.Greater(t, maxCost, int64(8*MiB))

		// NumCounters must be a tiny fraction of the old hardcoded 4e6 so the
		// sketch+bloom don't cost ~16MB up front.
		assert.Less(t, numCounters, int64(100_000))
		assert.GreaterOrEqual(t, numCounters, int64(1<<14))
	})

	t.Run("no limit falls back to a bounded budget", func(t *testing.T) {
		maxCost, numCounters := cacheSizing(math.MaxInt64)

		assert.Equal(t, int64(64*MiB), maxCost)
		assert.Equal(t, int64((64*MiB/(2<<10))*10), numCounters) // 327680
	})

	t.Run("zero/unknown limit is treated as unbounded", func(t *testing.T) {
		maxCost, _ := cacheSizing(0)
		assert.Equal(t, int64(64*MiB), maxCost)
	})

	t.Run("numCounters is floored", func(t *testing.T) {
		_, numCounters := cacheSizing(1 * MiB)
		assert.Equal(t, int64(1<<14), numCounters)
	})

	t.Run("large host scales up but caps counters", func(t *testing.T) {
		maxCost, numCounters := cacheSizing(8 << 30) // 8GiB
		assert.Equal(t, int64((8<<30)/6), maxCost)
		assert.Equal(t, int64(4_000_000), numCounters) // capped
	})
}
