package deleteAll

import (
	"context"
	"errors"
	"fmt"
	"go/types"
	"log/slog"
	"net/http"
	"smart-pc-waker-agent/internal/domain/models"

	"github.com/MaxRomanov007/smart-pc-go-lib/api/response"
	"github.com/MaxRomanov007/smart-pc-go-lib/logger/sl"
	"github.com/go-chi/render"
)

type RegisteredGetter interface {
	GetPcs(ctx context.Context) ([]models.Registered, error)
}

type RegisterDeleter interface {
	DeletePcByID(ctx context.Context, pcID string) error
}

type CanPowerOnSetter interface {
	SetCanPowerOn(ctx context.Context, pcID string, canPowerOn bool) error
}

func New(
	log *slog.Logger,
	getter RegisteredGetter,
	deleter RegisterDeleter,
	setter CanPowerOnSetter,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "http-server.handlers.create-registered"
		log := log.With(sl.Op(op), sl.ReqID(r))

		reg, err := getter.GetPcs(r.Context())
		if err != nil {
			log.Error("failed to retrieve registered pcs")
			render.JSON(w, r, response.InternalError())
			return
		}

		errs := make([]error, 0, len(reg)*2)
		for _, pc := range reg {
			if err := setter.SetCanPowerOn(r.Context(), pc.ID, false); err != nil {
				errs = append(
					errs,
					fmt.Errorf(
						"%s: failed to set can_power_on for pc (id: %s): %w",
						op,
						pc.ID,
						err,
					),
				)
				continue
			}
			if err := deleter.DeletePcByID(r.Context(), pc.ID); err != nil {
				errs = append(
					errs,
					fmt.Errorf("%s: failed to delete pc (id: %s): %w", op, pc.ID, err),
				)
				continue
			}
		}
		if len(errs) > 0 {
			log.Error("failed to delete registered pcs", errors.Join(errs...))
			render.JSON(w, r, response.InternalError())
			return
		}

		log.Info("successfully deleted all pcs")
		render.JSON(w, r, response.OK[types.Nil](nil))
		return
	}
}
