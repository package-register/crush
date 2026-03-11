package webdav

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		wantErr bool
	}{
		{"valid URL", "http://localhost:8080/", false},
		{"valid URL without trailing slash", "http://localhost:8080", false},
		{"invalid URL", "://invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.baseURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewClient() returned nil client")
			}
		})
	}
}

func TestClient_Ping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodOptions {
			t.Errorf("Expected OPTIONS, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewClient(server.URL)
	err := client.Ping(context.Background())
	if err != nil {
		t.Errorf("Ping() error = %v", err)
	}
}

func TestClient_Get(t *testing.T) {
	expectedData := []byte("test content")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		w.Header().Set("ETag", "\"abc123\"")
		w.WriteHeader(http.StatusOK)
		w.Write(expectedData)
	}))
	defer server.Close()

	client, _ := NewClient(server.URL)
	data, err := client.Get(context.Background(), "/test.txt")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(data) != string(expectedData) {
		t.Errorf("Get() data = %s, want %s", data, expectedData)
	}
}

func TestClient_GetWithETag(t *testing.T) {
	expectedData := []byte("test content")
	expectedETag := "\"abc123\""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", expectedETag)
		w.WriteHeader(http.StatusOK)
		w.Write(expectedData)
	}))
	defer server.Close()

	client, _ := NewClient(server.URL)
	data, etag, err := client.GetWithETag(context.Background(), "/test.txt")
	if err != nil {
		t.Fatalf("GetWithETag() error = %v", err)
	}
	if string(data) != string(expectedData) {
		t.Errorf("GetWithETag() data = %s, want %s", data, expectedData)
	}
	if etag != expectedETag {
		t.Errorf("GetWithETag() etag = %s, want %s", etag, expectedETag)
	}
}

func TestClient_Put(t *testing.T) {
	testData := []byte("test content")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("Expected PUT, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, _ := NewClient(server.URL)
	err := client.Put(context.Background(), "/test.txt", testData)
	if err != nil {
		t.Errorf("Put() error = %v", err)
	}
}

func TestClient_PutIfNoneMatch(t *testing.T) {
	testData := []byte("test content")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") != "*" {
			t.Error("Expected If-None-Match: *")
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, _ := NewClient(server.URL)
	err := client.PutIfNoneMatch(context.Background(), "/test.txt", testData)
	if err != nil {
		t.Errorf("PutIfNoneMatch() error = %v", err)
	}
}

func TestClient_Delete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("Expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, _ := NewClient(server.URL)
	err := client.Delete(context.Background(), "/test.txt")
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}
}

func TestClient_MkCol(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "MKCOL" {
			t.Errorf("Expected MKCOL, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, _ := NewClient(server.URL)
	err := client.MkCol(context.Background(), "/test-dir/")
	if err != nil {
		t.Errorf("MkCol() error = %v", err)
	}
}

func TestClient_PropFind(t *testing.T) {
	response := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:">
	<D:response>
		<D:href>/test/</D:href>
		<D:propstat>
			<D:prop>
				<D:resourcetype><D:collection/></D:resourcetype>
				<D:displayname>test</D:displayname>
			</D:prop>
			<D:status>HTTP/1.1 200 OK</D:status>
		</D:propstat>
	</D:response>
</D:multistatus>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			t.Errorf("Expected PROPFIND, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Write([]byte(response))
	}))
	defer server.Close()

	client, _ := NewClient(server.URL)
	resp, err := client.PropFind(context.Background(), "/test/", 1, `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:">
	<D:prop>
		<D:resourcetype/>
		<D:displayname/>
	</D:prop>
</D:propfind>`)
	if err != nil {
		t.Fatalf("PropFind() error = %v", err)
	}
	if len(resp.Responses) != 1 {
		t.Errorf("PropFind() responses = %d, want 1", len(resp.Responses))
	}
}

func TestClient_Auth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok {
			t.Error("Expected basic auth")
		}
		if username != "testuser" || password != "testpass" {
			t.Errorf("Auth credentials = %s:%s, want testuser:testpass", username, password)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewClient(server.URL)
	client.SetAuth("testuser", "testpass")

	err := client.Ping(context.Background())
	if err != nil {
		t.Errorf("Ping() with auth error = %v", err)
	}
}

func TestClient_TokenAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Error("Expected Bearer token")
		}
		if auth != "Bearer testtoken123" {
			t.Errorf("Auth token = %s, want Bearer testtoken123", auth)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewClient(server.URL)
	client.SetToken("testtoken123")

	err := client.Ping(context.Background())
	if err != nil {
		t.Errorf("Ping() with token error = %v", err)
	}
}

func TestClient_Retry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewClient(server.URL,
		WithRetryConfig(5, 10*time.Millisecond),
	)

	err := client.Ping(context.Background())
	if err != nil {
		t.Errorf("Ping() with retry error = %v", err)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestClient_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, WithTimeout(10*time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := client.Ping(ctx)
	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestError_IsRetryable(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"408 Request Timeout", http.StatusRequestTimeout, true},
		{"429 Too Many Requests", http.StatusTooManyRequests, true},
		{"502 Bad Gateway", http.StatusBadGateway, true},
		{"503 Service Unavailable", http.StatusServiceUnavailable, true},
		{"504 Gateway Timeout", http.StatusGatewayTimeout, true},
		{"404 Not Found", http.StatusNotFound, false},
		{"401 Unauthorized", http.StatusUnauthorized, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &Error{
				Op:         "test",
				StatusCode: tt.statusCode,
				Err:        fmt.Errorf("test error"),
			}
			if got := err.IsRetryable(); got != tt.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestError_IsConflict(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"409 Conflict", http.StatusConflict, true},
		{"412 Precondition Failed", http.StatusPreconditionFailed, true},
		{"404 Not Found", http.StatusNotFound, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &Error{
				Op:         "test",
				StatusCode: tt.statusCode,
				Err:        fmt.Errorf("test error"),
			}
			if got := err.IsConflict(); got != tt.want {
				t.Errorf("IsConflict() = %v, want %v", got, tt.want)
			}
		})
	}
}
