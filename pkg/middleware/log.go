// Package middleware provides Gin middlewares.
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/zhufuyi/stasrv/pkg/logger"
)

// LogOption set logOptions.
type LogOption func(*logOptions)

type logOptions struct {
	logger        *slog.Logger
	skipPaths     map[string]struct{}
	traceIDHeader string
}

func (o *logOptions) apply(opts ...LogOption) {
	for _, opt := range opts {
		opt(o)
	}
}

func defaultLogOptions() *logOptions {
	return &logOptions{
		skipPaths: map[string]struct{}{
			"/health":     {},
			"/healthz":    {},
			"/metrics":    {},
			"/swagger":    {},
			"/swagger-ui": {},
		},
	}
}

func WithSkipPaths(paths ...string) LogOption {
	return func(o *logOptions) {
		if o.skipPaths == nil {
			o.skipPaths = make(map[string]struct{})
		}
		for _, path := range paths {
			o.skipPaths[path] = struct{}{}
		}
	}
}

func WithTraceIDHeader(header string) LogOption {
	return func(o *logOptions) {
		o.traceIDHeader = header
	}
}

func WithSlogLogger(l *slog.Logger) LogOption {
	return func(o *logOptions) {
		o.logger = l
	}
}

// ----------------------------------------------------------------

const defaultTraceIDHeader = "X-Request-Id"

type SlogConfig struct {
	logger        *slog.Logger
	skipPaths     map[string]struct{}
	traceIDHeader string
}

// SlogLogger Gin logging middleware
func SlogLogger(opts ...LogOption) gin.HandlerFunc {
	o := defaultLogOptions()
	o.apply(opts...)

	cfg := &SlogConfig{
		logger:        o.logger,
		skipPaths:     o.skipPaths,
		traceIDHeader: o.traceIDHeader,
	}

	if cfg.logger == nil {
		cfg.logger = logger.Get()
	}

	if cfg.traceIDHeader == "" {
		cfg.traceIDHeader = defaultTraceIDHeader
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if _, ok := cfg.skipPaths[path]; ok {
			c.Next()
			return
		}

		traceID := c.GetHeader(cfg.traceIDHeader)
		if traceID == "" {
			traceID = GenerateTraceID()
		}

		c.Header(cfg.traceIDHeader, traceID)

		ctxWithTrace := context.WithValue(c.Request.Context(), logger.ContextTraceIDKey, traceID)
		c.Request = c.Request.WithContext(ctxWithTrace)

		query := c.Request.URL.RawQuery
		start := time.Now()

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}

		maxAttrs := 8
		if len(c.Errors) > 0 {
			maxAttrs++
		}
		attrs := make([]slog.Attr, 0, maxAttrs)

		if len(query) > 256 {
			query = query[:256]
		}

		attrs = append(attrs,
			slog.Int("status", status),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.String("query", query),
			slog.String("client_ip", c.ClientIP()),
			slog.Int("resp_size", c.Writer.Size()),
			slog.Float64("latency_ms", float64(latency.Microseconds())/1000),
		)

		if len(c.Errors) > 0 {
			if len(c.Errors) == 1 {
				attrs = append(attrs, slog.String("errors", c.Errors[0].Err.Error()))
			} else {
				var sb strings.Builder
				for i, e := range c.Errors {
					if i > 0 {
						sb.WriteString("; ")
					}
					sb.WriteString(e.Err.Error())
				}
				attrs = append(attrs, slog.String("errors", sb.String()))
			}
		}

		cfg.logger.LogAttrs(c.Request.Context(), level, "HTTP Request", attrs...)
	}
}

// -----------------------------------------------------------------

var (
	traceRandSeed uint64
	traceCounter  uint64
)

func init() {
	var b [8]byte
	_, _ = rand.Read(b[:])
	traceRandSeed = binary.BigEndian.Uint64(b[:])
}

// GenerateTraceID generate trace id
func GenerateTraceID() string {
	var b [16]byte

	// High 8 bytes: direct use of nanosecond timestamps, guaranteed trend increment (database index-friendly)
	now := uint64(time.Now().UnixNano())
	binary.BigEndian.PutUint64(b[0:8], now)

	// Low 8 bytes: atom counter XOR or random seed
	// Ensure monotonic increment within the same process, and no conflict across processes/restarts
	count := atomic.AddUint64(&traceCounter, 1)
	binary.BigEndian.PutUint64(b[8:16], count^traceRandSeed)

	return hex.EncodeToString(b[:])
}
