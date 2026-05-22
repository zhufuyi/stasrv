package logger

import (
	"bytes"
	"io"
	"os"
	"sync"
	"time"
)

// initialize asynchronous writer: buffer 256KB, forced flush every 50 ms
var globalAsyncWriter = NewAsyncWriter(os.Stdout, 256*1024, 50*time.Millisecond)

// GetAsyncWriter returns the global asynchronous writer.
func GetAsyncWriter() *AsyncWriter {
	return globalAsyncWriter
}

// AsyncWriter is a high-performance non-blocking double-buffered asynchronous writer.
type AsyncWriter struct {
	mu            sync.Mutex
	dest          io.Writer
	activeBuf     *bytes.Buffer
	flushBuf      *bytes.Buffer
	sizeLimit     int
	flushInterval time.Duration
	notify        chan struct{}
	closeChan     chan struct{}
}

// NewAsyncWriter create an async writer
func NewAsyncWriter(dest io.Writer, sizeLimit int, interval time.Duration) *AsyncWriter {
	w := &AsyncWriter{
		dest:          dest,
		activeBuf:     bytes.NewBuffer(make([]byte, 0, sizeLimit)),
		flushBuf:      bytes.NewBuffer(make([]byte, 0, sizeLimit)),
		sizeLimit:     sizeLimit,
		flushInterval: interval,
		notify:        make(chan struct{}, 1),
		closeChan:     make(chan struct{}),
	}
	go w.backendLoop()
	return w
}

// Write implements the io.Writer interface.
func (w *AsyncWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	n, err = w.activeBuf.Write(p)
	// Notify the background goroutine to flush
	// when the buffer reaches the configured threshold.
	if w.activeBuf.Len() >= w.sizeLimit {
		select {
		case w.notify <- struct{}{}:
		default:
		}
	}
	w.mu.Unlock()
	return n, err
}

// backendLoop is the dedicated background flushing goroutine.
// It is the only goroutine responsible for performing actual I/O syscalls.
func (w *AsyncWriter) backendLoop() {
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.flush()
		case <-w.notify:
			w.flush()
		case <-w.closeChan:
			w.flush()
			return
		}
	}
}

func (w *AsyncWriter) flush() {
	w.mu.Lock()
	if w.activeBuf.Len() == 0 {
		w.mu.Unlock()
		return
	}

	// Core optimization:
	// instantly swap the two buffer pointers so the business goroutine
	// can release the lock immediately.
	w.activeBuf, w.flushBuf = w.flushBuf, w.activeBuf
	w.mu.Unlock()

	// Perform I/O safely and efficiently in a completely lock-free state.
	_, _ = w.dest.Write(w.flushBuf.Bytes())
	w.flushBuf.Reset()
}

// Close gracefully shuts down the asynchronous writer.
// Typically called before the application exits.
func (w *AsyncWriter) Close() error {
	close(w.closeChan)
	return nil
}

// CloseAsyncLogger gracefully closes the global asynchronous logger.
func CloseAsyncLogger() {
	if globalAsyncWriter != nil {
		_ = globalAsyncWriter.Close()
	}
}
