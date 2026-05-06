package configStorage

import (
	"context"
	"fmt"
	"slices"
	"smart-pc-waker-agent/internal/config"
	"smart-pc-waker-agent/internal/domain/models"
	"smart-pc-waker-agent/internal/storage"
	"sync"

	"golang.org/x/oauth2"
)

type Storage struct {
	cfg *config.Config
	mut sync.Mutex
}

func New(cfg *config.Config) *Storage {
	return &Storage{
		cfg: cfg,
		mut: sync.Mutex{},
	}
}

func (s *Storage) GetMACByPcId(_ context.Context, pcId string) (string, error) {
	s.mut.Lock()
	defer s.mut.Unlock()

	for _, pc := range s.cfg.Storage.Pcs {
		if pc.ID == pcId {
			return pc.MAC, nil
		}
	}

	return "", storage.ErrNotFound
}

func (s *Storage) GetAuthToken(_ context.Context) (*oauth2.Token, error) {
	s.mut.Lock()
	defer s.mut.Unlock()

	token := s.cfg.Storage.AuthToken
	if token == nil {
		return nil, storage.ErrNotFound
	}

	return token, nil
}

func (s *Storage) SetAuthToken(_ context.Context, token *oauth2.Token) error {
	const op = "storage.config-storage.SetAuthToken"

	s.mut.Lock()
	defer s.mut.Unlock()

	s.cfg.Storage.AuthToken = token

	if err := s.cfg.Save(); err != nil {
		return fmt.Errorf("%s: failed to save config: %w", op, err)
	}

	return nil
}

func (s *Storage) SavePc(_ context.Context, pcID string, mac string) error {
	const op = "storage.config-storage.SavePc"

	s.mut.Lock()
	defer s.mut.Unlock()

	found := false
	for i := range s.cfg.Storage.Pcs {
		if s.cfg.Storage.Pcs[i].ID == pcID {
			s.cfg.Storage.Pcs[i].MAC = mac
			found = true
			break
		}
	}

	if !found {
		s.cfg.Storage.Pcs = append(s.cfg.Storage.Pcs, config.Pc{
			ID:  pcID,
			MAC: mac,
		})
	}

	if err := s.cfg.Save(); err != nil {
		return fmt.Errorf("%s: failed to save config: %w", op, err)
	}

	return nil
}

func (s *Storage) GetPcs(_ context.Context) ([]models.Registered, error) {
	s.mut.Lock()
	defer s.mut.Unlock()

	registered := make([]models.Registered, 0, len(s.cfg.Storage.Pcs))
	for _, pc := range s.cfg.Storage.Pcs {
		registered = append(registered, models.Registered{
			ID:  pc.ID,
			MAC: pc.MAC,
		})
	}

	return registered, nil
}

func (s *Storage) DeletePcByID(_ context.Context, pcID string) error {
	const op = "storage.config-storage.DeletePcByID"

	s.mut.Lock()
	defer s.mut.Unlock()

	s.cfg.Storage.Pcs = slices.DeleteFunc(s.cfg.Storage.Pcs, func(p config.Pc) bool {
		return p.ID == pcID
	})

	if err := s.cfg.Save(); err != nil {
		return fmt.Errorf("%s: failed to save config: %w", op, err)
	}

	return nil
}

func (s *Storage) DeleteAllPcs(_ context.Context) error {
	const op = "storage.config-storage.DeleteAllPcs"

	s.mut.Lock()
	defer s.mut.Unlock()

	s.cfg.Storage.Pcs = make([]config.Pc, 0)

	if err := s.cfg.Save(); err != nil {
		return fmt.Errorf("%s: failed to save config: %w", op, err)
	}

	return nil
}
