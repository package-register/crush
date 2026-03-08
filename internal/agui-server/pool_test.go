package aguiserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

// BenchmarkConnectionPool benchmarks the connection creation with pool optimization.
func BenchmarkConnectionPool(b *testing.B) {
	manager := NewConnectionManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn := NewConnection(
			fmt.Sprintf("conn-%d", i),
			fmt.Sprintf("thread-%d", i/100),
			fmt.Sprintf("run-%d", i),
		)
		manager.Add(conn)
		manager.Remove(conn.ID)
		close(conn.Done)
	}
}

// BenchmarkEventSerialization benchmarks event serialization with and without pool.
func BenchmarkEventSerializationPooled(b *testing.B) {
	event := NewEvent(RunStarted, RunStartedEvent{
		ThreadID: "test-thread",
		RunID:    "test-run",
	})

	b.Run("WithPool", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := getEventBuffer()
			json.NewEncoder(buf).Encode(event)
			putEventBuffer(buf)
		}
	})

	b.Run("WithoutPool", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := &bytes.Buffer{}
			json.NewEncoder(buf).Encode(event)
		}
	})
}

// BenchmarkEventMarshalJSON benchmarks MarshalJSON with and without pool.
func BenchmarkEventMarshalJSONPooled(b *testing.B) {
	event := NewEvent(RunStarted, RunStartedEvent{
		ThreadID: "test-thread",
		RunID:    "test-run",
	})

	b.Run("WithPool", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = event.MarshalJSON()
		}
	})

	b.Run("WithoutPool", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = json.Marshal(event)
		}
	})
}

// BenchmarkEventString benchmarks String method with and without pool.
func BenchmarkEventStringPooled(b *testing.B) {
	event := NewEvent(RunStarted, RunStartedEvent{
		ThreadID: "test-thread",
		RunID:    "test-run",
	})

	b.Run("WithPool", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = event.String()
		}
	})

	b.Run("WithoutPool", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			data, _ := json.Marshal(event)
			_ = string(data)
		}
	})
}

// BenchmarkBufferPoolConcurrency benchmarks concurrent access to the buffer pool.
func BenchmarkBufferPoolConcurrency(b *testing.B) {
	event := NewEvent(RunStarted, RunStartedEvent{
		ThreadID: "test-thread",
		RunID:    "test-run",
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := getEventBuffer()
			json.NewEncoder(buf).Encode(event)
			putEventBuffer(buf)
		}
	})
}

// BenchmarkConnectionManagerConcurrency benchmarks concurrent connection management.
func BenchmarkConnectionManagerConcurrency(b *testing.B) {
	manager := NewConnectionManager()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			conn := NewConnection(
				fmt.Sprintf("conn-%d", i),
				fmt.Sprintf("thread-%d", i%100),
				fmt.Sprintf("run-%d", i),
			)
			manager.Add(conn)
			manager.Remove(conn.ID)
			close(conn.Done)
			i++
		}
	})
}

// BenchmarkSSEHandlerSendEvent benchmarks the sendEvent method.
func BenchmarkSSEHandlerSendEvent(b *testing.B) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	event := NewEvent(RunStarted, RunStartedEvent{
		ThreadID: "test-thread",
		RunID:    "test-run",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := &mockResponseWriter{buf: &bytes.Buffer{}}
		handler.sendEvent(w, event)
	}
}

// mockResponseWriter is a mock http.ResponseWriter for benchmarking.
type mockResponseWriter struct {
	buf  *bytes.Buffer
	code int
}

func (m *mockResponseWriter) Header() http.Header {
	return http.Header{}
}

func (m *mockResponseWriter) Write(p []byte) (int, error) {
	return m.buf.Write(p)
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.code = statusCode
}

func (m *mockResponseWriter) Flush() {}

// TestPoolSafety tests the thread safety of the buffer pool.
func TestPoolSafety(t *testing.T) {
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Start multiple goroutines to concurrently use the pool
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				buf := getEventBuffer()
				if buf == nil {
					errors <- fmt.Errorf("got nil buffer from pool")
					return
				}

				// Write some data
				event := NewEvent(RunStarted, RunStartedEvent{
					ThreadID: fmt.Sprintf("thread-%d", id),
					RunID:    fmt.Sprintf("run-%d", j),
				})
				json.NewEncoder(buf).Encode(event)

				// Verify buffer is working correctly
				if buf.Len() == 0 {
					errors <- fmt.Errorf("buffer should have data")
					return
				}

				putEventBuffer(buf)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}
}

// TestPoolBufferReset tests that buffers are properly reset when returned to pool.
func TestPoolBufferReset(t *testing.T) {
	buf := getEventBuffer()
	buf.WriteString("test data")

	if buf.Len() == 0 {
		t.Error("buffer should have data before reset")
	}

	putEventBuffer(buf)

	// Get the same buffer again and verify it's reset
	buf2 := getEventBuffer()
	if buf2.Len() != 0 {
		t.Error("buffer should be reset after return to pool")
	}

	putEventBuffer(buf2)
}

// TestPoolNilSafety tests that putEventBuffer handles nil safely.
func TestPoolNilSafety(t *testing.T) {
	// Should not panic
	putEventBuffer(nil)
	putJSONBuffer(nil)
}

// TestPoolDataIsolation tests that data from previous uses doesn't leak.
func TestPoolDataIsolation(t *testing.T) {
	buf := getEventBuffer()
	buf.WriteString("sensitive data")
	putEventBuffer(buf)

	buf2 := getEventBuffer()
	if buf2.Len() != 0 {
		t.Error("buffer should be empty after reset")
	}

	// Write new data
	buf2.WriteString("new data")
	data := buf2.String()
	if data != "new data" {
		t.Errorf("expected 'new data', got %q", data)
	}

	putEventBuffer(buf2)
}

// BenchmarkAllocationComparison compares allocations between pooled and non-pooled approaches.
func BenchmarkAllocationComparison(b *testing.B) {
	event := NewEvent(TextMessageContent, TextMessageContentEvent{
		MessageID: "msg-123",
		Content:   "Hello, World!",
	})

	b.Run("PooledEncode", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := getEventBuffer()
			json.NewEncoder(buf).Encode(event)
			putEventBuffer(buf)
		}
	})

	b.Run("NonPooledEncode", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := &bytes.Buffer{}
			json.NewEncoder(buf).Encode(event)
		}
	})

	b.Run("PooledMarshal", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = event.MarshalJSON()
		}
	})

	b.Run("NonPooledMarshal", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = json.Marshal(event)
		}
	})
}

// BenchmarkHeartbeatWithPool benchmarks heartbeat event creation with pool.
func BenchmarkHeartbeatWithPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		event := NewEvent(CustomEvent, map[string]string{"type": "heartbeat"})
		buf := getEventBuffer()
		json.NewEncoder(buf).Encode(event)
		putEventBuffer(buf)
	}
}

// BenchmarkLargeEventSerialization benchmarks serialization of large events.
func BenchmarkLargeEventSerialization(b *testing.B) {
	// Create a large state delta event
	largeDelta := make(map[string]any)
	for i := 0; i < 100; i++ {
		largeDelta[fmt.Sprintf("key-%d", i)] = fmt.Sprintf("value-%d with some extra content", i)
	}

	event := NewEvent(StateDelta, StateDeltaEvent{
		Delta: largeDelta,
	})

	b.Run("Pooled", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := getEventBuffer()
			json.NewEncoder(buf).Encode(event)
			putEventBuffer(buf)
		}
	})

	b.Run("NonPooled", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := &bytes.Buffer{}
			json.NewEncoder(buf).Encode(event)
		}
	})
}

// TestBufferPoolPerformance verifies buffer pool functionality.
func TestBufferPoolPerformance(t *testing.T) {
	event := NewEvent(RunStarted, RunStartedEvent{
		ThreadID: "test-thread",
		RunID:    "test-run",
	})

	// Warm up pool
	for i := 0; i < 10; i++ {
		buf := getEventBuffer()
		json.NewEncoder(buf).Encode(event)
		putEventBuffer(buf)
	}

	// Measure pooled performance
	startPooled := time.Now()
	for i := 0; i < 1000; i++ {
		buf := getEventBuffer()
		json.NewEncoder(buf).Encode(event)
		putEventBuffer(buf)
	}
	pooledDuration := time.Since(startPooled)

	// Measure non-pooled performance
	startNonPooled := time.Now()
	for i := 0; i < 1000; i++ {
		buf := &bytes.Buffer{}
		json.NewEncoder(buf).Encode(event)
	}
	nonPooledDuration := time.Since(startNonPooled)

	t.Logf("Pooled: %v, Non-pooled: %v", pooledDuration, nonPooledDuration)

	// Just verify both approaches complete successfully
	// Performance comparison is environment-dependent
	if pooledDuration <= 0 {
		t.Error("Pooled duration should be positive")
	}
	if nonPooledDuration <= 0 {
		t.Error("Non-pooled duration should be positive")
	}
}
