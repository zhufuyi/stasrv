// Package trace provides utilities for generating and managing trace IDs.
package trace

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"sync/atomic"
	"time"
)

var (
	traceRandSeed uint64
	traceCounter  uint64
)

func init() {
	var b [8]byte
	_, _ = rand.Read(b[:])
	traceRandSeed = binary.BigEndian.Uint64(b[:])
}

func GenerateTraceID() string {
	var b [16]byte
	now := uint64(time.Now().UnixNano())
	binary.BigEndian.PutUint64(b[0:8], now)

	count := atomic.AddUint64(&traceCounter, 1)
	binary.BigEndian.PutUint64(b[8:16], count^traceRandSeed)

	return hex.EncodeToString(b[:])
}
