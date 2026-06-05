package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestInit(t *testing.T) {
	// Test Init with all options
	Init(
		WithServiceName("test-service"),
		WithEnv("dev"),
		WithVersion("1.0.0"),
		WithHost("localhost"),
	)

	assert.NotNil(t, rootLogger)
}

func TestGet(t *testing.T) {
	// Reset rootLogger to test lazy initialization
	rootLogger = nil
	logger := Get()
	assert.NotNil(t, logger)
	assert.NotNil(t, rootLogger)
}

func TestOptions(t *testing.T) {
	opts := defaultOptions()
	assert.Equal(t, "", opts.serviceName)

	handlers := []Option{
		WithServiceName("svc"),
		WithEnv("prod"),
		WithVersion("v1"),
		WithHost("127.0.0.1"),
	}

	for _, h := range handlers {
		h(opts)
	}

	assert.Equal(t, "svc", opts.serviceName)
	assert.Equal(t, "prod", opts.env)
	assert.Equal(t, "v1", opts.version)
	assert.Equal(t, "127.0.0.1", opts.host)
}

func TestLoggingLevels(t *testing.T) {
	Init()

	// Testing standard logging methods
	Debug("debug message", zap.String("key", "value"))
	Info("info message")
	Warn("warn message")
	Error("error message")

	// Test Panic (using assert.Panics because zap.Panic will panic)
	assert.Panics(t, func() {
		Panic("panic message")
	})
}

func TestFormattingMethods(t *testing.T) {
	Init()

	// Testing Sugared logging methods
	Debugf("debug %s", "format")
	Infof("info %s", "format")
	Warnf("warn %s", "format")
	Errorf("error %s", "format")

	// Note: Fatalf is hard to test in unit tests as it calls os.Exit(1)
}

func TestWithFields(t *testing.T) {
	Init()
	logger := WithFields(zap.String("extra", "field"))
	assert.NotNil(t, logger)
}

func TestSync(t *testing.T) {
	Init()
	err := Sync()
	// Sync often returns error on some OS/Environments when syncing stdout,
	// but the function handles the /dev/stdout case.
	// We just ensure it doesn't crash.
	_ = err
}

func TestGetLoggerInternal(t *testing.T) {
	l := getLogger()
	assert.NotNil(t, l)

	s := getSugaredLogger()
	assert.NotNil(t, s)
}

func TestApplyOptions(t *testing.T) {
	o := &options{}
	o.apply(WithServiceName("test"))
	assert.Equal(t, "test", o.serviceName)
}
