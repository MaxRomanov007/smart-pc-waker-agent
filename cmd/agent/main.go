package main

import (
	"context"
	"os/signal"
	"smart-pc-waker-agent/internal/config"
	"smart-pc-waker-agent/internal/lib/logger"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.MustLoad(ctx)

	logCtx, cancelLogCtx := context.WithCancel(context.Background())
	defer cancelLogCtx()
	log := logger.MustSetupLogger(logCtx, cfg.Env, string(*cfg.LogPath))

	log.Debug("debug messages are enabled")
}
