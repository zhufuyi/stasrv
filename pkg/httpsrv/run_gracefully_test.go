package httpsrv

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestOptions(t *testing.T) {
	opts := []Option{
		WithTLS("cert.pem", "key.pem"),
		WithForceShutdownTimeout(10 * time.Second),
		WithForceShutdownTimeout(50 * time.Millisecond), // 小于 100ms，应该被忽略
	}

	o := defaultOptions()
	o.apply(opts...)

	if !o.isTLS {
		t.Errorf("expected isTLS to be true")
	}
	if o.certFile != "cert.pem" || o.keyFile != "key.pem" {
		t.Errorf("expected certFile=cert.pem, keyFile=key.pem, got %s, %s", o.certFile, o.keyFile)
	}
	if o.shutdownTimeout != 10*time.Second {
		t.Errorf("expected shutdownTimeout to be 10s, got %v", o.shutdownTimeout)
	}
}

func TestListenAndServeGracefully_Success(t *testing.T) {
	safeRunWithTimeout(3*time.Second, func(cancel context.CancelFunc) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		errCh := make(chan error, 1)
		go func() {
			errCh <- ListenAndServeGracefully("127.0.0.1:0", handler)
		}()

		time.Sleep(500 * time.Millisecond)

		p, err := os.FindProcess(os.Getpid())
		if err != nil {
			t.Fatalf("failed to find current process: %v", err)
		}
		p.Signal(syscall.SIGINT)

		err = <-errCh
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})
}

func TestListenAndServeGracefully_StartupError(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer l.Close()

	addr := l.Addr().String()

	err = ListenAndServeGracefully(addr, nil)
	if err == nil {
		t.Fatal("expected error due to port already in use, got nil")
	}
	if !strings.Contains(err.Error(), "failed to start server") {
		t.Errorf("expected 'failed to start server' error, got: %v", err)
	}
}

func TestListenAndServeGracefully_TLS_Error(t *testing.T) {
	err := ListenAndServeGracefully("127.0.0.1:0", nil, WithTLS("invalid.crt", "invalid.key"))
	if err == nil {
		t.Fatal("expected error due to invalid certs, got nil")
	}
	if !strings.Contains(err.Error(), "failed to start server") {
		t.Errorf("expected 'failed to start server' error, got: %v", err)
	}
}

func TestRunGracefully_ShutdownTimeout(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{
		Addr:    "127.0.0.1:18081", // 固定一个测试端口
		Handler: handler,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- RunGracefully(server, WithForceShutdownTimeout(150*time.Millisecond))
	}()

	time.Sleep(100 * time.Millisecond)

	go func() {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1:18081/", nil)
		http.DefaultClient.Do(req)
	}()

	time.Sleep(500 * time.Millisecond)

	if runtime.GOOS == "windows" {
		_ = server.Shutdown(context.Background())
	} else {
		p, _ := os.FindProcess(os.Getpid())
		p.Signal(syscall.SIGTERM)
	}

	err := <-errCh
	if err == nil {
		t.Fatal("expected shutdown timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "server forced to shutdown") {
		t.Errorf("expected 'server forced to shutdown' error, got: %v", err)
	}
}

func safeRunWithTimeout(d time.Duration, fn func(cancel context.CancelFunc)) {
	ctx, cancel := context.WithTimeout(context.Background(), d)

	go func() {
		defer func() {
			if e := recover(); e != nil {
				fmt.Println(e)
			}
		}()

		fn(cancel)
	}()

	for range ctx.Done() {
		return
	}
}
