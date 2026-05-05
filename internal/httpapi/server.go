package httpapi

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/x-dora/rw-node-go/internal/config"
)

type Server struct {
	httpServer         *http.Server
	internalServer     *http.Server
	internalSocketPath string
	useTLS             bool
}

func NewServer(cfg config.Config, handlers Handlers, logger *slog.Logger) (*Server, error) {
	handler := http.Handler(NewRouter(cfg, handlers, logger))
	handler = ZstdMiddlewareWithLimit(handler, cfg.RequestBodyLimitBytes)

	var tlsConfig *tls.Config
	useTLS := cfg.SecretKey != ""
	if useTLS {
		payload, err := config.DecodeSecretKey(cfg.SecretKey)
		if err != nil {
			return nil, err
		}

		tlsConfig, err = TLSConfigFromSecret(payload)
		if err != nil {
			return nil, err
		}

		publicKey, err := ParseJWTPublicKey(payload.JWTPublicKey)
		if err != nil {
			return nil, err
		}
		handler = JWTMiddlewareWithExemptPaths(publicKey, map[string]struct{}{
			"/internal/get-config": {},
		})(handler)
	}

	server := &Server{
		httpServer: &http.Server{
			Addr:              cfg.ListenAddress(),
			Handler:           handler,
			TLSConfig:         tlsConfig,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		useTLS: useTLS,
	}
	if cfg.InternalSocketPath != "" {
		server.internalSocketPath = cfg.InternalSocketPath
		server.internalServer = &http.Server{
			Handler:           InternalTokenMiddleware(NewRouter(cfg, handlers, logger), cfg.InternalRESTToken),
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		}
	}
	return server, nil
}

func (s *Server) ListenAndServe() error {
	if s.internalServer != nil && s.internalSocketPath != "" {
		listener, err := internalUnixListener(s.internalSocketPath)
		if err != nil {
			return err
		}
		go func() {
			_ = s.internalServer.Serve(listener)
		}()
	}
	if s.useTLS {
		if s.httpServer.TLSConfig == nil {
			return fmt.Errorf("TLS enabled without TLS config")
		}
		return s.httpServer.ListenAndServeTLS("", "")
	}
	return s.httpServer.ListenAndServe()
}

func (s *Server) ListenAndServeInternal(socketPath string) error {
	if s.internalServer == nil {
		return nil
	}
	if socketPath != "" {
		s.internalSocketPath = socketPath
	}
	listener, err := internalUnixListener(s.internalSocketPath)
	if err != nil {
		return err
	}
	defer os.Remove(s.internalSocketPath)
	return s.internalServer.Serve(listener)
}

func internalUnixListener(socketPath string) (net.Listener, error) {
	if socketPath == "" {
		return nil, fmt.Errorf("internal socket path is empty")
	}
	_ = os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("listen internal socket: %w", err)
	}
	return listener, nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.internalServer != nil {
		_ = s.internalServer.Shutdown(ctx)
		if s.internalSocketPath != "" {
			_ = os.Remove(s.internalSocketPath)
		}
	}
	return s.httpServer.Shutdown(ctx)
}
