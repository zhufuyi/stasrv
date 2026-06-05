package trace

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateTraceID_FormatAndLength(t *testing.T) {
	id := GenerateTraceID()
	assert.Len(t, id, 32, "trace ID should be 32 hex characters (16 bytes)")
	// verify it's valid hex
	_, err := hex.DecodeString(id)
	assert.NoError(t, err)
}

func TestGenerateTraceID_Uniqueness(t *testing.T) {
	ids := make(map[string]struct{})
	// generate many IDs in quick succession (some may share the same nanosecond)
	for i := 0; i < 1000; i++ {
		id := GenerateTraceID()
		_, dup := ids[id]
		assert.False(t, dup, "duplicate trace ID found: %s", id)
		ids[id] = struct{}{}
	}
}

func BenchmarkGenerateTraceID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateTraceID()
	}
}
