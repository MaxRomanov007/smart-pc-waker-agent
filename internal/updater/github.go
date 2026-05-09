package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const githubAPI = "https://api.github.com/repos/%s/releases/latest"

// ReleaseInfo holds the data we need from a GitHub release.
type ReleaseInfo struct {
	Version     string // e.g. "v1.2.3"
	DownloadURL string // direct URL to the .tar.gz asset
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func fetchLatestRelease(repo string) (ReleaseInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	url := fmt.Sprintf(githubAPI, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ReleaseInfo{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ReleaseInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ReleaseInfo{}, fmt.Errorf("github api returned %d", resp.StatusCode)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return ReleaseInfo{}, err
	}

	want := archName() + ".tar.gz"
	for _, a := range rel.Assets {
		if a.Name == want {
			return ReleaseInfo{
				Version:     rel.TagName,
				DownloadURL: a.BrowserDownloadURL,
			}, nil
		}
	}

	return ReleaseInfo{}, fmt.Errorf("no asset %q in release %s", want, rel.TagName)
}

// archName returns the asset base name for the running binary, matching the
// naming convention in the release workflow:
//
//	agent-linux-amd64
//	agent-linux-arm64
//	agent-linux-armv5 / armv6 / armv7
//	agent-linux-mips / mipsle / mips64le
//	agent-linux-386
func archName() string {
	arch := runtime.GOARCH
	switch arch {
	case "arm":
		arch = armVariant() // armv5 / armv6 / armv7
	}
	return fmt.Sprintf("agent-linux-%s", arch)
}

// downloadBinary fetches the .tar.gz asset and returns a reader over the raw
// binary inside the archive. The caller must close the returned ReadCloser.
func downloadBinary(release ReleaseInfo) (io.ReadCloser, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, release.DownloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	binary, err := extractFromTarGz(resp.Body, archName())
	if err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("extract: %w", err)
	}

	return binary, nil
}

// extractFromTarGz finds binaryName inside the .tar.gz stream and returns a
// reader over it. Mirrors installer/github.go extractBinary logic exactly.
func extractFromTarGz(body io.Reader, binaryName string) (io.ReadCloser, error) {
	gz, err := gzip.NewReader(body)
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("binary %q not found in archive", binaryName)
		}
		if err != nil {
			return nil, fmt.Errorf("tar: %w", err)
		}
		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == binaryName {
			return &tarEntry{Reader: tr, gz: gz}, nil
		}
	}
}

type tarEntry struct {
	io.Reader
	gz *gzip.Reader
}

func (e *tarEntry) Close() error { return e.gz.Close() }

// armVariant reads GOARM from the binary's build info to distinguish
// armv5 / armv6 / armv7. Falls back to "armv7" which is the most common.
func armVariant() string {
	// runtime.GOARM is not exported, but it is baked into the binary as a
	// build constant accessible via the internal/goarch package — which we
	// cannot import. The reliable cross-platform approach is to read
	// /proc/cpuinfo, mirroring what the installer's detectARMVersion does.
	data, err := readFile("/proc/cpuinfo")
	if err != nil {
		return "armv7"
	}

	for _, line := range strings.Split(data, "\n") {
		lower := strings.ToLower(line)
		if !strings.HasPrefix(lower, "cpu architecture") {
			continue
		}
		switch {
		case strings.Contains(lower, "5"):
			return "armv5"
		case strings.Contains(lower, "6"):
			return "armv6"
		case strings.Contains(lower, "7"):
			return "armv7"
		}
	}

	// Fallback: try uname string baked into the kernel version.
	uname, err := readFile("/proc/version")
	if err != nil {
		return "armv7"
	}
	switch {
	case strings.Contains(uname, "armv5"):
		return "armv5"
	case strings.Contains(uname, "armv6"):
		return "armv6"
	default:
		return "armv7"
	}
}
