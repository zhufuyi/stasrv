package hertzlogger

import (
	"context"
	"io"
	"testing"

	"go.uber.org/zap/zapcore"

	"github.com/zhufuyi/stasrv/pkg/logger"
	"github.com/zhufuyi/stasrv/pkg/trace/integration/hertztrace"
)

func TestMain(m *testing.M) {
	logger.Init(logger.WithLogLevel(zapcore.DebugLevel))

	m.Run()
}

func TestNewHertzLogger(t *testing.T) {
	l := NewHertzLogger()
	if l == nil {
		t.Fatal("NewHertzLogger() returned nil")
	}
}

func TestHertzLogger_LogMethods(t *testing.T) {
	l := NewHertzLogger()
	hl, ok := l.(*hertzLogger)
	if !ok {
		t.Fatal("NewHertzLogger() did not return *hertzLogger")
	}

	tests := []struct {
		name string
		fn   func()
	}{
		{"Trace", func() { hl.Trace("trace message") }},
		{"Debug", func() { hl.Debug("debug message") }},
		{"Info", func() { hl.Info("info message") }},
		{"Notice", func() { hl.Notice("notice message") }},
		{"Warn", func() { hl.Warn("warn message") }},
		{"Error", func() { hl.Error("error message") }},
		// {"Fatal", func() { hl.Fatal("fatal message") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.fn()
		})
	}
}

func TestHertzLogger_LogfMethods(t *testing.T) {
	l := NewHertzLogger()
	hl, ok := l.(*hertzLogger)
	if !ok {
		t.Fatal("NewHertzLogger() did not return *hertzLogger")
	}

	tests := []struct {
		name string
		fn   func()
	}{
		{"Tracef", func() { hl.Tracef("tracef: %s", "arg") }},
		{"Debugf", func() { hl.Debugf("debugf: %s", "arg") }},
		{"Infof", func() { hl.Infof("infof: %s", "arg") }},
		{"Noticef", func() { hl.Noticef("noticef: %s", "arg") }},
		{"Warnf", func() { hl.Warnf("warnf: %s", "arg") }},
		{"Errorf", func() { hl.Errorf("errorf: %s", "arg") }},
		// {"Fatalf", func() { hl.Fatalf("fatalf: %s", "arg") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.fn()
		})
	}
}

func TestHertzLogger_CtxLogMethods(t *testing.T) {
	l := NewHertzLogger()
	hl, ok := l.(*hertzLogger)
	if !ok {
		t.Fatal("NewHertzLogger() did not return *hertzLogger")
	}

	ctx := context.WithValue(context.Background(), hertztrace.TraceIDKey, "trace-id-abc-123")

	tests := []struct {
		name string
		fn   func()
	}{
		{"CtxTracef", func() { hl.CtxTracef(ctx, "ctx tracef: %s", "arg") }},
		{"CtxDebugf", func() { hl.CtxDebugf(ctx, "ctx debugf: %s", "arg") }},
		{"CtxInfof", func() { hl.CtxInfof(ctx, "ctx infof: %s", "arg") }},
		{"CtxNoticef", func() { hl.CtxNoticef(ctx, "ctx noticef: %s", "arg") }},
		{"CtxWarnf", func() { hl.CtxWarnf(ctx, "ctx warnf: %s", "arg") }},
		{"CtxErrorf", func() { hl.CtxErrorf(ctx, "ctx errorf: %s", "arg") }},
		// {"CtxFatalf", func() { hl.CtxFatalf(ctx, "ctx fatalf: %s", "arg") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.fn()
		})
	}
}

func TestHertzLogger_SetLevel(t *testing.T) {
	l := NewHertzLogger()
	hl, ok := l.(*hertzLogger)
	if !ok {
		t.Fatal("NewHertzLogger() did not return *hertzLogger")
	}

	hl.SetLevel(0)
}

func TestHertzLogger_SetOutput(t *testing.T) {
	l := NewHertzLogger()
	hl, ok := l.(*hertzLogger)
	if !ok {
		t.Fatal("NewHertzLogger() did not return *hertzLogger")
	}
	hl.SetOutput(io.Discard)
}
