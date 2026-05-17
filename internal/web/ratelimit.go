package web

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

// rateBucket tracks request count within a sliding window for one IP+tier.
type rateBucket struct {
	count   int
	resetAt time.Time
}

// rateLimiter implements a simple per-IP token bucket rate limiter.
// Zero external dependencies — uses sync.Mutex for concurrent access.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rateBucket // key: "ip:tier"
	limits  map[string]int         // tier → max requests per window
	window  time.Duration
}

// newRateLimiter creates a rate limiter with the given window duration.
func newRateLimiter(window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		buckets: make(map[string]*rateBucket),
		limits:  make(map[string]int),
		window:  window,
	}

	// Background cleanup of stale buckets (every 5 minutes)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rl.cleanup()
		}
	}()

	return rl
}

// setLimit configures the max requests per window for a tier.
func (rl *rateLimiter) setLimit(tier string, maxRequests int) {
	rl.limits[tier] = maxRequests
}

// allow checks if a request from the given IP in the given tier is allowed.
// Returns (allowed bool, retryAfterSec int).
func (rl *rateLimiter) allow(ip, tier string) (bool, int) {
	key := ip + ":" + tier
	limit, ok := rl.limits[tier]
	if !ok {
		return true, 0 // no limit configured for this tier
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, exists := rl.buckets[key]

	if !exists || now.After(bucket.resetAt) {
		// New window
		rl.buckets[key] = &rateBucket{
			count:   1,
			resetAt: now.Add(rl.window),
		}
		return true, 0
	}

	if bucket.count >= limit {
		retryAfter := int(bucket.resetAt.Sub(now).Seconds()) + 1
		return false, retryAfter
	}

	bucket.count++
	return true, 0
}

// cleanup removes expired buckets.
func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	for key, bucket := range rl.buckets {
		if now.After(bucket.resetAt) {
			delete(rl.buckets, key)
		}
	}
}

// rateMiddleware returns HTTP middleware that enforces rate limiting for the given tier.
func (rl *rateLimiter) rateMiddleware(tier string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		allowed, retryAfter := rl.allow(ip, tier)
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			jsonError(w, "rate limit exceeded, retry after "+strconv.Itoa(retryAfter)+"s", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	}
}
