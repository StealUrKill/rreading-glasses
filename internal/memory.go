package internal

import (
	"context"
	"math"
	"runtime/debug"
	"time"

	"github.com/dgraph-io/ristretto/v2"
)

var _ cache[[]byte] = (*memoryCache)(nil)

// newMemoryCache returns a new in-memory cache sized relative to GOMEMLIMIT.
func newMemoryCache() cache[[]byte] {
	maxCost, numCounters := cacheSizing(debug.SetMemoryLimit(-1))

	r, err := ristretto.NewCache(&ristretto.Config[string, []byte]{
		NumCounters: numCounters,
		MaxCost:     maxCost,
		BufferItems: 64, // Number of keys per Get buffer.
	})
	if err != nil {
		panic(err)
	}

	return &memoryCache{r}
}

// cacheSizing derives ristretto's value budget (MaxCost) and admission counter
// count (NumCounters) from GOMEMLIMIT.
func cacheSizing(limit int64) (maxCost, numCounters int64) {
	// On hosts with no memory limit SetMemoryLimit returns math.MaxInt64. Fall
	// back to a fixed budget rather than an effectively unbounded cache.
	if limit <= 0 || limit >= math.MaxInt64/4 {
		maxCost = 64 << 20
	} else {
		maxCost = limit / 6
	}

	const estAvgItemBytes = 2 << 10
	numCounters = (maxCost / estAvgItemBytes) * 10
	if numCounters < 1<<14 {
		numCounters = 1 << 14
	}
	if numCounters > 4_000_000 {
		numCounters = 4_000_000
	}

	return maxCost, numCounters
}

type memoryCache struct {
	r *ristretto.Cache[string, []byte]
}

func (c *memoryCache) Get(_ context.Context, key string) ([]byte, bool) {
	return c.r.Get(key)
}

func (c *memoryCache) GetWithTTL(ctx context.Context, key string) ([]byte, time.Duration, bool) {
	ttl, ok := c.r.GetTTL(key)
	bytes, _ := c.Get(ctx, key)
	return bytes, ttl, ok
}

func (c *memoryCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) {
	_ = c.r.SetWithTTL(key, value, int64(len(value)), ttl)
	c.r.Wait() // Synchronous set.
}

func (c *memoryCache) Expire(_ context.Context, key string) error {
	c.r.Del(key)
	c.r.Wait() // Synchronous delete.
	return nil
}

func (c *memoryCache) Delete(ctx context.Context, key string) error {
	return c.Expire(ctx, key)
}
