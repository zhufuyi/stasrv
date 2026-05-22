package logger

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAsyncWriter_WriteAndFlushByTicker(t *testing.T) {
	var buf bytes.Buffer

	w := NewAsyncWriter(&buf, 1024, 20*time.Millisecond)
	defer w.Close()

	msg := "hello async logger"

	n, err := w.Write([]byte(msg))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if n != len(msg) {
		t.Fatalf("unexpected write length: got=%d want=%d", n, len(msg))
	}

	time.Sleep(50 * time.Millisecond)

	got := buf.String()

	if got != msg {
		t.Fatalf("unexpected flushed content: got=%q want=%q", got, msg)
	}
}

func TestAsyncWriter_FlushBySizeLimit(t *testing.T) {
	var buf bytes.Buffer

	w := NewAsyncWriter(&buf, 8, time.Hour)
	defer w.Close()

	msg := "12345678"

	_, err := w.Write([]byte(msg))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	got := buf.String()

	if got != msg {
		t.Fatalf("unexpected flushed content: got=%q want=%q", got, msg)
	}
}

func TestAsyncWriter_ConcurrentWrite(t *testing.T) {
	var buf bytes.Buffer

	w := NewAsyncWriter(&buf, 1024, 10*time.Millisecond)
	defer w.Close()

	const goroutines = 20
	const perGoroutine = 100

	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for j := 0; j < perGoroutine; j++ {
				_, err := w.Write([]byte("x"))
				if err != nil {
					t.Errorf("write failed: %v", err)
				}
			}
		}()
	}

	wg.Wait()

	time.Sleep(100 * time.Millisecond)

	gotLen := len(buf.String())
	wantLen := goroutines * perGoroutine

	if gotLen != wantLen {
		t.Fatalf("unexpected total length: got=%d want=%d", gotLen, wantLen)
	}
}

func TestAsyncWriter_CloseFlushesRemainingData(t *testing.T) {
	var buf bytes.Buffer

	w := NewAsyncWriter(&buf, 1024, time.Hour)

	msg := "flush before close"

	_, err := w.Write([]byte(msg))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	err = w.Close()
	if err != nil {
		t.Fatalf("close failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	got := buf.String()

	if got != msg {
		t.Fatalf("unexpected flushed content after close: got=%q want=%q", got, msg)
	}
}

func TestGetAsyncWriter(t *testing.T) {
	w := GetAsyncWriter()

	if w == nil {
		t.Fatal("GetAsyncWriter returned nil")
	}
}

func TestAsyncWriter_MultipleWrites(t *testing.T) {
	var buf bytes.Buffer

	w := NewAsyncWriter(&buf, 1024, 20*time.Millisecond)
	defer w.Close()

	messages := []string{
		"hello",
		" ",
		"world",
		"\n",
	}

	for _, msg := range messages {
		_, err := w.Write([]byte(msg))
		if err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}

	time.Sleep(50 * time.Millisecond)

	got := buf.String()

	want := strings.Join(messages, "")

	if got != want {
		t.Fatalf("unexpected flushed content: got=%q want=%q", got, want)
	}
}
