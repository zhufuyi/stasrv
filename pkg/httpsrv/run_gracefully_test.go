package httpsrv

import (
	"context"
	"net"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func TestDefaultOptions(t *testing.T) {
	o := defaultOptions()
	if o.shutdownTimeout != 5*time.Second {
		t.Errorf("default shutdown timeout = %v, want 5s", o.shutdownTimeout)
	}
}

func TestOptionsApply(t *testing.T) {
	o := defaultOptions()
	opt := WithForceShutdownTimeout(10 * time.Second)
	o.apply(opt)
	if o.shutdownTimeout != 10*time.Second {
		t.Errorf("after apply, shutdown timeout = %v, want 10s", o.shutdownTimeout)
	}

	o2 := defaultOptions()
	opt2 := WithForceShutdownTimeout(50 * time.Millisecond)
	o2.apply(opt2)
	if o2.shutdownTimeout != 5*time.Second {
		t.Errorf("after applying too small timeout, shutdown timeout = %v, want unchanged 5s", o2.shutdownTimeout)
	}
}

func TestRunGracefully_FailedToStart(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	addr := l.Addr().String()
	h := server.New(server.WithHostPorts(addr))

	err = RunGracefully(h)
	if err == nil {
		t.Fatal("expected error for failed start, got nil")
	}
	if !strings.Contains(err.Error(), "failed to start server") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunGracefully_ShutdownByExternal(t *testing.T) {
	h := server.New(server.WithHostPorts("127.0.0.1:0"))

	errCh := make(chan error, 1)
	go func() {
		errCh <- RunGracefully(h)
	}()

	time.Sleep(300 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.Shutdown(ctx); err != nil {
		t.Fatalf("external shutdown failed: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			if !strings.Contains(err.Error(), "server forced to shutdown") {
				t.Errorf("unexpected error after external shutdown: %v", err)
			}
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for RunGracefully after external shutdown")
	}
}

func TestRunGracefully_SignalShutdown(t *testing.T) {
	h := server.New(server.WithHostPorts("127.0.0.1:0"))

	errCh := make(chan error, 1)
	go func() {
		errCh <- RunGracefully(h)
	}()

	time.Sleep(500 * time.Millisecond)

	if runtime.GOOS == "windows" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		h.Shutdown(ctx)
	} else {
		p, _ := os.FindProcess(os.Getpid())
		p.Signal(syscall.SIGTERM)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Logf("server exited: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for graceful shutdown")
	}
}
