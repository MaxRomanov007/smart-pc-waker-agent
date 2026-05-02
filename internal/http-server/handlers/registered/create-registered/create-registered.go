package createRegistered

import (
	"context"
	"go/types"
	"log/slog"
	"net/http"

	"github.com/MaxRomanov007/smart-pc-go-lib/api/response"
	"github.com/MaxRomanov007/smart-pc-go-lib/logger/sl"
	"github.com/MaxRomanov007/smart-pc-go-lib/middlewares/reqmw"
	"github.com/go-chi/render"
)

type Request struct {
	PcID string `json:"pcId" validate:"required,uuid"`
	MAC  string `json:"mac"  validate:"required,mac"`
}

type CanPowerOnSetter interface {
	SetCanPowerOn(ctx context.Context, pcID string, canPowerOn bool) error
}

type PcSaver interface {
	SavePc(ctx context.Context, pcID string, mac string) error
}

func New(log *slog.Logger, setter CanPowerOnSetter, saver PcSaver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "http-server.handlers.create-registered"
		log := log.With(sl.Op(op), sl.ReqID(r))

		req := reqmw.MustGet[Request](r)

		if err := setter.SetCanPowerOn(r.Context(), req.PcID, true); err != nil {
			log.Error("failed to set can_power_on to true", sl.Err(err))
			render.JSON(w, r, response.InternalError())
			return
		}

		if err := saver.SavePc(r.Context(), req.PcID, req.MAC); err != nil {
			log.Error("failed to save pc", sl.Err(err))
			render.JSON(w, r, response.InternalError())

			if err := setter.SetCanPowerOn(r.Context(), req.PcID, false); err != nil {
				log.Error("failed to set can_power_on to false", sl.Err(err))
			}
			return
		}

		log.Info("pc registered successfully")
		render.JSON(w, r, response.OK[types.Nil](nil))
		return
	}
}
