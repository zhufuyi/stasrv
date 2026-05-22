package logger

import (
	"bytes"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func setupTestLogger() (*bytes.Buffer, func()) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	originalLogger := globalLogger
	globalLogger = slog.New(handler)

	teardown := func() {
		globalLogger = originalLogger
	}

	return &buf, teardown
}

func TestGet(t *testing.T) {
	originalLogger := globalLogger
	defer func() { globalLogger = originalLogger }()

	globalLogger = nil
	logger1 := Get()
	if logger1 == nil {
		t.Fatal("Get() should not return nil")
	}

	logger2 := Get()
	if logger1 != logger2 {
		t.Fatal("Get() should return the same instance (singleton)")
	}
}

func TestStandardLogMethods(t *testing.T) {
	buf, teardown := setupTestLogger()
	defer teardown()

	tests := []struct {
		name          string
		logFunc       func(msg string, args ...any)
		msg           string
		args          []any
		expectedLevel string
	}{
		{"Debug", Debug, "debug message", []any{"key", "value"}, "level=DEBUG"},
		{"Info", Info, "info message", []any{"key", "value"}, "level=INFO"},
		{"Warn", Warn, "warn message", []any{"key", "value"}, "level=WARN"},
		{"Error", Error, "error message", []any{"key", "value"}, "level=ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc(tt.msg, tt.args...)

			output := buf.String()
			if !strings.Contains(output, tt.expectedLevel) {
				t.Errorf("expected %s, got output: %s", tt.expectedLevel, output)
			}
			if !strings.Contains(output, tt.msg) {
				t.Errorf("expected message %q, got output: %s", tt.msg, output)
			}
			if !strings.Contains(output, "key=value") {
				t.Errorf("expected args key=value, got output: %s", output)
			}
		})
	}
}

func TestFormatLogMethods(t *testing.T) {
	buf, teardown := setupTestLogger()
	defer teardown()

	tests := []struct {
		name          string
		logFunc       func(format string, args ...any)
		format        string
		args          []any
		expectedLevel string
		expectedMsg   string
	}{
		{"Debugf", Debugf, "debug %s %d", []any{"format", 1}, "level=DEBUG", "debug format 1"},
		{"Infof", Infof, "info %s %d", []any{"format", 2}, "level=INFO", "info format 2"},
		{"Warnf", Warnf, "warn %s %d", []any{"format", 3}, "level=WARN", "warn format 3"},
		{"Errorf", Errorf, "error %s %d", []any{"format", 4}, "level=ERROR", "error format 4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc(tt.format, tt.args...)

			output := buf.String()
			if !strings.Contains(output, tt.expectedLevel) {
				t.Errorf("expected %s, got output: %s", tt.expectedLevel, output)
			}
			if !strings.Contains(output, tt.expectedMsg) {
				t.Errorf("expected message %q, got output: %s", tt.expectedMsg, output)
			}
		})
	}
}

func TestFatal(t *testing.T) {
	if os.Getenv("TEST_FATAL_CRASH") == "1" {
		setupTestLogger()
		Fatal("this is a fatal error")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestFatal")
	cmd.Env = append(os.Environ(), "TEST_FATAL_CRASH=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}

func TestFatalf(t *testing.T) {
	if os.Getenv("TEST_FATALF_CRASH") == "1" {
		setupTestLogger()
		Fatalf("this is a fatal error: %d", 100)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestFatalf")
	cmd.Env = append(os.Environ(), "TEST_FATALF_CRASH=1")
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}
