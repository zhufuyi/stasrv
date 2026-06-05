// Package logger is a logger library based on zap
package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Option set options.
type Option func(*options)

type options struct {
	serviceName string
	env         string
	version     string
	host        string
	logLevel    zapcore.Level
}

func (o *options) apply(opts ...Option) {
	for _, opt := range opts {
		opt(o)
	}
}

func defaultOptions() *options {
	return &options{
		logLevel: zapcore.InfoLevel,
	}
}

func WithServiceName(serviceName string) Option {
	return func(o *options) {
		o.serviceName = serviceName
	}
}

func WithEnv(env string) Option {
	return func(o *options) {
		o.env = env
	}
}

func WithVersion(version string) Option {
	return func(o *options) {
		o.version = version
	}
}

func WithHost(host string) Option {
	return func(o *options) {
		o.host = host
	}
}

func WithLogLevel(level zapcore.Level) Option {
	return func(o *options) {
		o.logLevel = level
	}
}

var rootLogger *zap.Logger

func Init(opts ...Option) {
	cfg := zap.NewProductionConfig()

	cfg.Encoding = "json"

	cfg.EncoderConfig.TimeKey = "time"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	cfg.EncoderConfig.LevelKey = "level"
	cfg.EncoderConfig.CallerKey = "caller"
	cfg.EncoderConfig.MessageKey = "msg"

	rootLogger, _ = cfg.Build(
		zap.AddCaller(),
		// zap.AddCallerSkip(0),
	)

	o := defaultOptions()
	o.apply(opts...)

	cfg.Level.SetLevel(o.logLevel) // set log level

	if o.serviceName != "" {
		rootLogger = rootLogger.With(zap.String("service_name", o.serviceName))
	}
	if o.env != "" {
		rootLogger = rootLogger.With(zap.String("env", o.env))
	}
	if o.version != "" {
		rootLogger = rootLogger.With(zap.String("version", o.version))
	}
	if o.host != "" {
		rootLogger = rootLogger.With(zap.String("host", o.host))
	}
}

func Get() *zap.Logger {
	if rootLogger == nil {
		Init()
	}
	return rootLogger
}
