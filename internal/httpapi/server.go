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
	errCh := make(chan error, 2)
	if s.internalServer != nil {
		go func() {
			listener, err := net.Listen("tcp", s.internalServer.Addr)
			if err != nil {
				errCh <- fmt.Errorf("internal server: %w", err)
				return
			}
			logview.InfoTable(s.logger, "internal API listening", logview.ListenSummary("Internal API listening", listener.Addr().String(), "http"))
			if err := s.internalServer.Serve(listener); err != nil {
				if errors.Is(err, http.ErrServerClosed) {
					errCh <- nil
					return
				}
				errCh <- fmt.Errorf("internal server: %w", err)
			}
			errCh <- nil
		}()
	}

	go func() {
		var err error
		listener, listenErr := net.Listen("tcp", s.httpServer.Addr)
		if listenErr != nil {
			errCh <- listenErr
			return
		}
		if s.useTLS {
			if s.httpServer.TLSConfig == nil {
				_ = listener.Close()
				errCh <- fmt.Errorf("TLS enabled without TLS config")
				return
			}
			logview.InfoTable(s.logger, "main API listening", logview.ListenSummary("Main API listening", listener.Addr().String(), "https"))
			err = s.httpServer.ServeTLS(listener, "", "")
		} else {
			logview.InfoTable(s.logger, "main API listening", logview.ListenSummary("Main API listening", listener.Addr().String(), "http"))
			err = s.httpServer.Serve(listener)
		}
		if err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				errCh <- nil
				return
			}
			errCh <- err
		}
		errCh <- nil
	}()

	return <-errCh
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.internalServer != nil {
		_ = s.internalServer.Shutdown(ctx)
	}
	return s.httpServer.Shutdown(ctx)
}
