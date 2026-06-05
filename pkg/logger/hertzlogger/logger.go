// Package hertzlogger provides a hertz logger implementation based on zap.
package hertzlogger

import (
	"context"
	"io"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"go.uber.org/zap"

	"github.com/zhufuyi/stasrv/pkg/logger"
	"github.com/zhufuyi/stasrv/pkg/trace/integration/hertztrace"
)

// NewHertzLogger create a hertz logger
func NewHertzLogger() hlog.FullLogger {
	return &hertzLogger{
		zapLogger: logger.Get().WithOptions(zap.AddCallerSkip(2)),
		sugared:   logger.Get().Sugar().WithOptions(zap.AddCallerSkip(2)),
	}
}

type hertzLogger struct {
	zapLogger *zap.Logger
	sugared   *zap.SugaredLogger
}

func (l *hertzLogger) Trace(v ...any) {
	l.sugared.Debug(v...)
}

func (l *hertzLogger) Debug(v ...any) {
	l.sugared.Debug(v...)
}

func (l *hertzLogger) Info(v ...any) {
	l.sugared.Info(v...)
}

func (l *hertzLogger) Notice(v ...any) {
	l.sugared.Info(v...)
}

func (l *hertzLogger) Warn(v ...any) {
	l.sugared.Warn(v...)
}

func (l *hertzLogger) Error(v ...any) {
	l.sugared.Error(v...)
}

func (l *hertzLogger) Fatal(v ...any) {
	l.sugared.Fatal(v...)
}

func (l *hertzLogger) Tracef(format string, v ...any) {
	l.sugared.Debugf(format, v...)
}

func (l *hertzLogger) Debugf(format string, v ...any) {
	l.sugared.Debugf(format, v...)
}

func (l *hertzLogger) Infof(format string, v ...any) {
	l.sugared.Infof(format, v...)
}

func (l *hertzLogger) Noticef(format string, v ...any) {
	l.sugared.Infof(format, v...)
}

func (l *hertzLogger) Warnf(format string, v ...any) {
	l.sugared.Warnf(format, v...)
}

func (l *hertzLogger) Errorf(format string, v ...any) {
	l.sugared.Errorf(format, v...)
}

func (l *hertzLogger) Fatalf(format string, v ...any) {
	l.sugared.Fatalf(format, v...)
}

func (l *hertzLogger) CtxTracef(ctx context.Context, format string, v ...any) {
	l.sugared.With(hertztrace.TraceIDKey.String(), hertztrace.GetTraceID(ctx)).Debugf(format, v...)
}

func (l *hertzLogger) CtxDebugf(ctx context.Context, format string, v ...any) {
	l.sugared.With(hertztrace.TraceIDKey.String(), hertztrace.GetTraceID(ctx)).Debugf(format, v...)
}

func (l *hertzLogger) CtxInfof(ctx context.Context, format string, v ...any) {
	l.sugared.With(hertztrace.TraceIDKey.String(), hertztrace.GetTraceID(ctx)).Infof(format, v...)
}

func (l *hertzLogger) CtxNoticef(ctx context.Context, format string, v ...any) {
	l.sugared.With(hertztrace.TraceIDKey.String(), hertztrace.GetTraceID(ctx)).Infof(format, v...)
}

func (l *hertzLogger) CtxWarnf(ctx context.Context, format string, v ...any) {
	l.sugared.With(hertztrace.TraceIDKey.String(), hertztrace.GetTraceID(ctx)).Warnf(format, v...)
}

func (l *hertzLogger) CtxErrorf(ctx context.Context, format string, v ...any) {
	l.sugared.With(hertztrace.TraceIDKey.String(), hertztrace.GetTraceID(ctx)).Errorf(format, v...)
}

func (l *hertzLogger) CtxFatalf(ctx context.Context, format string, v ...any) {
	l.sugared.With(hertztrace.TraceIDKey.String(), hertztrace.GetTraceID(ctx)).Fatalf(format, v...)
}

func (l *hertzLogger) SetLevel(level hlog.Level) {
	// Note: The log level of Zap is usually determined during initialization.
	// If dynamic adjustment is required, it is usually necessary to use zap. AtomicLevel.
	// Here is a simple logical mapping provided (as an example only, the actual effect depends on the logger configuration returned by Get())
	l.zapLogger.Warn("SetLevel called, but zap level should be managed by its AtomicLevel if needed.")
}

func (l *hertzLogger) SetOutput(writer io.Writer) {
	// Note: The output target of Zap is fixed when building the Core.
	// To make dynamic changes, it is necessary to rebuild the zap.Logger.
	l.zapLogger.Warn("SetOutput called, but zap output is immutable after creation.")
}
