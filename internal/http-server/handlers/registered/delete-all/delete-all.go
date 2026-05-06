package deleteAll

import (
	"context"
	"go/types"
	"log/slog"
	"net/http"

	"github.com/MaxRomanov007/smart-pc-go-lib/api/response"
	"github.com/MaxRomanov007/smart-pc-go-lib/logger/sl"
	"github.com/go-chi/render"
)

type Cleaner interface {
	DeleteAllPcs(ctx context.Context) error
}

func New(log *slog.Logger, cleaner Cleaner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "http-server.handlers.create-registered"
		log := log.With(sl.Op(op), sl.ReqID(r))

		if err := cleaner.DeleteAllPcs(r.Context()); err != nil {
			log.Error("failed to delete all pcs", sl.Err(err))
			render.JSON(w, r, response.InternalError())
			return
		}

		log.Info("successfully deleted all pcs")
		render.JSON(w, r, response.OK[types.Nil](nil))
		return
	}
}
