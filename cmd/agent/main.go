package main

import (
	"context"
	"os"
	"os/signal"
	"smart-pc-waker-agent/internal/config"
	httpServer "smart-pc-waker-agent/internal/http-server"
	"smart-pc-waker-agent/internal/lib/logger"
	"smart-pc-waker-agent/internal/mqtt"
	pcsChecker "smart-pc-waker-agent/internal/pcs-checker"
	pcsService "smart-pc-waker-agent/internal/services/pcs-service"
	configStorage "smart-pc-waker-agent/internal/storage/config-storage"
	"syscall"

	authorization "smart-pc-waker-agent/internal/auth"

	"github.com/MaxRomanov007/smart-pc-go-lib/logger/sl"
	"github.com/MaxRomanov007/smart-pc-go-lib/waitable"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.MustLoad(ctx)

	logCtx, cancelLogCtx := context.WithCancel(context.Background())
	defer cancelLogCtx()
	log := logger.MustSetupLogger(logCtx, cfg.Env, string(*cfg.LogPath))

	log.Debug("debug messages are enabled")

	storage := configStorage.New(cfg)

	auth, err := authorization.New(ctx, cfg.Auth, storage, storage)
	if err != nil {
		log.Error("failed to create auth", sl.Err(err))
		os.Exit(1)
	}

	mqttConn, err := mqtt.New(
		ctx,
		log,
		cfg.MQTT,
		auth,
		storage,
	)
	if err != nil {
		log.Error("failed to create mqtt connection", sl.Err(err))
		os.Exit(1)
	}
	go func() {
		<-mqttConn.Done()
		log.Info("mqtt connection closed")
	}()

	pcs, err := pcsService.New(ctx, auth, cfg.Services.Pcs)
	if err != nil {
		log.Error("failed to create pcs service", sl.Err(err))
		os.Exit(1)
	}

	srv := httpServer.New(log, cfg.HTTPServer, storage, pcs)
	go func() {
		if err := srv.Run(ctx); err != nil {
			log.Error("http server error", sl.Err(err))
			os.Exit(1)
		}
	}()

	checker := pcsChecker.New(ctx, log, cfg.Checker.Interval, pcs, storage, storage)

	waitable.WaitAll(mqttConn, srv, checker)
}
