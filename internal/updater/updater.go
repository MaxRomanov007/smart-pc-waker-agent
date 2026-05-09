// Package updater provides background self-update logic for the router agent.
// It polls GitHub Releases, downloads the new binary, atomically replaces the
// running executable via go-update, and restarts the process via syscall.Exec
// so the init system (procd / SysV / rc.local) sees a clean restart.
package updater

import (
	"context"
	"log/slog"
	"time"

	"github.com/MaxRomanov007/smart-pc-go-lib/logger/sl"
)

const defaultInterval = 3 * time.Hour

// UpdatedFunc is called after the new binary is applied, just before exec.
// Use it for cleanup or logging. Keep it short — exec follows immediately.
type UpdatedFunc func(newVersion string)

// Service polls GitHub and self-updates the running binary.
type Service struct {
	log            *slog.Logger
	repo           string
	currentVersion string
	interval       time.Duration
	onUpdated      UpdatedFunc
}

// Option configures the Service.
type Option func(*Service)

// WithInterval overrides the default 3h polling interval.
func WithInterval(d time.Duration) Option {
	return func(s *Service) { s.interval = d }
}

// WithOnUpdated registers a callback invoked after a successful update,
// just before the process is replaced via exec.
func WithOnUpdated(fn UpdatedFunc) Option {
	return func(s *Service) { s.onUpdated = fn }
}

// New creates a Service and immediately starts the background polling loop.
// repo is the GitHub repository in "owner/repo" format, e.g. "MaxRomanov007/smart-pc-waker-agent".
// currentVersion must match the release tag format, e.g. "v1.2.3".
func New(
	ctx context.Context,
	log *slog.Logger,
	repo, currentVersion string,
	opts ...Option,
) *Service {
	s := &Service{
		log:            log.With("component", "updater"),
		repo:           repo,
		currentVersion: currentVersion,
		interval:       defaultInterval,
	}
	for _, o := range opts {
		o(s)
	}
	go s.run(ctx)
	return s
}

func (s *Service) run(ctx context.Context) {
	s.log.Info("updater started",
		slog.String("current", s.currentVersion),
		slog.String("repo", s.repo),
		slog.String("arch", archName()),
	)

	// Wait for the network to come up before the first check.
	// On routers the agent starts at boot before the WAN link is established.
	select {
	case <-ctx.Done():
		return
	case <-time.After(30 * time.Second):
	}

	s.checkWithRetry(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("updater stopped")
			return
		case <-ticker.C:
			s.checkWithRetry(ctx)
		}
	}
}

// checkWithRetry retries the check up to 3 times with exponential backoff.
// This handles transient network failures at boot or after a brief WAN drop.
func (s *Service) checkWithRetry(ctx context.Context) {
	backoff := 2 * time.Minute
	for attempt := 1; attempt <= 3; attempt++ {
		err := s.check()
		if err == nil {
			return
		}

		s.log.Warn("update check failed, will retry",
			slog.Int("attempt", attempt),
			slog.Duration("backoff", backoff),
			"err", err,
		)

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
			backoff *= 2
		}
	}
}

// check performs a single update check and applies it if a newer version is found.
// Returns nil both when already up to date and after a successful update+restart.
// Returns an error only on network/API failure so checkWithRetry can decide to retry.
func (s *Service) check() error {
	release, found, err := s.Check()
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	s.log.Info("applying update", slog.String("version", release.Version))

	if err := apply(release); err != nil {
		s.log.Error("update failed", sl.Err(err))
		return nil // apply errors are not retryable — binary may be corrupt
	}

	if s.onUpdated != nil {
		s.onUpdated(release.Version)
	}

	// Replace the process image — init system will see a clean restart.
	// Never returns on success.
	restart()
	return nil
}

// Check fetches the latest release from GitHub.
// Returns (release, true, nil) if a newer version is available,
// (zero, false, nil) if already up to date, or (zero, false, err) on failure.
func (s *Service) Check() (ReleaseInfo, bool, error) {
	release, err := fetchLatestRelease(s.repo)
	if err != nil {
		s.log.Warn("update check failed", sl.Err(err))
		return ReleaseInfo{}, false, err
	}

	if release.Version == s.currentVersion {
		s.log.Debug("already up to date", slog.String("version", s.currentVersion))
		return ReleaseInfo{}, false, nil
	}

	s.log.Info("update available",
		slog.String("current", s.currentVersion),
		slog.String("latest", release.Version),
	)
	return release, true, nil
}
