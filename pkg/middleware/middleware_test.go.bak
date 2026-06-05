package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/zhufuyi/stasrv/pkg/logger"
)

func TestGenerateTraceID(t *testing.T) {
	t.Run("Length and Format", func(t *testing.T) {
		id := GenerateTraceID()
		if len(id) != 32 {
			t.Errorf("expected trace ID length 32, got %d", len(id))
		}
	})

	t.Run("Uniqueness", func(t *testing.T) {
		m := make(map[string]struct{})
		for i := 0; i < 1000; i++ {
			id := GenerateTraceID()
			if _, exists := m[id]; exists {
				t.Fatalf("duplicate trace ID generated: %s", id)
			}
			m[id] = struct{}{}
		}
	})
}

func TestLogOptions(t *testing.T) {
	var buf bytes.Buffer
	customLogger := slog.New(slog.NewJSONHandler(&buf, nil))

	opts := defaultLogOptions()
	opts.apply(
		WithSkipPaths("/custom-skip"),
		WithSkipPrefixPaths("/custom-skip-prefix"),
		WithTraceIDHeader("X-Custom-Trace-Id"),
		WithSlogLogger(customLogger),
	)

	if _, ok := opts.skipPaths["/custom-skip"]; !ok {
		t.Error("expected /custom-skip in skipPaths")
	}
	if opts.traceIDHeader != "X-Custom-Trace-Id" {
		t.Errorf("expected header X-Custom-Trace-Id, got %s", opts.traceIDHeader)
	}
	if opts.logger != customLogger {
		t.Error("expected custom logger to be set")
	}
}

func TestSlogLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	parseLog := func(t *testing.T, buf *bytes.Buffer) map[string]any {
		var logData map[string]any
		if err := json.Unmarshal(buf.Bytes(), &logData); err != nil {
			t.Fatalf("failed to parse log JSON: %v", err)
		}
		return logData
	}

	t.Run("Normal Request (200 OK)", func(t *testing.T) {
		var buf bytes.Buffer
		l := slog.New(slog.NewJSONHandler(&buf, nil))
		r := gin.New()
		r.Use(SlogLogger(WithSlogLogger(l)))
		r.Use(Cors())

		r.GET("/ping", func(c *gin.Context) {
			ctxTraceID := c.Request.Context().Value(logger.ContextTraceIDKey)
			if ctxTraceID == nil || ctxTraceID == "" {
				t.Error("expected TraceID in context")
			}
			c.String(http.StatusOK, "pong")
		})

		req := httptest.NewRequest(http.MethodGet, "/ping?foo=bar", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		traceID := w.Header().Get(defaultTraceIDHeader)
		if traceID == "" {
			t.Error("expected trace ID in response header")
		}

		logData := parseLog(t, &buf)
		if logData["level"] != "INFO" {
			t.Errorf("expected level INFO, got %v", logData["level"])
		}
		if logData["status"].(float64) != 200 {
			t.Errorf("expected status 200, got %v", logData["status"])
		}
		if logData["path"] != "/ping" {
			t.Errorf("expected path /ping, got %v", logData["path"])
		}
		if logData["query"] != "foo=bar" {
			t.Errorf("expected query foo=bar, got %v", logData["query"])
		}
	})

	t.Run("Existing Trace ID", func(t *testing.T) {
		var buf bytes.Buffer
		l := slog.New(slog.NewJSONHandler(&buf, nil))
		r := gin.New()
		r.Use(SlogLogger(WithSlogLogger(l)))
		r.GET("/ping", func(c *gin.Context) { c.Status(200) })

		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.Header.Set(defaultTraceIDHeader, "custom-trace-id-123")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Header().Get(defaultTraceIDHeader) != "custom-trace-id-123" {
			t.Errorf("expected custom trace ID, got %s", w.Header().Get(defaultTraceIDHeader))
		}
	})

	t.Run("Skip Paths", func(t *testing.T) {
		var buf bytes.Buffer
		l := slog.New(slog.NewJSONHandler(&buf, nil))
		r := gin.New()
		r.Use(SlogLogger(WithSlogLogger(l)))
		r.GET("/health", func(c *gin.Context) { c.Status(200) })

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if buf.Len() > 0 {
			t.Error("expected no logs for skipped path")
		}
	})

	t.Run("Warning Log (400 Bad Request)", func(t *testing.T) {
		var buf bytes.Buffer
		l := slog.New(slog.NewJSONHandler(&buf, nil))
		r := gin.New()
		r.Use(SlogLogger(WithSlogLogger(l)))
		r.GET("/warn", func(c *gin.Context) { c.Status(400) })

		req := httptest.NewRequest(http.MethodGet, "/warn", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		logData := parseLog(t, &buf)
		if logData["level"] != "WARN" {
			t.Errorf("expected level WARN, got %v", logData["level"])
		}
	})

	t.Run("Error Log (500 Internal Server Error)", func(t *testing.T) {
		var buf bytes.Buffer
		l := slog.New(slog.NewJSONHandler(&buf, nil))
		r := gin.New()
		r.Use(SlogLogger(WithSlogLogger(l)))
		r.GET("/err", func(c *gin.Context) { c.Status(500) })

		req := httptest.NewRequest(http.MethodGet, "/err", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		logData := parseLog(t, &buf)
		if logData["level"] != "ERROR" {
			t.Errorf("expected level ERROR, got %v", logData["level"])
		}
	})

	t.Run("Gin Errors Logging", func(t *testing.T) {
		var buf bytes.Buffer
		l := slog.New(slog.NewJSONHandler(&buf, nil))
		r := gin.New()
		r.Use(SlogLogger(WithSlogLogger(l)))
		r.GET("/gin-err", func(c *gin.Context) {
			_ = c.Error(errors.New("first error"))
			_ = c.Error(errors.New("second error"))
			c.Status(500)
		})

		req := httptest.NewRequest(http.MethodGet, "/gin-err", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		logData := parseLog(t, &buf)
		expectedErrStr := "first error; second error"
		if logData["errors"] != expectedErrStr {
			t.Errorf("expected errors '%s', got %v", expectedErrStr, logData["errors"])
		}
	})
}
