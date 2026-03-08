package aguiserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestRateLimitMiddleware tests the rate limit middleware.
func TestRateLimitMiddleware(t *testing.T) {
	config := DefaultRateLimitConfig()
	config.GlobalQPS = 1000
	config.GlobalBurst = 1000
	config.SessionQPS = 100
	config.SessionBurst = 100

	middleware := RateLimitMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// TestExtractSessionID tests session ID extraction from various sources.
func TestExtractSessionID(t *testing.T) {
	tests := []struct {
		name       string
		setupReq   func() *http.Request
		expectID   string
	}{
		{
			name: "query_parameter_sessionId",
			setupReq: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/?sessionId=test123", nil)
			},
			expectID: "test123",
		},
		{
			name: "query_parameter_session_id",
			setupReq: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/?session_id=test456", nil)
			},
			expectID: "test456",
		},
		{
			name: "header_X-Session-ID",
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("X-Session-ID", "header789")
				return req
			},
			expectID: "header789",
		},
		{
			name: "header_X-Session-Id",
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("X-Session-Id", "header012")
				return req
			},
			expectID: "header012",
		},
		{
			name: "cookie_session_id",
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.AddCookie(&http.Cookie{Name: "session_id", Value: "cookie345"})
				return req
			},
			expectID: "cookie345",
		},
		{
			name: "no_session_id",
			setupReq: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			expectID: "",
		},
		{
			name: "query_takes_precedence",
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/?sessionId=query", nil)
				req.Header.Set("X-Session-ID", "header")
				req.AddCookie(&http.Cookie{Name: "session_id", Value: "cookie"})
				return req
			},
			expectID: "query",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupReq()
			sessionID := extractSessionID(req)
			if sessionID != tt.expectID {
				t.Errorf("Expected session ID %q, got %q", tt.expectID, sessionID)
			}
		})
	}
}

// TestExtractRequestID tests request ID extraction from various sources.
func TestExtractRequestID(t *testing.T) {
	tests := []struct {
		name       string
		setupReq   func() *http.Request
		expectID   string
	}{
		{
			name: "header_X-Request-ID",
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("X-Request-ID", "req123")
				return req
			},
			expectID: "req123",
		},
		{
			name: "header_X-Request-Id",
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("X-Request-Id", "req456")
				return req
			},
			expectID: "req456",
		},
		{
			name: "header_Request-ID",
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("Request-ID", "req789")
				return req
			},
			expectID: "req789",
		},
		{
			name: "query_parameter_requestId",
			setupReq: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/?requestId=query123", nil)
			},
			expectID: "query123",
		},
		{
			name: "query_parameter_request_id",
			setupReq: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/?request_id=query456", nil)
			},
			expectID: "query456",
		},
		{
			name: "no_request_id",
			setupReq: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			expectID: "",
		},
		{
			name: "header_takes_precedence",
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/?requestId=query", nil)
				req.Header.Set("X-Request-ID", "header")
				return req
			},
			expectID: "header",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupReq()
			requestID := extractRequestID(req)
			if requestID != tt.expectID {
				t.Errorf("Expected request ID %q, got %q", tt.expectID, requestID)
			}
		})
	}
}

// TestWriteRateLimitError tests rate limit error response.
func TestWriteRateLimitError(t *testing.T) {
	w := httptest.NewRecorder()

	writeRateLimitError(w, "global", 100, 30*time.Second)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status %d, got %d", http.StatusTooManyRequests, w.Code)
	}

	headers := w.Header()
	if headers.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", headers.Get("Content-Type"))
	}
	if headers.Get("Retry-After") != "30" {
		t.Errorf("Expected Retry-After 30, got %s", headers.Get("Retry-After"))
	}
	if headers.Get("X-RateLimit-Limit") != "100" {
		t.Errorf("Expected X-RateLimit-Limit 100, got %s", headers.Get("X-RateLimit-Limit"))
	}
	if headers.Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("Expected X-RateLimit-Remaining 0, got %s", headers.Get("X-RateLimit-Remaining"))
	}

	// Verify JSON body
	var response map[string]any
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	errorObj, ok := response["error"].(map[string]any)
	if !ok {
		t.Fatal("Expected error object in response")
	}

	if errorObj["limit_type"] != "global" {
		t.Errorf("Expected limit_type global, got %v", errorObj["limit_type"])
	}
	if int(errorObj["limit"].(float64)) != 100 {
		t.Errorf("Expected limit 100, got %v", errorObj["limit"])
	}
}

// TestWriteDuplicateRequestError tests duplicate request error response.
func TestWriteDuplicateRequestError(t *testing.T) {
	w := httptest.NewRecorder()

	writeDuplicateRequestError(w, "test-request-id-123")

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status %d, got %d", http.StatusConflict, w.Code)
	}

	headers := w.Header()
	if headers.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", headers.Get("Content-Type"))
	}
	if headers.Get("X-Request-ID") != "test-request-id-123" {
		t.Errorf("Expected X-Request-ID test-request-id-123, got %s", headers.Get("X-Request-ID"))
	}

	// Verify JSON body
	var response map[string]any
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	errorObj, ok := response["error"].(map[string]any)
	if !ok {
		t.Fatal("Expected error object in response")
	}

	if errorObj["request_id"] != "test-request-id-123" {
		t.Errorf("Expected request_id test-request-id-123, got %v", errorObj["request_id"])
	}
}

// TestRateLimitResponseWriter tests the rate limit response writer.
func TestRateLimitResponseWriter(t *testing.T) {
	w := httptest.NewRecorder()

	rw := &rateLimitResponseWriter{
		ResponseWriter: w,
		limit:          100,
		remaining:      50,
	}

	rw.WriteHeader(http.StatusOK)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	headers := w.Header()
	if headers.Get("X-RateLimit-Limit") != "100" {
		t.Errorf("Expected X-RateLimit-Limit 100, got %s", headers.Get("X-RateLimit-Limit"))
	}
	if headers.Get("X-RateLimit-Remaining") != "50" {
		t.Errorf("Expected X-RateLimit-Remaining 50, got %s", headers.Get("X-RateLimit-Remaining"))
	}
}

// TestSessionRateLimiter tests session rate limiter.
func TestSessionRateLimiter(t *testing.T) {
	limiter := NewSessionRateLimiter(10, 10)

	sessionID := "test-session"

	// Should allow requests up to burst limit
	for i := 0; i < 10; i++ {
		if !limiter.Allow(sessionID) {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// Next request should be rate limited (burst exhausted)
	if limiter.Allow(sessionID) {
		t.Error("Request should be rate limited after burst exhausted")
	}

	// Test SetRate
	limiter.SetRate(sessionID, 100)

	// Test Remove
	limiter.Remove(sessionID)
}

// TestNewSessionRateLimiter tests session rate limiter creation.
func TestNewSessionRateLimiter_Creation(t *testing.T) {
	limiter := NewSessionRateLimiter(50, 25)

	if limiter == nil {
		t.Fatal("Expected non-nil limiter")
	}

	if limiter.qps != 50 {
		t.Errorf("Expected QPS 50, got %f", limiter.qps)
	}

	if limiter.burst != 25 {
		t.Errorf("Expected burst 25, got %f", limiter.burst)
	}
}

// TestDefaultRateLimitConfig tests default rate limit configuration.
func TestDefaultRateLimitConfig(t *testing.T) {
	config := DefaultRateLimitConfig()

	if config.GlobalQPS != 100 {
		t.Errorf("Expected GlobalQPS 100, got %f", config.GlobalQPS)
	}
	if config.GlobalBurst != 100 {
		t.Errorf("Expected GlobalBurst 100, got %f", config.GlobalBurst)
	}
	if config.SessionQPS != 10 {
		t.Errorf("Expected SessionQPS 10, got %f", config.SessionQPS)
	}
	if config.SessionBurst != 10 {
		t.Errorf("Expected SessionBurst 10, got %f", config.SessionBurst)
	}
	if config.RequestTimeout != 5*time.Minute {
		t.Errorf("Expected RequestTimeout 5m, got %v", config.RequestTimeout)
	}
}

// TestRateLimitMiddleware_RateLimited tests middleware when rate limited.
func TestRateLimitMiddleware_RateLimited(t *testing.T) {
	config := RateLimitConfig{
		GlobalQPS:    1,
		GlobalBurst:  1,
		SessionQPS:   1,
		SessionBurst: 1,
	}

	middleware := RateLimitMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware(handler)

	// First request should succeed
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("First request should succeed, got status %d", w.Code)
	}

	// Second request should be rate limited
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	w = httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Second request should be rate limited, got status %d", w.Code)
	}
}

// TestRateLimitMiddleware_WithSessionID tests middleware with session ID.
func TestRateLimitMiddleware_WithSessionID(t *testing.T) {
	config := RateLimitConfig{
		GlobalQPS:    100,
		GlobalBurst:  100,
		SessionQPS:   2,
		SessionBurst: 2,
	}

	middleware := RateLimitMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware(handler)

	// First two requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/?sessionId=test", nil)
		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d should succeed, got status %d", i+1, w.Code)
		}
	}

	// Third request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/?sessionId=test", nil)
	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Third request should be rate limited, got status %d", w.Code)
	}
}

// TestRateLimitMiddleware_WithRequestID tests middleware with request ID deduplication.
func TestRateLimitMiddleware_WithRequestID(t *testing.T) {
	config := RateLimitConfig{
		GlobalQPS:      100,
		GlobalBurst:    100,
		SessionQPS:     100,
		SessionBurst:   100,
		RequestTimeout: 1 * time.Second,
	}

	middleware := RateLimitMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware(handler)

	// First request should succeed
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "unique-request-123")
	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("First request should succeed, got status %d", w.Code)
	}

	// Duplicate request should be rejected
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "unique-request-123")
	w = httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Duplicate request should be rejected, got status %d", w.Code)
	}
}

// TestRateLimitError_JSON tests RateLimitError JSON serialization.
func TestRateLimitError_JSON(t *testing.T) {
	rateLimitErr := RateLimitError{
		LimitType:  "session",
		Limit:      10,
		Remaining:  0,
		RetryAfter: 30 * time.Second,
		Message:    "Rate limit exceeded",
	}

	data, marshalErr := json.Marshal(rateLimitErr)
	if marshalErr != nil {
		t.Fatalf("Failed to marshal RateLimitError: %v", marshalErr)
	}

	var unmarshaled RateLimitError
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal RateLimitError: %v", err)
	}

	if unmarshaled.LimitType != "session" {
		t.Errorf("Expected LimitType session, got %s", unmarshaled.LimitType)
	}
	if unmarshaled.Limit != 10 {
		t.Errorf("Expected Limit 10, got %d", unmarshaled.Limit)
	}
}

// TestDuplicateRequestError_JSON tests DuplicateRequestError JSON serialization.
func TestDuplicateRequestError_JSON(t *testing.T) {
	dupErr := DuplicateRequestError{
		RequestID: "test-123",
		Message:   "Duplicate request",
	}

	data, marshalErr := json.Marshal(dupErr)
	if marshalErr != nil {
		t.Fatalf("Failed to marshal DuplicateRequestError: %v", marshalErr)
	}

	var unmarshaled DuplicateRequestError
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal DuplicateRequestError: %v", err)
	}

	if unmarshaled.RequestID != "test-123" {
		t.Errorf("Expected RequestID test-123, got %s", unmarshaled.RequestID)
	}
}

// Benchmark tests
func BenchmarkExtractSessionID(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/?sessionId=test123", nil)
	req.Header.Set("X-Session-ID", "header")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "cookie"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractSessionID(req)
	}
}

func BenchmarkExtractRequestID(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/?requestId=test123", nil)
	req.Header.Set("X-Request-ID", "header")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractRequestID(req)
	}
}

func BenchmarkRateLimitMiddleware(b *testing.B) {
	config := DefaultRateLimitConfig()
	middleware := RateLimitMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wrappedHandler.ServeHTTP(w, req)
	}
}

// TestRateLimiter_SetCapacity tests SetCapacity function.
func TestRateLimiter_SetCapacity(t *testing.T) {
	limiter := NewRateLimiter(10, 10)
	
	// First exhaust the initial capacity
	for i := 0; i < 10; i++ {
		limiter.Allow("session1")
	}
	
	// Set new capacity
	limiter.SetCapacity("session1", 50)
	
	// Give time for tokens to refill (rate is 10 QPS, so 0.1s = 1 token)
	time.Sleep(200 * time.Millisecond)
	
	// Should allow more requests now
	allowed := 0
	for i := 0; i < 50; i++ {
		if limiter.Allow("session1") {
			allowed++
		}
	}
	
	// Should have allowed some requests (at least 2 with 200ms at 10 QPS)
	if allowed < 2 {
		t.Errorf("Expected at least 2 requests allowed after SetCapacity, got %d", allowed)
	}
}

// TestTokenBucket_AllowN tests AllowN function.
func TestTokenBucket_AllowN(t *testing.T) {
	bucket := NewTokenBucket(100, 100)
	
	// AllowN should work for valid n
	if !bucket.AllowN(10) {
		t.Error("AllowN(10) should succeed with full bucket")
	}
	
	// AllowN should fail for n > capacity
	if bucket.AllowN(200) {
		t.Error("AllowN(200) should fail when n > capacity")
	}
}
