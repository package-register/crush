// Package aguiserver implements the AG-UI Protocol server for Crush.
// It provides SSE-based streaming of AG-UI events to external clients.
package aguiserver

import (
	"bytes"
	"sync"
)

// eventBufferPool is a pool of bytes.Buffer instances for event serialization.
// Using sync.Pool reduces memory allocations and GC pressure.
var eventBufferPool = sync.Pool{
	New: func() interface{} {
		return &bytes.Buffer{}
	},
}

// getEventBuffer retrieves a buffer from the pool.
// The caller must call putEventBuffer when done with the buffer.
func getEventBuffer() *bytes.Buffer {
	return eventBufferPool.Get().(*bytes.Buffer)
}

// putEventBuffer returns a buffer to the pool after resetting it.
// This should be called when the buffer is no longer needed.
func putEventBuffer(buf *bytes.Buffer) {
	if buf != nil {
		buf.Reset()
		eventBufferPool.Put(buf)
	}
}

// jsonEncoderPool is a pool of json.Encoder instances for event serialization.
// Reusing encoders reduces allocations and improves performance.
var jsonEncoderPool = sync.Pool{
	New: func() interface{} {
		return &bytes.Buffer{}
	},
}

// getJSONBuffer retrieves a buffer from the JSON encoder pool.
// The caller must call putJSONBuffer when done with the buffer.
func getJSONBuffer() *bytes.Buffer {
	return jsonEncoderPool.Get().(*bytes.Buffer)
}

// putJSONBuffer returns a buffer to the JSON encoder pool after resetting it.
// This should be called when the buffer is no longer needed.
func putJSONBuffer(buf *bytes.Buffer) {
	if buf != nil {
		buf.Reset()
		jsonEncoderPool.Put(buf)
	}
}
