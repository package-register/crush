package aguiserver

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTokenBucket_Allow(t *testing.T) {
	// Create a bucket with capacity 5 and refill rate 1 token/second
	bucket := NewTokenBucket(5, 1)

	// Should allow 5 requests immediately (burst)
	for i := 0; i < 5; i++ {
		if !bucket.Allow() {
			t.Errorf("Expected Allow() to return true for request %d", i)
		}
	}

	// 6th request should be denied (no tokens left)
	if bucket.Allow() {
		t.Error("Expected Allow() to return false after exhausting tokens")
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	// Create a bucket with capacity 2 and refill rate 10 tokens/second
	bucket := NewTokenBucket(2, 10)

	// Exhaust all tokens
	bucket.Allow()
	bucket.Allow()

	// Wait for refill (0.1 seconds = 1 token at 10 tokens/sec)
	time.Sleep(150 * time.Millisecond)

	// Should have at least 1 token now
	if !bucket.Allow() {
		t.Error("Expected Allow() to return true after refill")
	}
}

func TestTokenBucket_SetRate(t *testing.T) {
	bucket := NewTokenBucket(10, 1)

	// Change rate to 10 tokens/second
	bucket.SetRate(10)

	// Exhaust tokens
	for i := 0; i < 10; i++ {
		bucket.Allow()
	}

	// Wait for refill at new rate (0.1 seconds = 1 token at 10 tokens/sec)
	time.Sleep(150 * time.Millisecond)

	// Should have tokens now
	if !bucket.Allow() {
		t.Error("Expected Allow() to return true after rate change")
	}
}

func TestTokenBucket_SetCapacity(t *testing.T) {
	bucket := NewTokenBucket(5, 1)

	// Reduce capacity
	bucket.SetCapacity(2)

	// Should only allow 2 requests now
	if !bucket.Allow() {
		t.Error("Expected Allow() to return true for first request")
	}
	if !bucket.Allow() {
		t.Error("Expected Allow() to return true for second request")
	}
	if bucket.Allow() {
		t.Error("Expected Allow() to return false after capacity reduction")
	}
}

func TestTokenBucket_Tokens(t *testing.T) {
	bucket := NewTokenBucket(10, 1)

	// Initial tokens should be at capacity
	tokens := bucket.Tokens()
	if tokens != 10 {
		t.Errorf("Expected 10 tokens, got %f", tokens)
	}

	// Consume 5 tokens
	for i := 0; i < 5; i++ {
		bucket.Allow()
	}

	tokens = bucket.Tokens()
	if tokens < 4 || tokens > 6 {
		t.Errorf("Expected approximately 5 tokens, got %f", tokens)
	}
}

func TestTokenBucket_Reset(t *testing.T) {
	bucket := NewTokenBucket(10, 1)

	// Exhaust all tokens
	for i := 0; i < 10; i++ {
		bucket.Allow()
	}

	// Reset
	bucket.Reset()

	// Should have full capacity
	tokens := bucket.Tokens()
	if tokens != 10 {
		t.Errorf("Expected 10 tokens after reset, got %f", tokens)
	}
}

func TestTokenBucket_Concurrent(t *testing.T) {
	bucket := NewTokenBucket(100, 100)

	var allowed int64
	var wg sync.WaitGroup

	// Launch 200 goroutines trying to get tokens
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if bucket.Allow() {
				atomic.AddInt64(&allowed, 1)
			}
		}()
	}

	wg.Wait()

	// Should have allowed exactly 100 requests
	if allowed != 100 {
		t.Errorf("Expected 100 allowed requests, got %d", allowed)
	}
}

func TestRateLimiter_MultipleKeys(t *testing.T) {
	limiter := NewRateLimiter(10, 10)

	// Different keys should have independent limiters
	key1 := "key1"
	key2 := "key2"

	// Exhaust key1's tokens
	for i := 0; i < 10; i++ {
		if !limiter.Allow(key1) {
			t.Errorf("Expected Allow(%s) to return true for request %d", key1, i)
		}
	}

	// key1 should be rate limited
	if limiter.Allow(key1) {
		t.Error("Expected Allow(key1) to return false after exhausting tokens")
	}

	// key2 should still have tokens
	if !limiter.Allow(key2) {
		t.Error("Expected Allow(key2) to return true")
	}
}

func TestRateLimiter_SetRate(t *testing.T) {
	limiter := NewRateLimiter(1, 1)

	key := "test-key"

	// Exhaust tokens
	limiter.Allow(key)

	// Change rate
	limiter.SetRate(key, 10)

	// Wait for refill
	time.Sleep(150 * time.Millisecond)

	// Should have tokens now
	if !limiter.Allow(key) {
		t.Error("Expected Allow() to return true after rate change")
	}
}

func TestRateLimiter_Remove(t *testing.T) {
	limiter := NewRateLimiter(10, 10)

	key := "test-key"

	// Use some tokens
	limiter.Allow(key)

	// Remove the limiter
	limiter.Remove(key)

	// Should create a new limiter with full tokens
	if !limiter.Allow(key) {
		t.Error("Expected Allow() to return true after remove")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	limiter := NewRateLimiter(10, 10)

	// Create limiters for different keys
	limiter.Allow("key1")
	limiter.Allow("key2")

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Cleanup with very short timeout
	removed := limiter.Cleanup(5 * time.Millisecond)

	// Both should be removed
	if removed != 2 {
		t.Errorf("Expected 2 removed limiters, got %d", removed)
	}
}

func TestRequestTracker_Track(t *testing.T) {
	tracker := NewRequestTracker(100*time.Millisecond, 50*time.Millisecond)
	defer tracker.Close()

	requestID := "test-request-1"

	// First track should succeed
	if !tracker.Track(requestID) {
		t.Error("Expected first Track() to return true")
	}

	// Second track should fail (duplicate)
	if tracker.Track(requestID) {
		t.Error("Expected second Track() to return false for duplicate")
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// After timeout, should be able to track again
	if !tracker.Track(requestID) {
		t.Error("Expected Track() to return true after timeout")
	}
}

func TestRequestTracker_EmptyID(t *testing.T) {
	tracker := NewRequestTracker(100*time.Millisecond, 50*time.Millisecond)
	defer tracker.Close()

	// Empty request ID should always be allowed
	if !tracker.Track("") {
		t.Error("Expected Track(\"\") to return true")
	}
	if !tracker.Track("") {
		t.Error("Expected Track(\"\") to return true")
	}
}

func TestRequestTracker_Concurrent(t *testing.T) {
	tracker := NewRequestTracker(1*time.Second, 500*time.Millisecond)
	defer tracker.Close()

	requestID := "concurrent-request"

	var tracked int64
	var notTracked int64
	var wg sync.WaitGroup

	// Launch 100 goroutines trying to track the same request
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tracker.Track(requestID) {
				atomic.AddInt64(&tracked, 1)
			} else {
				atomic.AddInt64(&notTracked, 1)
			}
		}()
	}

	wg.Wait()

	// Only one should be tracked, rest should be duplicates
	if tracked != 1 {
		t.Errorf("Expected 1 tracked request, got %d", tracked)
	}
	if notTracked != 99 {
		t.Errorf("Expected 99 duplicate requests, got %d", notTracked)
	}
}

func TestRequestTracker_MultipleIDs(t *testing.T) {
	tracker := NewRequestTracker(1*time.Second, 500*time.Millisecond)
	defer tracker.Close()

	// Different request IDs should be tracked independently
	if !tracker.Track("request-1") {
		t.Error("Expected Track(request-1) to return true")
	}
	if !tracker.Track("request-2") {
		t.Error("Expected Track(request-2) to return true")
	}
	if !tracker.Track("request-3") {
		t.Error("Expected Track(request-3) to return true")
	}

	// Duplicates should fail
	if tracker.Track("request-1") {
		t.Error("Expected Track(request-1) to return false for duplicate")
	}
	if tracker.Track("request-2") {
		t.Error("Expected Track(request-2) to return false for duplicate")
	}
}

func TestRequestTracker_Remove(t *testing.T) {
	tracker := NewRequestTracker(1*time.Second, 500*time.Millisecond)
	defer tracker.Close()

	requestID := "test-request"

	// Track the request
	tracker.Track(requestID)

	// Remove it
	tracker.Remove(requestID)

	// Should be able to track again
	if !tracker.Track(requestID) {
		t.Error("Expected Track() to return true after remove")
	}
}

// Benchmark tests

func BenchmarkTokenBucket_Allow(b *testing.B) {
	bucket := NewTokenBucket(1000, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bucket.Allow()
	}
}

func BenchmarkTokenBucket_Concurrent(b *testing.B) {
	bucket := NewTokenBucket(10000, 10000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bucket.Allow()
		}
	})
}

func BenchmarkRateLimiter_Allow(b *testing.B) {
	limiter := NewRateLimiter(1000, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow("test-key")
	}
}

func BenchmarkRateLimiter_MultipleKeys(b *testing.B) {
	limiter := NewRateLimiter(1000, 1000)
	keys := make([]string, 100)
	for i := range keys {
		keys[i] = "key-" + string(rune(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow(keys[i%100])
	}
}

func BenchmarkRequestTracker_Track(b *testing.B) {
	tracker := NewRequestTracker(1*time.Second, 500*time.Millisecond)
	defer tracker.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		requestID := "request-" + string(rune(i%1000))
		tracker.Track(requestID)
	}
}

func BenchmarkRequestTracker_Concurrent(b *testing.B) {
	tracker := NewRequestTracker(1*time.Second, 500*time.Millisecond)
	defer tracker.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			requestID := "request-" + string(rune(i%1000))
			tracker.Track(requestID)
			i++
		}
	})
}
