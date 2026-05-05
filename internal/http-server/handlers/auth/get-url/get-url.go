package getURL

import (
	"log/slog"
	"net/http"

	"github.com/MaxRomanov007/smart-pc-go-lib/api/response"
	"github.com/MaxRomanov007/smart-pc-go-lib/logger/sl"
	"github.com/go-chi/render"
)

type AuthFlowBeginner interface {
	IsAuthorized() bool
	BeginAuthFlow(redirectURL string) (string, error)
}

type urlResponse struct {
	URL string `json:"url"`
}

func New(log *slog.Logger, auth AuthFlowBeginner, callbackURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "http-server.handlers.auth.get-url"
		log := log.With(sl.Op(op), sl.ReqID(r))

		if auth.IsAuthorized() {
			log.Warn("already authorized")
			render.JSON(w, r, response.BadRequest("already authorized"))
			return
		}

		url, err := auth.BeginAuthFlow(callbackURL)
		if err != nil {
			log.Error("failed to begin auth flow", sl.Err(err))
			render.JSON(w, r, response.InternalError())
			return
		}

		render.JSON(w, r, response.OK(&urlResponse{URL: url}))
	}
}
