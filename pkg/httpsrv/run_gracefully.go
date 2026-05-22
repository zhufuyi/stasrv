// Package httpsrv provides utilities for running HTTP servers with graceful shutdown capabilities.
package httpsrv

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Option set options.
type Option func(*options)

type options struct {
	isTLS             bool
	certFile, keyFile string
	shutdownTimeout   time.Duration // 5  seconds
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

// WithTLS enables TLS for the server.
func WithTLS(certFile, keyFile string) Option {
	return func(o *options) {
		o.isTLS = true
		o.certFile = certFile
		o.keyFile = keyFile
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

// RunGracefully starts the server in a graceful manner, with support for signal handling.
func RunGracefully(server *http.Server, opts ...Option) error {
	o := defaultOptions()
	o.apply(opts...)

	errCh := make(chan error, 1)

	go func() {
		var err error
		if o.isTLS {
			err = server.ListenAndServeTLS(o.certFile, o.keyFile)
		} else {
			err = server.ListenAndServe()
		}

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
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
	case sig := <-quit:
		log.Printf("Received signal: %v, shutting down server...\n", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), o.shutdownTimeout) // default is 5 seconds
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	log.Println("Server exited gracefully")
	return nil
}

// ListenAndServeGracefully starts a server with graceful shutdown capabilities.
func ListenAndServeGracefully(addr string, handler http.Handler, opts ...Option) error {
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}
	return RunGracefully(server, opts...)
}
