package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/x-dora/rw-node-go/internal/config"
)

type Server struct {
	httpServer *http.Server
}

func NewServer(cfg config.Config, handlers Handlers, logger *slog.Logger) *Server {
	router := NewRouter(cfg, handlers, logger)
	return &Server{
		httpServer: &http.Server{
			Addr:              cfg.ListenAddress(),
			Handler:           router,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
