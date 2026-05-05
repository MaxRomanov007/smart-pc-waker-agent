package getStatus

import (
	"log/slog"
	"net/http"

	"github.com/MaxRomanov007/smart-pc-go-lib/api/response"
	"github.com/MaxRomanov007/smart-pc-go-lib/logger/sl"
	"github.com/go-chi/render"
)

type AuthChecker interface {
	IsAuthorized() bool
}

type statusResponse struct {
	Authorized bool `json:"authorized"`
}

func New(log *slog.Logger, auth AuthChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "http-server.handlers.auth.get-status"
		log.With(sl.Op(op), sl.ReqID(r))

		authorized := auth.IsAuthorized()

		log.Info("got authorized", slog.Bool("authorized", authorized))

		render.JSON(w, r, response.OK(&statusResponse{
			Authorized: authorized,
		}))
	}
}
