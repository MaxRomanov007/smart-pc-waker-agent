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

	s.check()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("updater stopped")
			return
		case <-ticker.C:
			s.check()
		}
	}
}

func (s *Service) check() {
	release, found, err := s.Check()
	if err != nil || !found {
		return
	}

	s.log.Info("applying update", slog.String("version", release.Version))

	if err := apply(release); err != nil {
		s.log.Error("update failed", sl.Err(err))
		return
	}

	if s.onUpdated != nil {
		s.onUpdated(release.Version)
	}

	// Replace the process image — init system will see a clean restart.
	// Never returns on success.
	restart()
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
