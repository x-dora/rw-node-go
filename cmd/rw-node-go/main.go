package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/x-dora/rw-node-go/internal/config"
	"github.com/x-dora/rw-node-go/internal/controller"
	"github.com/x-dora/rw-node-go/internal/httpapi"
	"github.com/x-dora/rw-node-go/internal/state"
	"github.com/x-dora/rw-node-go/internal/version"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	runtimeState := state.NewRuntimeState()
	controllers := controller.NewRegistry(cfg, runtimeState, logger)
	defer func() {
		if controllers.Snapshot != nil {
			if err := controllers.Snapshot.Close(); err != nil {
				logger.Warn("close system snapshotter", "error", err)
			}
		}
	}()
	server, err := httpapi.NewServer(cfg, httpapi.Handlers{
		Xray:     controllers.Xray,
		Handler:  controllers.Handler,
		Stats:    controllers.Stats,
		Vision:   controllers.Vision,
		Plugin:   controllers.Plugin,
		Internal: controllers.Internal,
	}, logger)
	if err != nil {
		logger.Error("create server", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting rw-node-go", "addr", cfg.ListenAddress(), "project_version", version.ProjectVersion, "node_version", runtimeState.NodeVersion)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown server", "error", err)
			os.Exit(1)
		}
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}
}
