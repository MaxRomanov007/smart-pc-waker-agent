package pcsService

import (
	"context"
	"fmt"
	"net/http"
	"smart-pc-waker-agent/internal/config"

	"github.com/MaxRomanov007/smart-pc-go-lib/api/response"
	"github.com/MaxRomanov007/smart-pc-go-lib/authorization"
	"github.com/MaxRomanov007/smart-pc-go-lib/domain/models"
)

type Service struct {
	apiClient *authorization.ApiClient
	baseURL   string
}

func (s *Service) SetCanPowerOn(ctx context.Context, pcID string, canPowerOn bool) error {
	const op = "pcs-service.UpdatePcCommand"

	resp, err := authorization.DoNewRequest[models.Command](
		ctx,
		s.apiClient,
		http.MethodPatch,
		s.url(fmt.Sprintf("/pcs/%s", pcID)),
		struct {
			CanPowerOn bool `json:"canPowerOn"`
		}{
			CanPowerOn: canPowerOn,
		},
	)
	if err != nil {
		return fmt.Errorf("%s: failed to do request: %w", op, err)
	}

	if resp.Status != response.StatusOK {
		return fmt.Errorf("%s: response status is not ok: %s", op, resp.Status)
	}

	return nil
}

func (s *Service) GetPcs(ctx context.Context) ([]models.Pc, error) {
	const op = "pcs-service.GetPcs"

	resp, err := authorization.DoNewRequest[[]models.Pc](
		ctx,
		s.apiClient,
		http.MethodGet,
		s.url("/pcs"),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to do request: %w", op, err)
	}

	if resp.Status != response.StatusOK {
		return nil, fmt.Errorf("%s: status is not OK: %s", op, resp.Status)
	}

	return *resp.Data, nil
}

func New(
	ctx context.Context,
	auth *authorization.Auth,
	cfg config.PcsService,
) (*Service, error) {
	const op = "pcs-service.New"

	client, err := auth.NewApiClient(ctx, &http.Client{
		Timeout: cfg.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create api client: %w", op, err)
	}

	service := &Service{
		apiClient: client,
		baseURL:   cfg.BaseURL,
	}

	return service, nil
}

func (s *Service) url(endpoint string) string {
	return fmt.Sprintf("%s/u/%s%s", s.baseURL, s.apiClient.UID, endpoint)
}
