package version

import (
	"log/slog"
	"net/http"

	"github.com/MaxRomanov007/smart-pc-go-lib/api/response"
	"github.com/MaxRomanov007/smart-pc-go-lib/logger/sl"
	"github.com/go-chi/render"
)

type Response struct {
	Version string `json:"version"`
}

func New(log *slog.Logger, version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "http-server.handlers.auth.callback"
		log := log.With(sl.Op(op), sl.ReqID(r))

		log.Info("returning version", slog.String("version", version))
		render.JSON(w, r, response.OK(&Response{Version: version}))
	}
}
