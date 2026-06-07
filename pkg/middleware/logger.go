// Package middleware provides Hertz middlewares.
package middleware

import (
	"context"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/zhufuyi/stasrv/pkg/trace/integration/hertztrace"
)

const (
	HeaderTraceID        = "X-Request-ID" //  Corresponding trace_id
	DefaultSlowThreshold = 500 * time.Millisecond
)

var defaultIgnoreRoutes = map[string]struct{}{
	"/ping":    {},
	"/pong":    {},
	"/health":  {},
	"/metrics": {},
}

func newDefaultLogger() *zap.Logger {
	cfg := zap.NewProductionConfig()
	cfg.Encoding = "json"
	cfg.EncoderConfig.TimeKey = "time"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.LevelKey = "level"
	cfg.EncoderConfig.CallerKey = "caller"
	cfg.EncoderConfig.MessageKey = "msg"

	logger, _ := cfg.Build(zap.AddCaller())
	return logger
}

type AccessLogOption func(*accessLogOptions)

type accessLogOptions struct {
	enable        bool
	log           *zap.Logger
	slowThreshold time.Duration
	minStatusCode int
	ignoreRoutes  map[string]struct{}
}

func defaultAccessLogOptions() *accessLogOptions {
	routes := make(map[string]struct{}, len(defaultIgnoreRoutes))
	for k, v := range defaultIgnoreRoutes {
		routes[k] = v
	}

	return &accessLogOptions{
		enable:        true,
		log:           newDefaultLogger(),
		slowThreshold: DefaultSlowThreshold,
		minStatusCode: 0,
		ignoreRoutes:  routes,
	}
}

func (o *accessLogOptions) apply(opts ...AccessLogOption) {
	for _, opt := range opts {
		opt(o)
	}
}

func WithEnable(enable bool) AccessLogOption {
	return func(o *accessLogOptions) {
		o.enable = enable
	}
}

func WithLogger(log *zap.Logger) AccessLogOption {
	return func(o *accessLogOptions) {
		if log != nil {
			o.log = log
		}
	}
}

func WithSlowThreshold(d time.Duration) AccessLogOption {
	return func(o *accessLogOptions) {
		if d > 0 {
			o.slowThreshold = d
		}
	}
}

func WithMinStatusCode(code int) AccessLogOption {
	return func(o *accessLogOptions) {
		if code >= 0 {
			o.minStatusCode = code
		}
	}
}

func WithIgnoreRoutes(routes ...string) AccessLogOption {
	return func(o *accessLogOptions) {
		for _, route := range routes {
			o.ignoreRoutes[route] = struct{}{}
		}
	}
}

// -------------------------------------------------------------------------------

// AccessLog hertz access log middleware.
func AccessLog(opts ...AccessLogOption) app.HandlerFunc {
	o := defaultAccessLogOptions()
	o.apply(opts...)

	if !o.enable {
		return func(ctx context.Context, c *app.RequestContext) {
			c.Next(ctx)
		}
	}

	accessLog := o.log.WithOptions(zap.WithCaller(false))

	return func(ctx context.Context, c *app.RequestContext) {
		path := string(c.Path())
		if _, ok := o.ignoreRoutes[path]; ok {
			c.Next(ctx)
			return
		}

		// Inject Trace ID and get updated context
		ctx, traceID := hertztrace.SetTraceID(ctx, c)

		start := time.Now()

		c.Next(ctx)

		status := c.Response.StatusCode()
		latency := time.Since(start)

		if status < o.minStatusCode && latency < o.slowThreshold {
			return
		}

		fields := []zap.Field{
			zap.String(hertztrace.TraceIDKey.String(), traceID),
			zap.String("method", string(c.Method())),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Int64("latency_ms", latency.Milliseconds()),
			zap.Int("size", len(c.Response.Body())),
		}

		switch {
		case status >= consts.StatusInternalServerError:
			accessLog.Error("http access", fields...)
		case status >= consts.StatusBadRequest:
			accessLog.Warn("http access", fields...)
		case latency >= o.slowThreshold:
			accessLog.Warn("slow request", fields...)
		default:
			accessLog.Info("http access", fields...)
		}
	}
}
