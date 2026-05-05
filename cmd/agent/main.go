package main

import (
	"context"
	"fmt"
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
	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.MustLoad(ctx)

	log := logger.MustSetupLogger(ctx, cfg.Env, string(cfg.LogPath))
	log.Debug("debug messages are enabled")

	storage := configStorage.New(cfg)

	// Auth инициализируется всегда — даже если токена ещё нет.
	// В этом случае агент запустится без авторизации; клиент должен
	// вызвать GET /auth/url → пройти flow → GET /auth/callback.
	auth, err := authorization.New(ctx, cfg.Auth, storage, storage)
	if err != nil {
		log.Error("failed to create auth", sl.Err(err))
		os.Exit(1)
	}

	srv := httpServer.New(log, cfg, auth)
	go func() {
		if err := srv.Run(ctx); err != nil {
			log.Error("http server error", sl.Err(err))
			os.Exit(1)
		}
	}()

	if !auth.IsAuthorized() {
		log.Warn("agent is not authorized; use GET /auth/url to complete authorization")
	}

	if err := auth.WaitReady(ctx); err != nil {
		log.Error("authorization failed or context cancelled", sl.Err(err))
		os.Exit(1)
	}

	pcs, err := pcsService.New(auth, cfg.Services.Pcs)
	if err != nil {
		log.Error("failed to create pcs service", sl.Err(err))
		os.Exit(1)
	}

	srv.Mount(storage, pcs)

	log.Info("set pcs can power on")
	if err := setCanPowerOn(ctx, storage, pcs, true); err != nil {
		log.Error("failed to set can_power_on", sl.Err(err))
	}

	go func() {
		<-signalCtx.Done()

		log.Info("set pcs can not power on")
		if err := setCanPowerOn(ctx, storage, pcs, false); err != nil {
			log.Error("failed to set can_power_on", sl.Err(err))
		}

		cancel()
	}()

	mqttConn, err := mqtt.New(ctx, log, cfg.MQTT, auth, storage)
	if err != nil {
		log.Error("failed to create mqtt connection", sl.Err(err))
		os.Exit(1)
	}
	go func() {
		<-mqttConn.Done()
		log.Info("mqtt connection closed")
	}()

	checker := pcsChecker.New(ctx, log, cfg.Checker.Interval, pcs, storage, storage)

	waitable.WaitAll(mqttConn, srv, checker)
}

func setCanPowerOn(
	ctx context.Context,
	storage *configStorage.Storage,
	service *pcsService.Service,
	canPowerOn bool,
) error {
	const op = "setCanPowerOn"

	registered, err := storage.GetPcs(ctx)
	if err != nil {
		return fmt.Errorf("%s: failed to get registered pcs: %w", op, err)
	}

	ids := make([]string, len(registered))
	for i, pc := range registered {
		ids[i] = pc.ID
	}

	if err := service.SetCanPowerOnForIds(ctx, ids, canPowerOn); err != nil {
		return fmt.Errorf("%s: failed to set can_power_on: %w", op, err)
	}

	return nil
}
