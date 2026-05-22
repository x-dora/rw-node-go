package httpapi

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/x-dora/rw-node-go/internal/config"
	"github.com/x-dora/rw-node-go/internal/logview"
)

type Server struct {
	httpServer     *http.Server
	internalServer *http.Server
	useTLS         bool
	logger         *slog.Logger
}

type listenerEntry struct {
	name     string
	server   *http.Server
	listener net.Listener
	scheme   string
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

		tlsConfig, err = TLSConfigFromSecretWithClientAuth(payload, cfg.TLSClientAuthMode())
		if err != nil {
			return nil, err
		}

		publicKey, err := ParseJWTPublicKey(payload.JWTPublicKey)
		if err != nil {
			return nil, err
		}
		handler = JWTMiddleware(publicKey)(handler)
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
		internalServer: &http.Server{
			Addr:              cfg.InternalListenAddress(),
			Handler:           NewInternalRouter(cfg, handlers, logger),
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		useTLS: useTLS,
		logger: logger,
	}
	return server, nil
}

func (s *Server) ListenAndServe() error {
	entries := make([]listenerEntry, 0, 2)
	if s.internalServer != nil {
		listener, err := net.Listen("tcp", s.internalServer.Addr)
		if err != nil {
			return fmt.Errorf("internal server: %w", err)
		}
		entries = append(entries, listenerEntry{
			name:     "internal server",
			server:   s.internalServer,
			listener: listener,
			scheme:   "http",
		})
	}

	if s.useTLS && s.httpServer.TLSConfig == nil {
		closeListeners(entries)
		return fmt.Errorf("TLS enabled without TLS config")
	}
	mainListener, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		closeListeners(entries)
		return fmt.Errorf("main server: %w", err)
	}
	mainScheme := "http"
	if s.useTLS {
		mainScheme = "https"
	}
	entries = append(entries, listenerEntry{
		name:     "main server",
		server:   s.httpServer,
		listener: mainListener,
		scheme:   mainScheme,
	})

	errCh := make(chan error, len(entries))
	for _, entry := range entries {
		entry := entry
		title := "Main API listening"
		message := "main API listening"
		if entry.server == s.internalServer {
			title = "Internal API listening"
			message = "internal API listening"
		}
		logview.InfoTable(s.logger, message, logview.ListenSummary(title, entry.listener.Addr().String(), entry.scheme))
		go func() {
			var serveErr error
			if entry.server == s.httpServer && s.useTLS {
				serveErr = entry.server.ServeTLS(entry.listener, "", "")
			} else {
				serveErr = entry.server.Serve(entry.listener)
			}
			if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				errCh <- fmt.Errorf("%s: %w", entry.name, serveErr)
				return
			}
			errCh <- nil
		}()
	}

	var firstErr error
	for range entries {
		err := <-errCh
		if err != nil && firstErr == nil {
			firstErr = err
			_ = s.close()
		}
	}
	return firstErr
}

func closeListeners(entries []listenerEntry) {
	for _, entry := range entries {
		_ = entry.listener.Close()
	}
}

func (s *Server) close() error {
	var errs []error
	if s.internalServer != nil {
		if err := s.internalServer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("internal server: %w", err))
		}
	}
	if err := s.httpServer.Close(); err != nil {
		errs = append(errs, fmt.Errorf("main server: %w", err))
	}
	return errors.Join(errs...)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.shutdown(ctx, func(server *http.Server, ctx context.Context) error {
		return server.Shutdown(ctx)
	})
}

func (s *Server) shutdown(ctx context.Context, shutdown func(*http.Server, context.Context) error) error {
	var errs []error
	if s.internalServer != nil {
		if err := shutdown(s.internalServer, ctx); err != nil {
			errs = append(errs, fmt.Errorf("internal server: %w", err))
		}
	}
	if err := shutdown(s.httpServer, ctx); err != nil {
		errs = append(errs, fmt.Errorf("main server: %w", err))
	}
	return errors.Join(errs...)
}
