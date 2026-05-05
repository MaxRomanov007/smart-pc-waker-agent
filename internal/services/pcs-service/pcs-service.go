package pcsService

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	authorization "smart-pc-waker-agent/internal/auth"
	"smart-pc-waker-agent/internal/config"

	"github.com/MaxRomanov007/smart-pc-go-lib/api/response"
	apiclient "github.com/MaxRomanov007/smart-pc-go-lib/authorization/api-client"
	"github.com/MaxRomanov007/smart-pc-go-lib/domain/models"
)

type Service struct {
	apiClient *apiclient.Client
	baseURL   string
}

func (s *Service) SetCanPowerOn(ctx context.Context, pcID string, canPowerOn bool) error {
	const op = "pcs-service.UpdatePcCommand"

	resp, err := apiclient.Send[models.Command](
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

	resp, err := apiclient.Send[[]models.Pc](
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

func (s *Service) SetCanPowerOnForIds(ctx context.Context, pcIDs []string, canPowerOn bool) error {
	const op = "pcs-service.UpdatePcCommand"

	errs := make([]error, 0, len(pcIDs))
	for _, pcID := range pcIDs {
		if err := s.SetCanPowerOn(ctx, pcID, canPowerOn); err != nil {
			errs = append(errs, fmt.Errorf("failed to set can_power_on (pcID: %s): %w", pcID, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s: failed to set can_power_on: %w", op, errors.Join(errs...))
	}

	return nil
}

func New(
	auth *authorization.Auth,
	cfg config.PcsService,
) (*Service, error) {
	const op = "pcs-service.New"

	a, err := auth.Inner()
	if err != nil {
		return nil, fmt.Errorf("%s: failed to get auth: %w", op, err)
	}

	client := apiclient.New(&http.Client{Timeout: cfg.Timeout}, a)

	service := &Service{
		apiClient: client,
		baseURL:   cfg.BaseURL,
	}

	return service, nil
}

func (s *Service) url(endpoint string) string {
	return fmt.Sprintf("%s/u/%s%s", s.baseURL, s.apiClient.UID, endpoint)
}
