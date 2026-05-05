package auth

import (
	"context"
	"errors"
	"fmt"
	"smart-pc-waker-agent/internal/config"
	"smart-pc-waker-agent/internal/storage"
	"sync"

	"github.com/MaxRomanov007/smart-pc-go-lib/authorization"
	"golang.org/x/oauth2"
)

type TokenGetter interface {
	GetAuthToken(ctx context.Context) (*oauth2.Token, error)
}

type TokenSetter interface {
	SetAuthToken(ctx context.Context, token *oauth2.Token) error
}

// Auth — потокобезопасная обёртка вокруг *authorization.Auth.
//
// Если при старте токен не найден, Auth остаётся в неавторизованном состоянии.
// Клиент инициирует flow через /auth/url (→ BeginAuthFlow),
// пользователь проходит авторизацию, провайдер редиректит на /auth/callback
// (→ CompleteAuthFlow), после чего Auth становится готовым к использованию.
type Auth struct {
	mu    sync.RWMutex
	inner *authorization.Auth
	flow  *authorization.AuthFlow // активный PKCE-flow, nil если flow не начат
	cfg   *authorization.Config
	ready chan struct{}
}

func New(
	ctx context.Context,
	cfg config.Auth,
	getter TokenGetter,
	setter TokenSetter,
) (*Auth, error) {
	const op = "auth.New"

	authConfig := &authorization.Config{
		Oauth2Config: &oauth2.Config{
			ClientID: cfg.Oauth2.ClientID,
			Scopes:   cfg.Oauth2.Scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:  cfg.Oauth2.Endpoint.AuthURL,
				TokenURL: cfg.Oauth2.Endpoint.TokenURL,
			},
		},
		LoadToken: func(inner context.Context) (*oauth2.Token, error) {
			return getter.GetAuthToken(inner)
		},
		SaveToken: func(inner context.Context, token *oauth2.Token) error {
			return setter.SetAuthToken(inner, token)
		},
		UserInfoURL: cfg.UserinfoURL,
	}

	a := &Auth{
		cfg:   authConfig,
		ready: make(chan struct{}),
	}

	inner, err := authorization.Load(ctx, authConfig)
	if errors.Is(err, storage.ErrNotFound) {
		return a, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	a.inner = inner
	close(a.ready)
	return a, nil
}

// IsAuthorized возвращает true, если токен получен и Auth готов к работе.
func (a *Auth) IsAuthorized() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.inner != nil
}

// BeginAuthFlow подготавливает PKCE-параметры и возвращает URL для авторизации.
// redirectURL — полный адрес вашего /auth/callback роута.
// Повторный вызов до завершения предыдущего flow перезаписывает его.
func (a *Auth) BeginAuthFlow(redirectURL string) (string, error) {
	const op = "auth.BeginAuthFlow"

	if a.IsAuthorized() {
		return "", fmt.Errorf("%s: already authorized", op)
	}

	flow, err := a.cfg.PrepareAuthFlow(redirectURL)
	if err != nil {
		return "", fmt.Errorf("%s: failed to prepare auth flow: %w", op, err)
	}

	a.mu.Lock()
	a.flow = flow
	a.mu.Unlock()

	return flow.URL, nil
}

// CompleteAuthFlow обменивает code (из OAuth2 callback) на токен.
// Вызывается из хендлера /auth/callback.
func (a *Auth) CompleteAuthFlow(ctx context.Context, state, code string) error {
	const op = "auth.CompleteAuthFlow"

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.inner != nil {
		return fmt.Errorf("%s: already authorized", op)
	}
	if a.flow == nil {
		return fmt.Errorf("%s: no active auth flow; call /auth/url first", op)
	}

	inner, err := a.flow.Finalize(ctx, state, code)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	a.inner = inner
	close(a.ready)
	a.flow = nil
	return nil
}

// Inner возвращает *authorization.Auth для сервисов, которым он нужен напрямую.
// Возвращает ошибку если агент ещё не авторизован.
func (a *Auth) Inner() (*authorization.Auth, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.inner == nil {
		return nil, fmt.Errorf("not authorized")
	}
	return a.inner, nil
}

// WaitReady блокируется до завершения авторизации или отмены ctx.
func (a *Auth) WaitReady(ctx context.Context) error {
	select {
	case <-a.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
