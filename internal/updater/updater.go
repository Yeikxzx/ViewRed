// Package updater provides GitHub-Releases-based auto-update with
// self-replacement. It periodically checks the latest release, downloads
// the new binary for the current platform, and schedules in-place
// replacement on process exit.
//
// The updater is disabled for dev builds (appversion.Channel == "dev").
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/hashicorp/go-version"

	appversion "github.com/viewred/viewred/internal/version"
)

const (
	checkInterval = 6 * time.Hour
	repoOwner     = "Yeikxzx"
	repoName      = "ViewRed"
	tempSuffix    = ".new"
)

// Release represents a GitHub Release returned by the Releases API.
type Release struct {
	TagName string  `json:"tag_name"`
	Name    string  `json:"name"`
	Assets  []Asset `json:"assets"`
}

// Asset represents a single downloadable asset in a GitHub Release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// Updater checks GitHub Releases for newer versions and handles
// downloading and self-replacement of the current binary.
type Updater struct {
	current     *version.Version
	latest      *version.Version
	downloadURL string
	httpClient  *http.Client
	// apiURL overrides the GitHub Releases API endpoint (for tests).
	apiURL string
}

// New creates an Updater. Returns an error for dev builds.
func New() (*Updater, error) {
	current := appversion.Get()
	if current.Channel == "dev" {
		return nil, fmt.Errorf("updater disabled: running dev build %q", current.Version)
	}
	v, err := version.NewVersion(current.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid current version %q: %w", current.Version, err)
	}
	return &Updater{
		current:    v,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Check queries the latest release. Returns whether an update is available.
func (u *Updater) Check(ctx context.Context) (updateAvailable bool, latestVersion string, err error) {
	apiURL := u.apiURL
	if apiURL == "" {
		apiURL = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return false, "", fmt.Errorf("check request: %w", err)
	}
	req.Header.Set("User-Agent", appversion.Get().UserAgent())
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return false, "", fmt.Errorf("decode release: %w", err)
	}

	tag := strings.TrimPrefix(rel.TagName, "v")
	latest, err := version.NewVersion(tag)
	if err != nil {
		return false, "", fmt.Errorf("invalid release tag %q: %w", rel.TagName, err)
	}

	u.latest = latest

	if latest.GreaterThan(u.current) {
		for _, a := range rel.Assets {
			if a.Name == u.assetName() {
				u.downloadURL = a.URL
				return true, latest.String(), nil
			}
		}
		return false, latest.String(), nil
	}

	return false, "", nil
}

func (u *Updater) assetName() string {
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	return fmt.Sprintf("viewred-%s-%s%s", runtime.GOOS, runtime.GOARCH, suffix)
}

// Download fetches the new binary to <executable>.new and returns the path.
func (u *Updater) Download(ctx context.Context) (string, error) {
	if u.downloadURL == "" {
		return "", fmt.Errorf("no download URL — call Check() first")
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("get executable path: %w", err)
	}
	tmpPath := exe + tempSuffix

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("download request: %w", err)
	}
	req.Header.Set("User-Agent", appversion.Get().UserAgent())

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download status %d", resp.StatusCode)
	}

	f, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}

	hasher := sha256.New()
	writer := io.MultiWriter(f, hasher)

	n, err := io.Copy(writer, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("write download: %w", err)
	}
	if n == 0 {
		os.Remove(tmpPath)
		return "", fmt.Errorf("downloaded zero bytes")
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0o755); err != nil {
			os.Remove(tmpPath)
			return "", fmt.Errorf("chmod: %w", err)
		}
	}

	log.Printf("updater: downloaded v%s (%d bytes, sha256=%s)", u.latest, n, hex.EncodeToString(hasher.Sum(nil)))
	return tmpPath, nil
}

// Run starts periodic update checking. Blocks until ctx is cancelled or an
// update is ready for self-replacement.
func (u *Updater) Run(ctx context.Context) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		available, ver, err := u.Check(ctx)
		if err != nil {
			log.Printf("updater: check: %v", err)
			continue
		}
		if !available {
			if ver != "" {
				log.Printf("updater: current %s, latest %s — no update needed", u.current, ver)
			}
			continue
		}

		log.Printf("updater: v%s available (current: %s)", ver, u.current)
		newPath, err := u.Download(ctx)
		if err != nil {
			log.Printf("updater: download: %v", err)
			continue
		}

		if err := u.ReplaceOnExit(newPath); err != nil {
			log.Printf("updater: replace: %v", err)
			continue
		}

		log.Printf("updater: v%s downloaded, will replace on exit", ver)
		return
	}
}
