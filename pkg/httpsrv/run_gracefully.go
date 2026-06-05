// Package httpsrv provides utilities for running Hertz servers with graceful shutdown capabilities.
package httpsrv

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
)

// Option set options.
type Option func(*options)

type options struct {
	shutdownTimeout time.Duration // default 5 seconds
}

func (o *options) apply(opts ...Option) {
	for _, opt := range opts {
		opt(o)
	}
}

func defaultOptions() *options {
	return &options{
		shutdownTimeout: 5 * time.Second,
	}
}

// WithForceShutdownTimeout sets the timeout for forceful shutdown.
func WithForceShutdownTimeout(timeout time.Duration) Option {
	return func(o *options) {
		if timeout <= time.Millisecond*100 {
			return
		}
		o.shutdownTimeout = timeout
	}
}

// -------------------------------------------------------------

// RunGracefully starts the Hertz server in a graceful manner, with support for signal handling.
func RunGracefully(h *server.Hertz, opts ...Option) error {
	o := defaultOptions()
	o.apply(opts...)

	errCh := make(chan error, 1)

	go func() {
		if err := h.Run(); err != nil {
			errCh <- err
		} else {
			errCh <- nil
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("failed to start server: %w", err)
		}
	case _ = <-quit:
	}

	ctx, cancel := context.WithTimeout(context.Background(), o.shutdownTimeout)
	defer cancel()

	if err := h.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	return nil
}
