package pcsChecker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	localModels "smart-pc-waker-agent/internal/domain/models"

	"github.com/MaxRomanov007/smart-pc-go-lib/domain/models"
	"github.com/MaxRomanov007/smart-pc-go-lib/logger/sl"
)

type Checker struct {
	done              chan struct{}
	serverGetter      ServerGetter
	registeredGetter  RegisteredGetter
	registeredDeleter RegisteredDeleter
}

type ServerGetter interface {
	GetPcs(ctx context.Context) ([]models.Pc, error)
}

type RegisteredGetter interface {
	GetPcs(ctx context.Context) ([]localModels.Registered, error)
}

type RegisteredDeleter interface {
	DeletePcByID(ctx context.Context, pcID string) error
}

func New(
	ctx context.Context,
	logger *slog.Logger,
	interval time.Duration,
	serverGetter ServerGetter,
	registeredGetter RegisteredGetter,
	registeredDeleter RegisteredDeleter,
) *Checker {
	const component = "pcs-checker"
	log := logger.With(sl.Component(component))

	checker := &Checker{
		done:              make(chan struct{}),
		serverGetter:      serverGetter,
		registeredGetter:  registeredGetter,
		registeredDeleter: registeredDeleter,
	}

	log.Info("starting pcs checker")

	go func() {
		defer close(checker.done)
		defer log.Info("pcs checker stopped")

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				log.Info("checking pcs")

				if err := checker.SyncPcs(ctx); err != nil {
					log.Error("failed to sync pcs", sl.Err(err))
				}
			}
		}
	}()

	return checker
}

func (c *Checker) SyncPcs(ctx context.Context) error {
	const op = "pcs-checker.syncPcs"

	registeredPcs, err := c.registeredGetter.GetPcs(ctx)
	if err != nil {
		return fmt.Errorf("%s: failed to get registered pcs: %w", op, err)
	}

	serverPcs, err := c.serverGetter.GetPcs(ctx)
	if err != nil {
		return fmt.Errorf("%s: failed to get pcs from server: %w", op, err)
	}

	errs := make([]error, 0, len(registeredPcs))
	for _, registered := range registeredPcs {
		found := false
		for _, serverPc := range serverPcs {
			if registered.ID == serverPc.ID.String() {
				found = true
			}
		}

		if !found {
			if err := c.registeredDeleter.DeletePcByID(ctx, registered.ID); err != nil {
				errs = append(
					errs,
					fmt.Errorf("failed to delete registered pc (id: %s): %w", registered.ID, err),
				)
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s: failed to sync pcs: %w", op, errors.Join(errs...))
	}

	return nil
}

func (c *Checker) Done() <-chan struct{} {
	return c.done
}
