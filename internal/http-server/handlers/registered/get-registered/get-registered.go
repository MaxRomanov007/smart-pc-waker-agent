package getRegistered

import (
	"context"
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

func New(log *slog.Logger, getter RegisteredGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "http-server.handlers.create-registered"
		log := log.With(sl.Op(op), sl.ReqID(r))

		registered, err := getter.GetPcs(r.Context())
		if err != nil {
			log.Error("failed to get the registered pcs")
			render.JSON(w, r, response.InternalError())
			return
		}

		log.Debug("got registered pcs", slog.Any("registered", registered))
		render.JSON(w, r, response.OK(&registered))
		return
	}
}
