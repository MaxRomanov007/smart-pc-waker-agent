package httpServer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"smart-pc-waker-agent/internal/auth"
	"smart-pc-waker-agent/internal/config"
	authCallback "smart-pc-waker-agent/internal/http-server/handlers/auth/callback"
	getStatus "smart-pc-waker-agent/internal/http-server/handlers/auth/get-status"
	getURL "smart-pc-waker-agent/internal/http-server/handlers/auth/get-url"
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
	router     *chi.Mux
	done       chan struct{}
}

func New(
	log *slog.Logger,
	cfg *config.Config,
	a *auth.Auth,
) *Server {
	r := chi.NewRouter()
	r.Use(
		middleware.RequestID,
		middleware.Recoverer,
		logmw.New(log),
	)

	r.Route("/auth", func(r chi.Router) {
		r.Get("/status", getStatus.New(log, a))
		r.Get("/url", getURL.New(log, a, cfg.Auth.CallbackURL))
		r.Get("/callback", authCallback.New(log, a))
	})

	srv := &http.Server{
		Addr:         cfg.HTTPServer.Address,
		Handler:      r,
		ReadTimeout:  cfg.HTTPServer.Timeout,
		WriteTimeout: cfg.HTTPServer.Timeout,
		IdleTimeout:  cfg.HTTPServer.IdleTimeout,
	}

	return &Server{
		HTTPServer: srv,
		router:     r,
		log:        log,
		cfg:        cfg.HTTPServer,
		done:       make(chan struct{}),
	}
}

func (s *Server) Mount(
	storage *configStorage.Storage,
	service *pcsService.Service,
) {
	v := validator.New()
	v.RegisterTagNameFunc(jsonTagName.New())

	s.router.Route("/registered", func(r chi.Router) {
		r.Get("/", getRegistered.New(s.log, storage))
		r.With(reqmw.New[createRegistered.Request](s.log, v)).
			Post("/", createRegistered.New(s.log, service, storage))
	})
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
			errorChan <- err
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
			return fmt.Errorf("%s: error stopping http server: %w", op, err)
		}
		log.Info("http server stopped")
	}

	return nil
}

func (s *Server) start() error {
	const op = "http-server.start"

	if err := s.HTTPServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *Server) stop(ctx context.Context) error {
	const op = "http-server.stop"

	if err := s.HTTPServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
