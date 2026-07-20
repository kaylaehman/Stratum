package api

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// keyedLimiter is a per-key (typically per-client-IP) token-bucket limiter. It
// guards pre-auth and expensive endpoints without letting one caller's traffic
// throttle everyone — a single shared *rate.Limiter (like PreviewLimiter) would
// let one abusive IP lock out every other user of an unauthenticated endpoint
// such as login. Idle buckets are evicted so the map can't grow unbounded under
// a spray of distinct source IPs.
type keyedLimiter struct {
	mu      sync.Mutex
	buckets map[string]*keyedBucket
	limit   rate.Limit
	burst   int
	idleTTL time.Duration
	lastGC  time.Time
}

type keyedBucket struct {
	lim  *rate.Limiter
	seen time.Time
}

// NewKeyedLimiter builds a limiter that refills one token every `every` and
// tolerates a short burst. Zero/negative burst is coerced to 1.
func NewKeyedLimiter(every time.Duration, burst int) *keyedLimiter { //nolint:revive // returned only via exported Handlers fields
	if burst < 1 {
		burst = 1
	}
	return &keyedLimiter{
		buckets: make(map[string]*keyedBucket),
		limit:   rate.Every(every),
		burst:   burst,
		idleTTL: 10 * time.Minute,
	}
}

// Allow reports whether an event for key may proceed now. A nil limiter allows
// everything (fail-open on misconfiguration, matching PreviewLimiter's nil check).
func (k *keyedLimiter) Allow(key string) bool {
	if k == nil {
		return true
	}
	k.mu.Lock()
	defer k.mu.Unlock()

	now := time.Now()
	// Amortized eviction: sweep at most once per idleTTL, dropping buckets that
	// haven't been touched within the window.
	if now.Sub(k.lastGC) > k.idleTTL {
		for key, b := range k.buckets {
			if now.Sub(b.seen) > k.idleTTL {
				delete(k.buckets, key)
			}
		}
		k.lastGC = now
	}

	b := k.buckets[key]
	if b == nil {
		b = &keyedBucket{lim: rate.NewLimiter(k.limit, k.burst)}
		k.buckets[key] = b
	}
	b.seen = now
	return b.lim.Allow()
}
