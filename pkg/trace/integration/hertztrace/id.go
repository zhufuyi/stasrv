// Package hertztrace provides trace id injection and retrieval for Hertz.
package hertztrace

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/zhufuyi/stasrv/pkg/trace"
)

const (
	HeaderTraceID            = "X-Request-ID" //  Corresponding trace_id
	TraceIDKey    contextKey = "trace_id"
)

type contextKey string

func (k contextKey) String() string {
	return string(k)
}

// SetTraceID inject trace id and get updated context
func SetTraceID(ctx context.Context, c *app.RequestContext) (context.Context, string) {
	traceID := string(c.GetHeader(HeaderTraceID))
	if traceID == "" {
		traceID = trace.GenerateTraceID()
		c.Request.Header.Set(HeaderTraceID, traceID)
	}

	c.Response.Header.Set(HeaderTraceID, traceID)

	// Store in Hertz RequestContext
	c.Set(TraceIDKey.String(), traceID)

	// Wrap and return standard context.Context
	newCtx := context.WithValue(ctx, TraceIDKey, traceID)
	return newCtx, traceID
}

// GetTraceID get trace id from context
func GetTraceID(ctx context.Context) string {
	if v, ok := ctx.Value(TraceIDKey).(string); ok {
		return v
	}
	return ""
}

// GetTraceIDFromHertzContext get trace id from hertz context
func GetTraceIDFromHertzContext(c *app.RequestContext) string {
	if v, ok := c.Get(TraceIDKey.String()); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
