package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/x-dora/rw-node-go/internal/config"
)

func TestServerReportsInternalListenFailure(t *testing.T) {
	var logBuffer bytes.Buffer
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen occupied port: %v", err)
	}
	defer occupied.Close()

	_, internalPortText, err := net.SplitHostPort(occupied.Addr().String())
	if err != nil {
		t.Fatalf("split occupied addr: %v", err)
	}
	internalPort, err := net.LookupPort("tcp", internalPortText)
	if err != nil {
		t.Fatalf("parse occupied port: %v", err)
	}

	nodePort := freeTCPPort(t)
	server, err := NewServer(config.Config{
		NodePort:              nodePort,
		InternalRESTPort:      internalPort,
		RequestBodyLimitBytes: 1 << 20,
	}, testHandlers(), slog.New(slog.NewTextHandler(&logBuffer, nil)))
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer shutdownServer(t, server)

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err == nil || !strings.Contains(err.Error(), "internal server") {
			t.Fatalf("ListenAndServe() error = %v, want internal server listen error", err)
		}
		if strings.Contains(logBuffer.String(), "internal API listening") {
			t.Fatalf("logged internal API listening before bind succeeded:\n%s", logBuffer.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("ListenAndServe() did not return internal server listen error")
	}
}

func TestServerReportsMainListenFailureAndClosesInternalListener(t *testing.T) {
	occupied, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("listen occupied port: %v", err)
	}
	defer occupied.Close()

	_, nodePortText, err := net.SplitHostPort(occupied.Addr().String())
	if err != nil {
		t.Fatalf("split occupied addr: %v", err)
	}
	nodePort, err := net.LookupPort("tcp", nodePortText)
	if err != nil {
		t.Fatalf("parse occupied port: %v", err)
	}

	internalPort := freeTCPPort(t)
	server, err := NewServer(config.Config{
		NodePort:              nodePort,
		InternalRESTPort:      internalPort,
		RequestBodyLimitBytes: 1 << 20,
	}, testHandlers(), discardLogger())
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer shutdownServer(t, server)

	err = server.ListenAndServe()
	if err == nil || !strings.Contains(err.Error(), "main server") {
		t.Fatalf("ListenAndServe() error = %v, want main server listen error", err)
	}

	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", portString(internalPort)))
	if err != nil {
		t.Fatalf("internal listener was not closed after main listen failure: %v", err)
	}
	_ = listener.Close()
}

func TestServerReportsInternalListenFailureWithoutMainListenerLeak(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen occupied port: %v", err)
	}
	defer occupied.Close()

	_, internalPortText, err := net.SplitHostPort(occupied.Addr().String())
	if err != nil {
		t.Fatalf("split occupied addr: %v", err)
	}
	internalPort, err := net.LookupPort("tcp", internalPortText)
	if err != nil {
		t.Fatalf("parse occupied port: %v", err)
	}

	nodePort := freeTCPPort(t)
	server, err := NewServer(config.Config{
		NodePort:              nodePort,
		InternalRESTPort:      internalPort,
		RequestBodyLimitBytes: 1 << 20,
	}, testHandlers(), discardLogger())
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer shutdownServer(t, server)

	err = server.ListenAndServe()
	if err == nil || !strings.Contains(err.Error(), "internal server") {
		t.Fatalf("ListenAndServe() error = %v, want internal server listen error", err)
	}

	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", portString(nodePort)))
	if err != nil {
		t.Fatalf("main listener leaked after internal listen failure: %v", err)
	}
	_ = listener.Close()
}

func TestServerShutdownReturnsNil(t *testing.T) {
	nodePort := freeTCPPort(t)
	internalPort := freeTCPPort(t)
	server, err := NewServer(config.Config{
		NodePort:              nodePort,
		InternalRESTPort:      internalPort,
		RequestBodyLimitBytes: 1 << 20,
	}, testHandlers(), discardLogger())
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	waitForTCP(t, net.JoinHostPort("127.0.0.1", portString(nodePort)))
	waitForTCP(t, net.JoinHostPort("127.0.0.1", portString(internalPort)))

	shutdownServer(t, server)
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("ListenAndServe() after Shutdown error = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("ListenAndServe() did not return after Shutdown")
	}
}

func TestServerShutdownReturnsJoinedErrors(t *testing.T) {
	mainErr := errors.New("main boom")
	internalErr := errors.New("internal boom")
	server := &Server{
		httpServer:     &http.Server{Addr: "main"},
		internalServer: &http.Server{Addr: "internal"},
	}

	err := server.shutdown(context.Background(), func(server *http.Server, ctx context.Context) error {
		if server == nil {
			return nil
		}
		switch server.Addr {
		case "main":
			return mainErr
		case "internal":
			return internalErr
		default:
			return nil
		}
	})
	if !errors.Is(err, mainErr) || !errors.Is(err, internalErr) || !strings.Contains(err.Error(), "main server") || !strings.Contains(err.Error(), "internal server") {
		t.Fatalf("Shutdown() error = %v, want joined main and internal context errors", err)
	}
}

func testHandlers() Handlers {
	return Handlers{
		Xray:     tlsTestHandlers{},
		Handler:  tlsTestHandlers{},
		Stats:    tlsTestHandlers{},
		Plugin:   tlsTestHandlers{},
		Internal: tlsTestHandlers{},
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen free port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func portString(port int) string {
	return strconv.Itoa(port)
}

func waitForTCP(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("tcp %s did not become reachable: %v", addr, lastErr)
}

func shutdownServer(t *testing.T, server *Server) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}
