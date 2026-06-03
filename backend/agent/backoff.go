package agent

import (
	"math/rand/v2"
	"time"
)

// backoffConfig holds parameters for exponential backoff with full jitter.
// All durations are used directly; callers must supply sensible values.
type backoffConfig struct {
	base    time.Duration
	max     time.Duration
	factor  float64
	randSrc rand.Source // injectable for deterministic tests; nil uses global rand
}

// defaultBackoff is the production reconnection schedule:
// base=1s, factor=2, cap=30s, full jitter.
var defaultBackoff = backoffConfig{
	base:   1 * time.Second,
	max:    30 * time.Second,
	factor: 2.0,
}

// next returns the delay for attempt n (0-indexed) using full jitter:
//
//	sleep = random_between(0, min(cap, base * factor^n))
//
// Full jitter prevents thundering-herd when many nodes reconnect simultaneously.
func (b backoffConfig) next(attempt int) time.Duration {
	// Compute uncapped ceiling.
	ceiling := b.base
	for i := 0; i < attempt; i++ {
		next := time.Duration(float64(ceiling) * b.factor)
		if next > b.max || next < ceiling { // overflow guard
			ceiling = b.max
			break
		}
		ceiling = next
	}
	if ceiling > b.max {
		ceiling = b.max
	}

	// Full jitter: uniform[0, ceiling).
	if ceiling <= 0 {
		return 0
	}
	var jitter int64
	if b.randSrc != nil {
		jitter = rand.New(b.randSrc).Int64N(int64(ceiling))
	} else {
		jitter = rand.Int64N(int64(ceiling))
	}
	return time.Duration(jitter)
}
