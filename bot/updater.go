package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	ghOwner = "Stoufiler"
	ghRepo  = "frigate-telegram-notifier"
)

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

func checkForUpdate(ctx context.Context) (*ghRelease, bool, error) {
	release, err := fetchLatestRelease(ctx)
	if err != nil {
		return nil, false, err
	}
	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(version, "v")
	return release, latest != current, nil
}

func selfUpdate(ctx context.Context) error {
	if version == "dev" {
		return fmt.Errorf("self-update is not available for dev builds — build with a version tag")
	}
	if isDocker() {
		return fmt.Errorf("running inside Docker — update your container image instead")
	}

	release, needsUpdate, err := checkForUpdate(ctx)
	if err != nil {
		return fmt.Errorf("check for update: %w", err)
	}
	if !needsUpdate {
		fmt.Printf("Already up to date (%s).\n", version)
		return nil
	}

	asset := findAsset(release.Assets)
	if asset == nil {
		return fmt.Errorf("no binary for %s/%s in release %s", runtime.GOOS, runtime.GOARCH, release.TagName)
	}

	fmt.Printf("Updating %s → %s (%s)...\n", version, release.TagName, asset.Name)

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolve symlinks: %w", err)
	}

	if err := downloadAndReplace(ctx, asset.DownloadURL, exe); err != nil {
		return err
	}

	fmt.Printf("Updated to %s. Restart the bot to use the new version.\n", release.TagName)
	return nil
}

func fetchLatestRelease(ctx context.Context) (*ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", ghOwner, ghRepo)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API %s: %s", resp.Status, body)
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func downloadAndReplace(ctx context.Context, url, exe string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: %s", resp.Status)
	}

	tmp, err := os.CreateTemp(filepath.Dir(exe), ".frigate-bot-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write update: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("flush update: %w", err)
	}

	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	if err := os.Rename(tmpPath, exe); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}

func findAsset(assets []ghAsset) *ghAsset {
	name := assetName()
	for i := range assets {
		if assets[i].Name == name {
			return &assets[i]
		}
	}
	return nil
}

func assetName() string {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "linux/amd64":
		return "frigate_bot_linux_amd64"
	case "linux/arm64":
		return "frigate_bot_linux_arm64"
	case "darwin/amd64":
		return "frigate_bot_macos_amd64"
	case "darwin/arm64":
		return "frigate_bot_macos_arm64"
	case "windows/amd64":
		return "frigate_bot_windows.exe"
	default:
		return fmt.Sprintf("frigate_bot_%s_%s", runtime.GOOS, runtime.GOARCH)
	}
}

func isDocker() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return false
}
