package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"time"
)

// Frigate is a small HTTP client for the Frigate API with optional
// authentication and retries. It is safe for the bot's single-goroutine use.
type Frigate struct {
	baseURL    string
	user, pass string
	http       *http.Client
	maxRetries int
	retryDelay time.Duration
	loggedIn   bool
}

// NewFrigate builds a client for baseURL. When user and pass are both set, API
// requests are authenticated (Frigate's :8971 port); otherwise they are
// anonymous (:5000).
func NewFrigate(baseURL, user, pass string) *Frigate {
	jar, _ := cookiejar.New(nil)
	return &Frigate{
		baseURL:    strings.TrimRight(baseURL, "/"),
		user:       user,
		pass:       pass,
		http:       &http.Client{Timeout: 30 * time.Second, Jar: jar},
		maxRetries: 5,
		retryDelay: 2 * time.Second,
	}
}

func (f *Frigate) authEnabled() bool { return f.user != "" && f.pass != "" }

// Snapshot downloads the annotated snapshot for a detection event to dest.
// Both bbox (Frigate <=0.17) and bounding_box (0.18+) are passed so the
// bounding box is drawn regardless of version; the other is ignored.
func (f *Frigate) Snapshot(ctx context.Context, detectionID, dest string) error {
	url := fmt.Sprintf("%s/api/events/%s/snapshot.jpg?bbox=1&bounding_box=1", f.baseURL, detectionID)
	return f.download(ctx, url, dest, "snapshot")
}

// Preview downloads the review preview (animated GIF) to dest.
func (f *Frigate) Preview(ctx context.Context, reviewID, dest string) error {
	url := fmt.Sprintf("%s/api/review/%s/preview", f.baseURL, reviewID)
	return f.download(ctx, url, dest, "preview")
}

// login authenticates against Frigate; the JWT is stored in the cookie jar.
func (f *Frigate) login(ctx context.Context) error {
	body, _ := json.Marshal(map[string]string{"user": f.user, "password": f.pass})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.baseURL+"/api/login", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed: %s: %s", resp.Status, msg)
	}
	f.loggedIn = true
	return nil
}

// do performs req, transparently logging in when needed and re-logging in once
// on a 401 (handles token expiry for a long-running process).
func (f *Frigate) do(req *http.Request) (*http.Response, error) {
	if f.authEnabled() && !f.loggedIn {
		if err := f.login(req.Context()); err != nil {
			return nil, err
		}
	}
	resp, err := f.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized && f.authEnabled() {
		_ = resp.Body.Close()
		f.loggedIn = false
		if err := f.login(req.Context()); err != nil {
			return nil, err
		}
		return f.http.Do(req.Clone(req.Context()))
	}
	return resp, nil
}

// download fetches url into dest with retries. kind is used for log messages.
func (f *Frigate) download(ctx context.Context, url, dest, kind string) error {
	logf(tr("downloading_"+kind, map[string]any{"URL": url}))

	var lastErr error
	for attempt := 1; attempt <= f.maxRetries; attempt++ {
		if attempt > 1 {
			logf(tr("retry_attempt", map[string]any{"Attempt": attempt, "MaxRetries": f.maxRetries}))
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := f.do(req)
		if err != nil {
			lastErr = err
			logf(tr(kind+"_get_error", map[string]any{"Error": err}))
		} else if resp.StatusCode == http.StatusOK {
			err := saveBody(resp.Body, dest)
			_ = resp.Body.Close()
			return err
		} else {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("%s: %s", resp.Status, body)
			logf(tr(kind+"_failed", map[string]any{"Status": resp.Status, "Body": string(body)}))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(f.retryDelay):
		}
	}
	return fmt.Errorf("%s download failed after %d attempts: %w", kind, f.maxRetries, lastErr)
}

// saveBody streams an HTTP response body to a file on disk.
func saveBody(body io.Reader, dest string) error {
	file, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("%s: %w", tr("file_create_error", map[string]any{"Error": err}), err)
	}
	if _, err := io.Copy(file, body); err != nil {
		_ = file.Close()
		return fmt.Errorf("%s: %w", tr("file_write_error", map[string]any{"Error": err}), err)
	}
	// Close explicitly to surface flush errors on the written file.
	if err := file.Close(); err != nil {
		return fmt.Errorf("%s: %w", tr("file_write_error", map[string]any{"Error": err}), err)
	}
	return nil
}
