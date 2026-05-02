package configStorage

import (
	"context"
	"fmt"
	"smart-pc-waker-agent/internal/config"
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
