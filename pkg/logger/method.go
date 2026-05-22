package logger

import (
	"fmt"
	"os"
)

// Debug level information
func Debug(msg string, args ...any) {
	Get().Debug(msg, args...)
}

// Info level information
func Info(msg string, args ...any) {
	Get().Info(msg, args...)
}

// Warn level information
func Warn(msg string, args ...any) {
	Get().Warn(msg, args...)
}

// Error level information
func Error(msg string, args ...any) {
	Get().Error(msg, args...)
}

// Fatal level information
func Fatal(msg string, args ...any) {
	Get().Error(msg, args...)
	os.Exit(1) // nolint
}

// Debugf format level information
func Debugf(format string, a ...any) {
	Get().Debug(fmt.Sprintf(format, a...))
}

// Infof format level information
func Infof(format string, a ...any) {
	Get().Info(fmt.Sprintf(format, a...))
}

// Warnf format level information
func Warnf(format string, a ...any) {
	Get().Warn(fmt.Sprintf(format, a...))
}

// Errorf format level information
func Errorf(format string, a ...any) {
	Get().Error(fmt.Sprintf(format, a...))
}

// Fatalf format level information
func Fatalf(format string, a ...any) {
	Get().Error(fmt.Sprintf(format, a...))
	os.Exit(1) //nolint
}
