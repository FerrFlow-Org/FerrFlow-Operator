package controller

import (
	"math/rand"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
)

const (
	rateLimitBackoffBase  = 5 * time.Second
	rateLimitBackoffMax   = 5 * time.Minute
	rateLimitBackoffShift = 8
)

type rateLimitBackoff struct {
	mu       sync.Mutex
	attempts map[types.NamespacedName]int
	rand     *rand.Rand
}

func newRateLimitBackoff() *rateLimitBackoff {
	return &rateLimitBackoff{
		attempts: make(map[types.NamespacedName]int),
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (b *rateLimitBackoff) next(key types.NamespacedName) time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()

	attempt := b.attempts[key]
	b.attempts[key] = attempt + 1
	return b.jitter(backoffFor(attempt))
}

func (b *rateLimitBackoff) forget(key types.NamespacedName) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.attempts, key)
}

func (b *rateLimitBackoff) jitter(d time.Duration) time.Duration {
	half := d / 2
	if half <= 0 {
		return d
	}
	return half + time.Duration(b.rand.Int63n(int64(half)))
}

func backoffFor(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	if attempt > rateLimitBackoffShift {
		attempt = rateLimitBackoffShift
	}
	d := rateLimitBackoffBase << uint(attempt)
	if d > rateLimitBackoffMax {
		return rateLimitBackoffMax
	}
	return d
}
