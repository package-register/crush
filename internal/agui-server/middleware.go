package aguiserver

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// RateLimitConfig holds the configuration for rate limiting.
type RateLimitConfig struct {
	GlobalQPS      float64 // Global queries per second limit
	GlobalBurst    float64 // Global burst size
	SessionQPS     float64 // Per-session queries per second limit
	SessionBurst   float64 // Per-session burst size
	RequestTimeout time.Duration // Request ID tracking timeout
}

// DefaultRateLimitConfig returns the default rate limit configuration.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		GlobalQPS:      100,
		GlobalBurst:    100,
		SessionQPS:     10,
		SessionBurst:   10,
		RequestTimeout: 5 * time.Minute,
	}
}

// RateLimitMiddleware creates a middleware that applies rate limiting.
// It enforces both global and session-level rate limits.
// Note: The middleware creates a RequestTracker with a background goroutine.
// For long-running servers, consider using NewRateLimitMiddlewareWithCleanup.
func RateLimitMiddleware(config RateLimitConfig) func(http.Handler) http.Handler {
	globalLimiter := NewTokenBucket(config.GlobalBurst, config.GlobalQPS)
	sessionLimiters := NewRateLimiter(config.SessionQPS, config.SessionBurst)
	requestTracker := NewRequestTracker(config.RequestTimeout, config.RequestTimeout/2)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract session ID from request
			sessionID := extractSessionID(r)

			// Check global rate limit
			if !globalLimiter.Allow() {
				writeRateLimitError(w, "global", int(config.GlobalQPS), 1*time.Second)
				return
			}

			// Check session rate limit
			if sessionID != "" {
				if !sessionLimiters.Allow(sessionID) {
					writeRateLimitError(w, "session", int(config.SessionQPS), 1*time.Second)
					return
				}
			}

			// Check for duplicate request (RequestID deduplication)
			requestID := extractRequestID(r)
			if requestID != "" {
				if !requestTracker.Track(requestID) {
					writeDuplicateRequestError(w, requestID)
					return
				}
			}

			// Add rate limit headers to response
			w = &rateLimitResponseWriter{
				ResponseWriter: w,
				limit:          int(config.GlobalQPS),
				remaining:      int(globalLimiter.Tokens()),
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitMiddlewareWithCleanup creates a rate limiting middleware with cleanup support.
// Call the returned cleanup function to release resources when the middleware is no longer needed.
func RateLimitMiddlewareWithCleanup(config RateLimitConfig) (func(http.Handler) http.Handler, func()) {
	globalLimiter := NewTokenBucket(config.GlobalBurst, config.GlobalQPS)
	sessionLimiters := NewRateLimiter(config.SessionQPS, config.SessionBurst)
	requestTracker := NewRequestTracker(config.RequestTimeout, config.RequestTimeout/2)

	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionID := extractSessionID(r)

			if !globalLimiter.Allow() {
				writeRateLimitError(w, "global", int(config.GlobalQPS), 1*time.Second)
				return
			}

			if sessionID != "" {
				if !sessionLimiters.Allow(sessionID) {
					writeRateLimitError(w, "session", int(config.SessionQPS), 1*time.Second)
					return
				}
			}

			requestID := extractRequestID(r)
			if requestID != "" {
				if !requestTracker.Track(requestID) {
					writeDuplicateRequestError(w, requestID)
					return
				}
			}

			w = &rateLimitResponseWriter{
				ResponseWriter: w,
				limit:          int(config.GlobalQPS),
				remaining:      int(globalLimiter.Tokens()),
			}

			next.ServeHTTP(w, r)
		})
	}

	return middleware, requestTracker.Close
}

// extractSessionID extracts the session ID from the request.
// It checks query parameters, headers, and cookies in that order.
func extractSessionID(r *http.Request) string {
	// Check query parameter
	if sessionID := r.URL.Query().Get("sessionId"); sessionID != "" {
		return sessionID
	}
	if sessionID := r.URL.Query().Get("session_id"); sessionID != "" {
		return sessionID
	}

	// Check header
	if sessionID := r.Header.Get("X-Session-ID"); sessionID != "" {
		return sessionID
	}
	if sessionID := r.Header.Get("X-Session-Id"); sessionID != "" {
		return sessionID
	}

	// Check cookie
	if cookie, err := r.Cookie("session_id"); err == nil {
		return cookie.Value
	}

	return ""
}

// extractRequestID extracts the request ID from the request.
// It checks headers and query parameters.
func extractRequestID(r *http.Request) string {
	// Check header
	if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
		return requestID
	}
	if requestID := r.Header.Get("X-Request-Id"); requestID != "" {
		return requestID
	}
	if requestID := r.Header.Get("Request-ID"); requestID != "" {
		return requestID
	}

	// Check query parameter
	if requestID := r.URL.Query().Get("requestId"); requestID != "" {
		return requestID
	}
	if requestID := r.URL.Query().Get("request_id"); requestID != "" {
		return requestID
	}

	return ""
}

// RateLimitError represents a rate limit exceeded error.
type RateLimitError struct {
	LimitType  string        `json:"limit_type"`  // "global" or "session"
	Limit      int           `json:"limit"`       // The rate limit value
	Remaining  int           `json:"remaining"`   // Remaining quota
	RetryAfter time.Duration `json:"retry_after"` // Suggested retry time
	Message    string        `json:"message"`     // Human-readable message
}

// DuplicateRequestError represents a duplicate request error.
type DuplicateRequestError struct {
	RequestID string `json:"request_id"` // The duplicate request ID
	Message   string `json:"message"`    // Human-readable message
}

// writeRateLimitError writes a rate limit error response.
func writeRateLimitError(w http.ResponseWriter, limitType string, limit int, retryAfter time.Duration) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
	w.Header().Set("X-RateLimit-Remaining", "0")
	w.Header().Set("X-RateLimit-Reset", strconv.Itoa(int(time.Now().Add(retryAfter).Unix())))
	w.WriteHeader(http.StatusTooManyRequests)

	err := RateLimitError{
		LimitType:  limitType,
		Limit:      limit,
		Remaining:  0,
		RetryAfter: retryAfter,
		Message:    "Too many requests, please retry after " + strconv.Itoa(int(retryAfter.Seconds())) + " seconds",
	}

	json.NewEncoder(w).Encode(map[string]any{
		"error": err,
	})
}

// writeDuplicateRequestError writes a duplicate request error response.
func writeDuplicateRequestError(w http.ResponseWriter, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", requestID)
	w.WriteHeader(http.StatusConflict)

	err := DuplicateRequestError{
		RequestID: requestID,
		Message:   "Request with ID " + requestID + " is already being processed",
	}

	json.NewEncoder(w).Encode(map[string]any{
		"error": err,
	})
}

// rateLimitResponseWriter wraps http.ResponseWriter to add rate limit headers.
type rateLimitResponseWriter struct {
	http.ResponseWriter
	limit     int
	remaining int
}

// WriteHeader adds rate limit headers before writing the response.
func (w *rateLimitResponseWriter) WriteHeader(code int) {
	w.ResponseWriter.Header().Set("X-RateLimit-Limit", strconv.Itoa(w.limit))
	w.ResponseWriter.Header().Set("X-RateLimit-Remaining", strconv.Itoa(w.remaining))
	w.ResponseWriter.WriteHeader(code)
}

// SessionRateLimiter provides session-level rate limiting utilities.
type SessionRateLimiter struct {
	limiters *RateLimiter
	qps      float64
	burst    float64
}

// NewSessionRateLimiter creates a new session rate limiter.
func NewSessionRateLimiter(qps, burst float64) *SessionRateLimiter {
	return &SessionRateLimiter{
		limiters: NewRateLimiter(qps, burst),
		qps:      qps,
		burst:    burst,
	}
}

// Allow checks if a request is allowed for the given session.
func (s *SessionRateLimiter) Allow(sessionID string) bool {
	return s.limiters.Allow(sessionID)
}

// SetRate sets the rate limit for a specific session.
func (s *SessionRateLimiter) SetRate(sessionID string, qps float64) {
	s.limiters.SetRate(sessionID, qps)
}

// Remove removes the rate limiter for a session.
func (s *SessionRateLimiter) Remove(sessionID string) {
	s.limiters.Remove(sessionID)
}
