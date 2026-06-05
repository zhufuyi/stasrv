package hertztrace

import (
	"context"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/stretchr/testify/assert"
)

func TestContextKey_String(t *testing.T) {
	key := contextKey("test")
	assert.Equal(t, "test", key.String())
	assert.Equal(t, "trace_id", TraceIDKey.String())
}

func TestSetTraceIDToContext(t *testing.T) {
	t.Run("TraceID_Exists_In_Header", func(t *testing.T) {
		c := app.NewContext(0)
		existingID := "existing-trace-id-123"
		c.Request.Header.Set(HeaderTraceID, existingID)

		ctx, id := SetTraceID(context.Background(), c)

		assert.Equal(t, existingID, id)
		assert.Equal(t, existingID, string(c.Response.Header.Peek(HeaderTraceID)))

		val, _ := c.Get(TraceIDKey.String())
		assert.Equal(t, existingID, val)

		assert.Equal(t, existingID, ctx.Value(TraceIDKey))
	})

	t.Run("TraceID_Missing_In_Header", func(t *testing.T) {
		c := app.NewContext(0)

		ctx, id := SetTraceID(context.Background(), c)

		assert.NotEmpty(t, id)
		assert.Equal(t, id, string(c.Request.Header.Peek(HeaderTraceID)))
		assert.Equal(t, id, string(c.Response.Header.Peek(HeaderTraceID)))
		assert.Equal(t, id, GetTraceID(ctx))
	})
}

func TestGetTraceID(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		expectedID := "test-id"
		ctx := context.WithValue(context.Background(), TraceIDKey, expectedID)
		assert.Equal(t, expectedID, GetTraceID(ctx))
	})

	t.Run("NotFound", func(t *testing.T) {
		ctx := context.Background()
		assert.Equal(t, "", GetTraceID(ctx))
	})

	t.Run("WrongType", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), TraceIDKey, 123)
		assert.Equal(t, "", GetTraceID(ctx))
	})
}

func TestGetTraceIDFromHertzContext(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		c := app.NewContext(0)
		expectedID := "hertz-trace-id"
		c.Set(TraceIDKey.String(), expectedID)

		assert.Equal(t, expectedID, GetTraceIDFromHertzContext(c))
	})

	t.Run("NotFound", func(t *testing.T) {
		c := app.NewContext(0)
		assert.Equal(t, "", GetTraceIDFromHertzContext(c))
	})

	t.Run("WrongType", func(t *testing.T) {
		c := app.NewContext(0)
		c.Set(TraceIDKey.String(), 456)
		assert.Equal(t, "", GetTraceIDFromHertzContext(c))
	})
}
