package middleware

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewLogger_CanLog(t *testing.T) {
	logger := newDefaultLogger()

	core, recorded := observer.New(zapcore.InfoLevel)
	logger = logger.WithOptions(zap.WrapCore(func(zapcore.Core) zapcore.Core { return core }))

	logger.Info("test message", zap.String("key", "value"))

	entries := recorded.All()
	require.Len(t, entries, 1, "should have exactly one log entry")
	entry := entries[0]
	assert.Equal(t, zapcore.InfoLevel, entry.Level)
	assert.Equal(t, "test message", entry.Message)

	assert.Contains(t, entry.ContextMap(), "key")
}

func TestDefaultAccessLogOptions(t *testing.T) {
	opts := defaultAccessLogOptions()

	assert.True(t, opts.enable, "default should enable access log")
	assert.NotNil(t, opts.log, "default logger should not be nil")
	assert.Equal(t, DefaultSlowThreshold, opts.slowThreshold)
	assert.Equal(t, 0, opts.minStatusCode)

	assert.Len(t, opts.ignoreRoutes, len(defaultIgnoreRoutes))
	for route := range defaultIgnoreRoutes {
		_, exists := opts.ignoreRoutes[route]
		assert.True(t, exists, "default ignoreRoutes should contain %s", route)
	}
}

func TestWithEnable(t *testing.T) {
	opts := &accessLogOptions{enable: true}
	WithEnable(false)(opts)
	assert.False(t, opts.enable)

	WithEnable(true)(opts)
	assert.True(t, opts.enable)
}

func TestWithLogger(t *testing.T) {
	custom := zap.NewNop()
	opts := &accessLogOptions{log: nil}
	WithLogger(custom)(opts)
	assert.Same(t, custom, opts.log)

	// nil logger should be ignored
	prev := opts.log
	WithLogger(nil)(opts)
	assert.Same(t, prev, opts.log)
}

func TestWithSlowThreshold(t *testing.T) {
	opts := &accessLogOptions{}
	WithSlowThreshold(100 * time.Millisecond)(opts)
	assert.Equal(t, 100*time.Millisecond, opts.slowThreshold)

	// zero or negative duration should be ignored
	opts.slowThreshold = 0
	WithSlowThreshold(0)(opts)
	assert.Equal(t, time.Duration(0), opts.slowThreshold)

	WithSlowThreshold(-time.Second)(opts)
	assert.Equal(t, time.Duration(0), opts.slowThreshold)
}

func TestWithMinStatusCode(t *testing.T) {
	opts := &accessLogOptions{}
	WithMinStatusCode(400)(opts)
	assert.Equal(t, 400, opts.minStatusCode)

	// negative code should be ignored
	opts.minStatusCode = 0
	WithMinStatusCode(-100)(opts)
	assert.Equal(t, 0, opts.minStatusCode)

	// zero is allowed
	WithMinStatusCode(0)(opts)
	assert.Equal(t, 0, opts.minStatusCode)
}

func TestWithIgnoreRoutes(t *testing.T) {
	opts := &accessLogOptions{ignoreRoutes: make(map[string]struct{})}
	WithIgnoreRoutes("/api", "/health")(opts)
	assert.Len(t, opts.ignoreRoutes, 2)
	_, ok1 := opts.ignoreRoutes["/api"]
	_, ok2 := opts.ignoreRoutes["/health"]
	assert.True(t, ok1)
	assert.True(t, ok2)

	// adding duplicate should not increase length
	WithIgnoreRoutes("/api")(opts)
	assert.Len(t, opts.ignoreRoutes, 2)
}

func TestApplyOptions(t *testing.T) {
	o := defaultAccessLogOptions()
	originalLogger := o.log

	o.apply(WithEnable(false), WithSlowThreshold(time.Second), WithMinStatusCode(400), WithIgnoreRoutes("/test"))
	assert.False(t, o.enable)
	assert.Equal(t, time.Second, o.slowThreshold)
	assert.Equal(t, 400, o.minStatusCode)
	_, exists := o.ignoreRoutes["/test"]
	assert.True(t, exists)

	// logger should remain unchanged unless explicitly replaced
	assert.Same(t, originalLogger, o.log)
}

// setupTestRouter creates a Hertz engine with AccessLog middleware using provided options.
// It returns the router and the recorded log observer.
func setupTestRouter(t *testing.T, opts ...AccessLogOption) (*server.Hertz, *observer.ObservedLogs) {
	core, recorded := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	// Use a blank Hertz engine without default middlewares to avoid extra logging.
	h := server.New()

	allOpts := append([]AccessLogOption{WithLogger(logger)}, opts...)
	h.Use(AccessLog(allOpts...))

	return h, recorded
}

// executeRequest is a helper that performs a request on the given Hertz router.
func executeRequest(r *server.Hertz, method, path string, headers map[string]string, body ...string) *ut.ResponseRecorder {
	var b *ut.Body
	if len(body) > 0 {
		b = &ut.Body{Body: strings.NewReader(body[0]), Len: -1}
	}
	if headers != nil {
		utHeaders := []ut.Header{}
		for key, val := range headers {
			utHeaders = append(utHeaders, ut.Header{Key: key, Value: val})
		}
		return ut.PerformRequest(r.Engine, method, path, b, utHeaders...)
	}
	return ut.PerformRequest(r.Engine, method, path, b)
}

func TestAccessLog_Disabled(t *testing.T) {
	r, recorded := setupTestRouter(t, WithEnable(false))
	r.GET("/any", func(_ context.Context, ctx *app.RequestContext) {
		ctx.String(http.StatusOK, "ok")
	})

	respCtx := executeRequest(r, "GET", "/any", nil)
	assert.Equal(t, http.StatusOK, respCtx.Code)
	assert.Empty(t, recorded.All(), "no logs should be recorded when middleware is disabled")
}

func TestAccessLog_IgnoreRoutes(t *testing.T) {
	r, recorded := setupTestRouter(t, WithIgnoreRoutes("/health"))
	r.GET("/health", func(_ context.Context, ctx *app.RequestContext) {
		ctx.String(http.StatusOK, "healthy")
	})
	r.GET("/api", func(_ context.Context, ctx *app.RequestContext) {
		ctx.String(http.StatusOK, "data")
	})

	// request to ignored path
	executeRequest(r, "GET", "/health", nil)
	assert.Empty(t, recorded.All(), "ignored route should not produce logs")

	// request to normal path
	executeRequest(r, "GET", "/api", nil)
	entries := recorded.All()
	assert.NotEmpty(t, entries, "non-ignored route should produce logs")
	assert.Equal(t, "http access", entries[0].Message)
}

func TestAccessLog_DefaultIgnoreRoutes(t *testing.T) {
	r, recorded := setupTestRouter(t) // uses default options (which ignores /ping, /pong, /health, /metrics)
	for _, path := range []string{"/ping", "/pong", "/health", "/metrics"} {
		r.GET(path, func(_ context.Context, ctx *app.RequestContext) {
			ctx.String(http.StatusOK, "ok")
		})
	}

	for _, path := range []string{"/ping", "/pong", "/health", "/metrics"} {
		executeRequest(r, "GET", path, nil)
	}
	assert.Empty(t, recorded.All(), "default ignored routes should not produce logs")

	// a non-ignored path should still log
	r.GET("/data", func(_ context.Context, ctx *app.RequestContext) {
		ctx.String(http.StatusOK, "data")
	})
	executeRequest(r, "GET", "/data", nil)
	assert.NotEmpty(t, recorded.All())
}

func TestAccessLog_GeneratesTraceIDWhenMissing(t *testing.T) {
	r, recorded := setupTestRouter(t)
	r.GET("/test", func(_ context.Context, ctx *app.RequestContext) {
		// Inside handler the header should already be set on the request
		reqID := ctx.Request.Header.Get(HeaderTraceID)
		assert.NotEmpty(t, reqID, "trace ID should be generated before handler executes")
		// Also set a custom header to verify later
		ctx.Response.Header.Set("X-Custom", "value")
		ctx.String(http.StatusOK, "body")
	})

	respCtx := executeRequest(r, "GET", "/test", nil)
	assert.Equal(t, http.StatusOK, respCtx.Code)

	// response should contain the generated trace ID
	// respTrace := respCtx.Header.Get(HeaderTraceID)
	respTrace := respCtx.Header().Get(HeaderTraceID)
	assert.NotEmpty(t, respTrace, "response should contain X-Request-ID header")

	// log entry should contain the same trace_id
	entries := recorded.All()
	require.NotEmpty(t, entries)
	entry := entries[0]
	assert.Equal(t, respTrace, entry.ContextMap()["trace_id"])
}

func TestAccessLog_ExistingTraceID(t *testing.T) {
	r, recorded := setupTestRouter(t)
	r.GET("/test", func(c context.Context, ctx *app.RequestContext) {
		ctx.String(http.StatusOK, "ok")
	})

	existingID := "custom-trace-id-1234567890abcdef"
	respCtx := executeRequest(r, "GET", "/test", map[string]string{HeaderTraceID: existingID})
	assert.Equal(t, http.StatusOK, respCtx.Code)

	// response should echo the same trace ID
	assert.Equal(t, existingID, respCtx.Header().Get(HeaderTraceID))

	entries := recorded.All()
	require.NotEmpty(t, entries)
	assert.Equal(t, existingID, entries[0].ContextMap()["trace_id"])
}

func TestAccessLog_LogFields(t *testing.T) {
	r, recorded := setupTestRouter(t)
	r.GET("/data", func(_ context.Context, ctx *app.RequestContext) {
		time.Sleep(time.Millisecond * 5)
		ctx.String(http.StatusOK, "payload")
	})

	executeRequest(r, "GET", "/data", nil)
	entries := recorded.All()
	require.NotEmpty(t, entries)
	m := entries[0].ContextMap()
	assert.Equal(t, "GET", m["method"])
	assert.Equal(t, "/data", m["path"])
	assert.Equal(t, int64(http.StatusOK), m["status"])
	assert.NotZero(t, m["latency_ms"])
	// size: hertz writes the response body, so size > 0
	assert.Greater(t, m["size"], int64(0))
}

func TestAccessLog_SizeFieldNonNegative(t *testing.T) {
	r, recorded := setupTestRouter(t)
	r.GET("/empty", func(_ context.Context, ctx *app.RequestContext) {
		// Do not write anything; status defaults to 200 but nobody
		ctx.Status(http.StatusNoContent)
	})

	executeRequest(r, "GET", "/empty", nil)
	entries := recorded.All()
	require.NotEmpty(t, entries)
	size := entries[0].ContextMap()["size"]
	assert.EqualValues(t, 0, size) // NoContent writes 0 bytes, Size() returns 0 -> max(0,0)=0
}

func TestAccessLog_LogLevelsAndMinStatusCode(t *testing.T) {
	// Use a very high slow threshold so that latency never triggers slow logs.
	r, recorded := setupTestRouter(t, WithSlowThreshold(time.Hour), WithMinStatusCode(200))
	r.GET("/ok", func(_ context.Context, ctx *app.RequestContext) {
		ctx.String(http.StatusOK, "ok")
	})
	r.GET("/bad", func(_ context.Context, ctx *app.RequestContext) {
		ctx.String(http.StatusBadRequest, "bad request")
	})
	r.GET("/err", func(_ context.Context, ctx *app.RequestContext) {
		ctx.String(http.StatusInternalServerError, "error")
	})

	// OK -> Info
	executeRequest(r, "GET", "/ok", nil)
	entries := recorded.TakeAll()
	require.Len(t, entries, 1)
	assert.Equal(t, zapcore.InfoLevel, entries[0].Level)
	assert.Equal(t, "http access", entries[0].Message)

	// Bad Request -> Warn
	executeRequest(r, "GET", "/bad", nil)
	entries = recorded.TakeAll()
	require.Len(t, entries, 1)
	assert.Equal(t, zapcore.WarnLevel, entries[0].Level)
	assert.Equal(t, "http access", entries[0].Message)

	// Internal Server Error -> Error
	executeRequest(r, "GET", "/err", nil)
	entries = recorded.TakeAll()
	require.Len(t, entries, 1)
	assert.Equal(t, zapcore.ErrorLevel, entries[0].Level)
	assert.Equal(t, "http access", entries[0].Message)
}

func TestAccessLog_SlowRequestLogLevel(t *testing.T) {
	r, recorded := setupTestRouter(t, WithSlowThreshold(10*time.Millisecond))
	r.GET("/slow", func(_ context.Context, ctx *app.RequestContext) {
		time.Sleep(20 * time.Millisecond)
		ctx.String(http.StatusOK, "slow")
	})

	executeRequest(r, "GET", "/slow", nil)
	entries := recorded.All()
	require.NotEmpty(t, entries)
	// It should be a warn log because of latency
	assert.Equal(t, zapcore.WarnLevel, entries[0].Level)
	assert.Equal(t, "slow request", entries[0].Message)
}

func TestAccessLog_MinStatusCodeFilter(t *testing.T) {
	// Set minStatusCode to 400, slowThreshold high -> a 200 OK with low latency should be filtered out
	r, recorded := setupTestRouter(t, WithMinStatusCode(400), WithSlowThreshold(time.Hour))
	r.GET("/low", func(_ context.Context, ctx *app.RequestContext) {
		ctx.String(http.StatusOK, "ok")
	})
	r.GET("/high", func(_ context.Context, ctx *app.RequestContext) {
		ctx.String(http.StatusBadRequest, "bad")
	})

	// low status -> not logged
	executeRequest(r, "GET", "/low", nil)
	assert.Empty(t, recorded.All(), "response below minStatusCode should not be logged")

	// high status -> logged
	executeRequest(r, "GET", "/high", nil)
	assert.NotEmpty(t, recorded.All())
}

func TestAccessLog_SlowOverridesMinStatusCode(t *testing.T) {
	r, recorded := setupTestRouter(t, WithMinStatusCode(400), WithSlowThreshold(10*time.Millisecond))
	r.GET("/slow-ok", func(_ context.Context, ctx *app.RequestContext) {
		time.Sleep(20 * time.Millisecond)
		ctx.String(http.StatusOK, "slow but ok")
	})

	executeRequest(r, "GET", "/slow-ok", nil)
	entries := recorded.All()
	require.NotEmpty(t, entries, "slow request should be logged even if status < minStatusCode")
	assert.Equal(t, zapcore.WarnLevel, entries[0].Level)
	assert.Equal(t, "slow request", entries[0].Message)
}

func TestAccessLog_StatusCodeThresholds(t *testing.T) {
	r, recorded := setupTestRouter(t, WithSlowThreshold(time.Hour)) // never slow
	r.GET("/399", func(_ context.Context, ctx *app.RequestContext) { ctx.String(399, "custom") })
	r.GET("/400", func(_ context.Context, ctx *app.RequestContext) { ctx.String(400, "bad") })
	r.GET("/499", func(_ context.Context, ctx *app.RequestContext) { ctx.String(499, "custom") })
	r.GET("/500", func(_ context.Context, ctx *app.RequestContext) { ctx.String(500, "error") })

	// 399 is < 400 -> should be Info (because no slow threshold)
	executeRequest(r, "GET", "/399", nil)
	entries := recorded.TakeAll()
	require.Len(t, entries, 1)
	assert.Equal(t, zapcore.InfoLevel, entries[0].Level)

	// 400 -> Warn
	executeRequest(r, "GET", "/400", nil)
	entries = recorded.TakeAll()
	require.Len(t, entries, 1)
	assert.Equal(t, zapcore.WarnLevel, entries[0].Level)

	// 499 -> Warn (since < 500)
	executeRequest(r, "GET", "/499", nil)
	entries = recorded.TakeAll()
	require.Len(t, entries, 1)
	assert.Equal(t, zapcore.WarnLevel, entries[0].Level)

	// 500 -> Error
	executeRequest(r, "GET", "/500", nil)
	entries = recorded.TakeAll()
	require.Len(t, entries, 1)
	assert.Equal(t, zapcore.ErrorLevel, entries[0].Level)
}

func TestAccessLog_CombinedSlowAndErrorStatus(t *testing.T) {
	r, recorded := setupTestRouter(t, WithSlowThreshold(10*time.Millisecond))
	r.GET("/slow-500", func(_ context.Context, ctx *app.RequestContext) {
		time.Sleep(20 * time.Millisecond)
		ctx.String(http.StatusInternalServerError, "slow error")
	})

	executeRequest(r, "GET", "/slow-500", nil)
	entries := recorded.All()
	require.NotEmpty(t, entries)
	// Because the switch evaluates status >= 500 first, it should be Error, not Warn for slow
	assert.Equal(t, zapcore.ErrorLevel, entries[0].Level)
	assert.Equal(t, "http access", entries[0].Message)
}

// TestAccessLog_CustomLogger verifies that WithZapLogger replaces the logger correctly.
func TestAccessLog_CustomLogger(t *testing.T) {
	core1, rec1 := observer.New(zapcore.DebugLevel)
	logger1 := zap.New(core1)

	r := server.New()
	r.Use(AccessLog(WithLogger(logger1)))
	r.GET("/test", func(_ context.Context, ctx *app.RequestContext) {
		ctx.String(http.StatusOK, "test")
	})

	executeRequest(r, "GET", "/test", nil)
	require.NotEmpty(t, rec1.All())
}
