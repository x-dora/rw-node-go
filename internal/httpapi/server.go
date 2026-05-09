package httpapi

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/x-dora/rw-node-go/internal/config"
)

type Server struct {
	httpServer     *http.Server
	internalServer *http.Server
	useTLS         bool
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
		handler = JWTMiddlewareWithExemptPaths(publicKey, visionJWTExemptPaths())(handler)
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
	}
	return server, nil
}

func visionJWTExemptPaths() map[string]struct{} {
	return map[string]struct{}{
		"/vision/block-ip":   {},
		"/vision/unblock-ip": {},
	}
}

func (s *Server) ListenAndServe() error {
	errCh := make(chan error, 2)
	if s.internalServer != nil {
		go func() {
			if err := s.internalServer.ListenAndServe(); err != nil {
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
		if s.useTLS {
			if s.httpServer.TLSConfig == nil {
				errCh <- fmt.Errorf("TLS enabled without TLS config")
				return
			}
			err = s.httpServer.ListenAndServeTLS("", "")
		} else {
			err = s.httpServer.ListenAndServe()
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
