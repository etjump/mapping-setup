// Package fetch downloads files over HTTPS with caching, resume, retries
// and checksum verification.
package fetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// File is something to download.
type File struct {
	URL    string
	Name   string // file name in the cache; must be unique per content
	SHA256 string // lowercase hex; empty means "not verified"
	Size   int64  // bytes, or 0 if unknown
}

// StatusError is returned for HTTP responses outside the 2xx range.
type StatusError struct {
	URL  string
	Code int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s: HTTP %d", e.URL, e.Code)
}

// Client performs all network access for the installer.
type Client struct {
	http       *http.Client
	noRedirect *http.Client
	userAgent  string
}

// New creates a client that identifies itself with the installer version.
func New(version string) *Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		TLSHandshakeTimeout:   20 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		IdleConnTimeout:       90 * time.Second,
	}
	return &Client{
		http: &http.Client{Transport: transport},
		noRedirect: &http.Client{
			Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		userAgent: "etmapping-setup/" + version,
	}
}

const smallRequestTimeout = 30 * time.Second

func (c *Client) newRequest(ctx context.Context, method, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	return req, nil
}

// GetBytes fetches a small resource into memory.
func (c *Client) GetBytes(ctx context.Context, url string, header http.Header) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, smallRequestTimeout)
	defer cancel()

	req, err := c.newRequest(ctx, http.MethodGet, url)
	if err != nil {
		return nil, err
	}
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, &StatusError{URL: url, Code: resp.StatusCode}
	}
	return io.ReadAll(io.LimitReader(resp.Body, 32<<20))
}

// GetJSON fetches url and decodes it into v.
func (c *Client) GetJSON(ctx context.Context, url string, header http.Header, v any) error {
	body, err := c.GetBytes(ctx, url, header)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

// Reachable reports whether the host behind url answers at all. Any HTTP
// response counts, including errors.
func (c *Client) Reachable(ctx context.Context, url string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := c.newRequest(ctx, http.MethodHead, url)
	if err != nil {
		return err
	}
	resp, err := c.noRedirect.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// RedirectLocation returns the Location header url redirects to.
func (c *Client) RedirectLocation(ctx context.Context, url string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, smallRequestTimeout)
	defer cancel()

	req, err := c.newRequest(ctx, http.MethodHead, url)
	if err != nil {
		return "", err
	}
	resp, err := c.noRedirect.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", &StatusError{URL: url, Code: resp.StatusCode}
	}
	return loc, nil
}

// Size returns the download size of url, following redirects, or 0 if the
// server does not say.
func (c *Client) Size(ctx context.Context, url string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, smallRequestTimeout)
	defer cancel()

	req, err := c.newRequest(ctx, http.MethodHead, url)
	if err != nil {
		return 0, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return 0, &StatusError{URL: url, Code: resp.StatusCode}
	}
	if resp.ContentLength > 0 {
		return resp.ContentLength, nil
	}
	return 0, nil
}

// Progress is called while downloading with the bytes received so far and
// the total size (0 if unknown).
type Progress func(done, total int64)

const attempts = 4

// Download fetches f into dir and returns the local path. A complete file
// already in dir is reused; a partial one is resumed.
func (c *Client) Download(ctx context.Context, f File, dir string, progress Progress) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	final := filepath.Join(dir, f.Name)
	part := final + ".part"

	if _, err := os.Stat(final); err == nil {
		if err := verify(final, f.SHA256); err == nil {
			return final, nil
		}
		os.Remove(final)
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(attempt*attempt) * time.Second):
			}
		}
		lastErr = c.downloadOnce(ctx, f.URL, part, progress)
		if lastErr == nil {
			break
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		var se *StatusError
		if errors.As(lastErr, &se) && se.Code >= 400 && se.Code < 500 && se.Code != http.StatusTooManyRequests {
			break // retrying will not help
		}
	}
	if lastErr != nil {
		return "", lastErr
	}

	if err := verify(part, f.SHA256); err != nil {
		os.Remove(part)
		return "", err
	}
	if err := os.Rename(part, final); err != nil {
		return "", err
	}
	return final, nil
}

func (c *Client) downloadOnce(ctx context.Context, url, part string, progress Progress) error {
	var offset int64
	if fi, err := os.Stat(part); err == nil {
		offset = fi.Size()
	}

	req, err := c.newRequest(ctx, http.MethodGet, url)
	if err != nil {
		return err
	}
	if offset > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	flags := os.O_CREATE | os.O_WRONLY
	switch {
	case resp.StatusCode == http.StatusPartialContent && offset > 0:
		flags |= os.O_APPEND
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && offset > 0:
		// The partial file cannot be resumed; start over on the next attempt.
		os.Remove(part)
		return fmt.Errorf("%s: cannot resume download", url)
	case resp.StatusCode >= 200 && resp.StatusCode <= 299:
		offset = 0
		flags |= os.O_TRUNC
	default:
		return &StatusError{URL: url, Code: resp.StatusCode}
	}

	total := int64(0)
	if resp.ContentLength > 0 {
		total = offset + resp.ContentLength
	}

	out, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	done := offset
	buf := make([]byte, 256<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			done += int64(n)
			if progress != nil {
				progress(done, total)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	if total > 0 && done != total {
		return fmt.Errorf("%s: incomplete download (%d of %d bytes)", url, done, total)
	}
	return out.Close()
}

// FormatSize renders a byte count for people.
func FormatSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func verify(path, want string) error {
	if want == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("%s: checksum mismatch (expected %s, got %s)", filepath.Base(path), want, got)
	}
	return nil
}
