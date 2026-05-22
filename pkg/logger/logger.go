// Package logger is a high-performance log library based on slog optimization
package logger

import (
	"context"
	"log/slog"
)

// Option set options.
type Option func(*options)

type options struct {
	isJSONFormat bool
	level        slog.Level
}

func (o *options) apply(opts ...Option) {
	for _, opt := range opts {
		opt(o)
	}
}

func defaultOptions() *options {
	return &options{
		isJSONFormat: false,
		level:        slog.LevelInfo,
	}
}

func WithJSONFormat(enable bool) Option {
	return func(o *options) {
		o.isJSONFormat = enable
	}
}

func WithLevel(level slog.Level) Option {
	return func(o *options) {
		o.level = level
	}
}

// ------------------------------------------------------------------------------

var globalLogger *slog.Logger

const ContextTraceIDKey = "_trace_id_"

// Init initialize log
func Init(opts ...Option) {
	o := defaultOptions()
	o.apply(opts...)

	var (
		baseHandler slog.Handler
		optsHandler = &slog.HandlerOptions{
			Level:     o.level,
			AddSource: false,
		}
	)

	if o.isJSONFormat {
		baseHandler = slog.NewJSONHandler(globalAsyncWriter, optsHandler)
	} else {
		baseHandler = slog.NewTextHandler(globalAsyncWriter, optsHandler)
	}

	globalLogger = slog.New(&contextHandler{Handler: baseHandler})
}

func defaultLogger() *slog.Logger {
	optsHandler := &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: false,
	}
	baseHandler := slog.NewTextHandler(globalAsyncWriter, optsHandler)
	return slog.New(&contextHandler{Handler: baseHandler})
}

// Get global logger
func Get() *slog.Logger {
	if globalLogger != nil {
		return globalLogger
	}
	globalLogger = defaultLogger()
	return globalLogger
}

// wrap an existing Handler with the ability to dynamically read TraceID from Context
type contextHandler struct {
	slog.Handler
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if ctx != nil {
		// try to get trace_id
		traceID := GetTraceID(ctx)
		if traceID != "" {
			r.AddAttrs(slog.String("trace_id", traceID))
		}
	}
	return h.Handler.Handle(ctx, r)
}

// GetTraceID get trace_id from ContextTraceIDKey
func GetTraceID(ctx context.Context) string {
	if ctx != nil {
		if traceID, ok := ctx.Value(ContextTraceIDKey).(string); ok && traceID != "" {
			return traceID
		}
	}
	return ""
}
