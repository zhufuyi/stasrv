package logger

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestOptions(t *testing.T) {
	opts := defaultOptions()
	if opts.isJSONFormat != false {
		t.Errorf("expected default isJSONFormat to be false, got %v", opts.isJSONFormat)
	}
	if opts.level != slog.LevelInfo {
		t.Errorf("expected default level to be Info, got %v", opts.level)
	}

	opts.apply(WithJSONFormat(true), WithLevel(slog.LevelDebug))
	if opts.isJSONFormat != true {
		t.Errorf("expected isJSONFormat to be true, got %v", opts.isJSONFormat)
	}
	if opts.level != slog.LevelDebug {
		t.Errorf("expected level to be Debug, got %v", opts.level)
	}
}

func TestGetTraceID(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{
			name:     "nil context",
			ctx:      nil,
			expected: "",
		},
		{
			name:     "empty context",
			ctx:      context.Background(),
			expected: "",
		},
		{
			name:     "context with valid trace id",
			ctx:      context.WithValue(context.Background(), ContextTraceIDKey, "trace-12345"),
			expected: "trace-12345",
		},
		{
			name:     "context with empty trace id",
			ctx:      context.WithValue(context.Background(), ContextTraceIDKey, ""),
			expected: "",
		},
		{
			name:     "context with invalid type",
			ctx:      context.WithValue(context.Background(), ContextTraceIDKey, 12345),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := GetTraceID(tt.ctx)
			if actual != tt.expected {
				t.Errorf("GetTraceID() = %v, expected %v", actual, tt.expected)
			}
		})
	}
}

func TestLoggerInitAndGet(t *testing.T) {
	originalLogger := globalLogger
	defer func() { globalLogger = originalLogger }()

	globalLogger = nil
	logger1 := Get()
	if logger1 == nil {
		t.Fatal("Get() returned nil before Init()")
	}

	Init(WithJSONFormat(true), WithLevel(slog.LevelDebug))
	logger2 := Get()
	if logger2 == nil {
		t.Fatal("Get() returned nil after Init()")
	}
	if logger1 == logger2 {
		t.Fatal("Expected a new logger instance after Init()")
	}
}

func TestContextHandler(t *testing.T) {
	var buf bytes.Buffer

	baseHandler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	h := &contextHandler{Handler: baseHandler}
	logger := slog.New(h)

	logger.InfoContext(context.Background(), "message without trace id")
	if strings.Contains(buf.String(), "trace_id") {
		t.Errorf("did not expect trace_id in log, got: %s", buf.String())
	}
	buf.Reset()

	ctx := context.WithValue(context.Background(), ContextTraceIDKey, "test-trace-999")
	logger.InfoContext(ctx, "message with trace id")

	output := buf.String()
	if !strings.Contains(output, "trace_id=test-trace-999") {
		t.Errorf("expected trace_id in log, got: %s", output)
	}
}

func TestAsyncWriterInitialization(t *testing.T) {
	var buf bytes.Buffer
	sizeLimit := 1024
	interval := 10 * time.Millisecond

	writer := NewAsyncWriter(&buf, sizeLimit, interval)

	if writer.dest != &buf {
		t.Errorf("expected dest to be set correctly")
	}
	if writer.sizeLimit != sizeLimit {
		t.Errorf("expected sizeLimit to be %d, got %d", sizeLimit, writer.sizeLimit)
	}
	if writer.flushInterval != interval {
		t.Errorf("expected flushInterval to be %v, got %v", interval, writer.flushInterval)
	}
	if writer.activeBuf == nil || writer.flushBuf == nil {
		t.Errorf("expected buffers to be initialized")
	}
	if writer.activeBuf.Cap() != sizeLimit {
		t.Errorf("expected activeBuf capacity to be %d, got %d", sizeLimit, writer.activeBuf.Cap())
	}
}
