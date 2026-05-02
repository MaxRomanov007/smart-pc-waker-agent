package httpServer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"smart-pc-waker-agent/internal/config"
	createRegistered "smart-pc-waker-agent/internal/http-server/handlers/registered/create-registered"
	getRegistered "smart-pc-waker-agent/internal/http-server/handlers/registered/get-registered"
	pcsService "smart-pc-waker-agent/internal/services/pcs-service"
	configStorage "smart-pc-waker-agent/internal/storage/config-storage"

	"github.com/MaxRomanov007/smart-pc-go-lib/logger/sl"
	"github.com/MaxRomanov007/smart-pc-go-lib/middlewares/logmw"
	"github.com/MaxRomanov007/smart-pc-go-lib/middlewares/reqmw"
	jsonTagName "github.com/MaxRomanov007/smart-pc-go-lib/validator/tag-names/json-tag-name"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-playground/validator/v10"
)

type Server struct {
	HTTPServer *http.Server
	log        *slog.Logger
	cfg        config.HTTPServer

	done chan struct{}
}

func New(
	log *slog.Logger,
	cfg config.HTTPServer,
	storage *configStorage.Storage,
	service *pcsService.Service,
) *Server {
	r := chi.NewRouter()
	r.Use(
		middleware.RequestID,
		middleware.Recoverer,
		logmw.New(log),
	)

	v := validator.New()
	v.RegisterTagNameFunc(jsonTagName.New())

	r.Route("/registered", func(r chi.Router) {
		r.Get("/", getRegistered.New(log, storage))
		r.With(reqmw.New[createRegistered.Request](log, v)).
			Post("/", createRegistered.New(log, service, storage))
	})

	srv := &http.Server{
		Addr:         cfg.Address,
		Handler:      r,
		ReadTimeout:  cfg.Timeout,
		WriteTimeout: cfg.Timeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	return &Server{
		HTTPServer: srv,
		log:        log,
		cfg:        cfg,
		done:       make(chan struct{}),
	}
}

func (s *Server) Done() <-chan struct{} {
	return s.done
}

func (s *Server) Run(ctx context.Context) error {
	const op = "http-server.Run"
	log := s.log.With(sl.Op(op))

	defer close(s.done)

	log.Info("starting http server", slog.String("address", s.HTTPServer.Addr))
	errorChan := make(chan error, 1)
	go func() {
		if err := s.start(); err != nil {
			log.Error("failed to start http server", sl.Err(err))
			errorChan <- err
			return
		}
	}()

	select {
	case err := <-errorChan:
		return fmt.Errorf("%s: error starting http server: %w", op, err)
	case <-ctx.Done():
		stopCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
		defer cancel()

		log.Info("shutting down http server")
		if err := s.stop(stopCtx); err != nil {
			log.Error("failed to stop http server", sl.Err(err))
			return fmt.Errorf("%s: error stopping http server: %w", op, err)
		}
		log.Info("http server stopped")
	}

	return nil
}

func (s *Server) start() error {
	const op = "http-server.Start"

	if err := s.HTTPServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("%s: failed to start server: %w", op, err)
	}

	return nil
}

func (s *Server) stop(ctx context.Context) error {
	const op = "http-server.Stop"

	if err := s.HTTPServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("%s: failed to stop server: %w", op, err)
	}

	return nil
}
