package httpapi

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/x-dora/rw-node-go/internal/config"
)

type Server struct {
	httpServer *http.Server
	useTLS     bool
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
		handler = JWTMiddleware(publicKey)(handler)
	}

	return &Server{
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
	}, nil
}

func (s *Server) ListenAndServe() error {
	if s.useTLS {
		if s.httpServer.TLSConfig == nil {
			return fmt.Errorf("TLS enabled without TLS config")
		}
		return s.httpServer.ListenAndServeTLS("", "")
	}
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
