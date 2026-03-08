package aguiserver

import (
	"sync"
	"time"
)

// TokenBucket implements the token bucket rate limiting algorithm.
// It allows for burst traffic while maintaining a steady average rate.
type TokenBucket struct {
	tokens     float64
	capacity   float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

// NewTokenBucket creates a new token bucket with the given capacity and refill rate.
// capacity: maximum number of tokens (burst size)
// refillRate: tokens added per second (sustained rate)
func NewTokenBucket(capacity, refillRate float64) *TokenBucket {
	return &TokenBucket{
		tokens:     capacity,
		capacity:   capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed under the rate limit.
// It consumes one token if available and returns true.
// Returns false if no tokens are available (rate limited).
func (b *TokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens = min(b.capacity, b.tokens+elapsed*b.refillRate)
	b.lastRefill = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// AllowN checks if n requests are allowed under the rate limit.
// It consumes n tokens if available and returns true.
// Returns false if insufficient tokens are available.
func (b *TokenBucket) AllowN(n int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens = min(b.capacity, b.tokens+elapsed*b.refillRate)
	b.lastRefill = now

	if b.tokens >= float64(n) {
		b.tokens -= float64(n)
		return true
	}
	return false
}

// SetRate updates the refill rate of the token bucket.
// This allows dynamic adjustment of the rate limit.
func (b *TokenBucket) SetRate(qps float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refillRate = qps
}

// SetCapacity updates the capacity of the token bucket.
// This allows dynamic adjustment of the burst size.
func (b *TokenBucket) SetCapacity(capacity float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.capacity = capacity
	if b.tokens > capacity {
		b.tokens = capacity
	}
}

// Tokens returns the current number of available tokens.
// This is useful for monitoring and debugging.
func (b *TokenBucket) Tokens() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	tokens := min(b.capacity, b.tokens+elapsed*b.refillRate)
	return tokens
}

// Reset resets the token bucket to full capacity.
func (b *TokenBucket) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tokens = b.capacity
	b.lastRefill = time.Now()
}

// RateLimiter manages rate limiters for multiple keys (e.g., sessions, IPs).
type RateLimiter struct {
	mu          sync.RWMutex
	limiters    map[string]*TokenBucket
	defaultQPS  float64
	defaultBurst float64
}

// NewRateLimiter creates a new rate limiter manager.
// defaultQPS: default queries per second for new keys
// defaultBurst: default burst size for new keys
func NewRateLimiter(defaultQPS, defaultBurst float64) *RateLimiter {
	return &RateLimiter{
		limiters:    make(map[string]*TokenBucket),
		defaultQPS:  defaultQPS,
		defaultBurst: defaultBurst,
	}
}

// GetLimiter retrieves or creates a rate limiter for the given key.
func (r *RateLimiter) GetLimiter(key string) *TokenBucket {
	r.mu.RLock()
	limiter, ok := r.limiters[key]
	r.mu.RUnlock()

	if ok {
		return limiter
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, ok = r.limiters[key]; ok {
		return limiter
	}

	limiter = NewTokenBucket(r.defaultBurst, r.defaultQPS)
	r.limiters[key] = limiter
	return limiter
}

// Allow checks if a request is allowed for the given key.
func (r *RateLimiter) Allow(key string) bool {
	return r.GetLimiter(key).Allow()
}

// SetRate sets the rate limit for a specific key.
func (r *RateLimiter) SetRate(key string, qps float64) {
	r.GetLimiter(key).SetRate(qps)
}

// SetCapacity sets the burst capacity for a specific key.
func (r *RateLimiter) SetCapacity(key string, capacity float64) {
	r.GetLimiter(key).SetCapacity(capacity)
}

// Remove removes the rate limiter for a specific key.
func (r *RateLimiter) Remove(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.limiters, key)
}

// Cleanup removes rate limiters that have been idle for too long.
// This helps prevent memory leaks in long-running servers.
func (r *RateLimiter) Cleanup(idleTimeout time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	removed := 0

	for key, limiter := range r.limiters {
		limiter.mu.Lock()
		idle := now.Sub(limiter.lastRefill)
		limiter.mu.Unlock()

		if idle > idleTimeout {
			delete(r.limiters, key)
			removed++
		}
	}

	return removed
}

// RequestTracker tracks request IDs to prevent duplicate submissions.
type RequestTracker struct {
	mu       sync.Map // map[string]time.Time
	timeout  time.Duration
	cleanupT time.Duration
}

// NewRequestTracker creates a new request tracker.
// timeout: how long to remember a request ID
// cleanupInterval: how often to clean up expired entries
func NewRequestTracker(timeout, cleanupInterval time.Duration) *RequestTracker {
	rt := &RequestTracker{
		timeout:  timeout,
		cleanupT: cleanupInterval,
	}
	go rt.cleanupLoop()
	return rt
}

// Track attempts to track a request ID.
// Returns true if the request is new (not tracked before).
// Returns false if the request ID is already being tracked (duplicate).
func (rt *RequestTracker) Track(requestID string) bool {
	if requestID == "" {
		return true
	}

	actual, loaded := rt.mu.LoadOrStore(requestID, time.Now())
	if loaded {
		// Check if the existing entry has expired
		timestamp := actual.(time.Time)
		if time.Since(timestamp) > rt.timeout {
			rt.mu.Delete(requestID)
			rt.mu.Store(requestID, time.Now())
			return true
		}
		return false
	}
	return true
}

// Remove removes a request ID from tracking.
func (rt *RequestTracker) Remove(requestID string) {
	rt.mu.Delete(requestID)
}

// cleanupLoop periodically removes expired request IDs.
func (rt *RequestTracker) cleanupLoop() {
	// Handle zero or negative cleanup interval
	if rt.cleanupT <= 0 {
		return
	}
	
	ticker := time.NewTicker(rt.cleanupT)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		rt.mu.Range(func(key, value any) bool {
			timestamp := value.(time.Time)
			if now.Sub(timestamp) > rt.timeout {
				rt.mu.Delete(key)
			}
			return true
		})
	}
}

// min returns the minimum of two float64 values.
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
