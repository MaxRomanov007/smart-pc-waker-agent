package authCallback

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/MaxRomanov007/smart-pc-go-lib/api/response"
	"github.com/MaxRomanov007/smart-pc-go-lib/logger/sl"
	"github.com/go-chi/render"
)

type AuthFlowCompleter interface {
	CompleteAuthFlow(ctx context.Context, state, code string) error
}

func New(log *slog.Logger, auth AuthFlowCompleter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "http-server.handlers.auth.callback"
		log := log.With(sl.Op(op), sl.ReqID(r))

		state := r.URL.Query().Get("state")
		code := r.URL.Query().Get("code")

		if state == "" || code == "" {
			log.Warn("missing state or code")
			render.JSON(w, r, response.BadRequest("missing state or code"))
			return
		}

		if err := auth.CompleteAuthFlow(r.Context(), state, code); err != nil {
			log.Error("failed to complete auth flow", sl.Err(err))
			render.JSON(w, r, response.InternalError())
			return
		}

		log.Info("authorization completed successfully")
		fmt.Fprintf(w, "auth completed successfully")
	}
}
