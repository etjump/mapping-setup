// Package sources knows where every part of the mapping setup comes from,
// how to find its latest version and how to install it.
package sources

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/etjump/mapping-setup/internal/fetch"
	"github.com/etjump/mapping-setup/internal/platform"
)

// Target describes the install being made.
type Target struct {
	Platform platform.Platform
	Engine   string // one of the platform.Engines IDs
	Root     string // the mapping install folder
}

// Is32Bit reports whether the chosen engine is a 32-bit build.
func (t Target) Is32Bit() bool {
	return t.Engine == "etlegacy-32" || t.Engine == "ete-32" || t.Engine == "et260b"
}

func (t Target) etmain() string { return filepath.Join(t.Root, "etmain") }

// Component is one resolved part of the setup: a specific version, the
// files to download for it and how to put them in place.
type Component struct {
	ID      string
	Name    string
	Version string
	Files   []fetch.File
	// Install receives the local paths of Files, in the same order.
	Install func(paths []string) error
}

// RadiantDir is where NetRadiant-custom gets installed.
func RadiantDir(root string) string { return filepath.Join(root, "netradiant-custom") }

func toolDir(root, name string) string { return filepath.Join(root, "tools", name) }

// cacheName gives rolling builds a cache file name that changes with their
// content, so a stale download is never reused.
func cacheName(a fetch.Asset) string {
	if len(a.SHA256) >= 12 {
		return a.SHA256[:12] + "-" + a.Name
	}
	return a.Name
}

func fileFromAsset(a fetch.Asset) fetch.File {
	return fetch.File{URL: a.URL, Name: cacheName(a), SHA256: a.SHA256, Size: a.Size}
}

func shortHash(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

// skipRootFiles keeps only entries inside a subfolder after stripping the
// archive's top folder, dropping things like README.md and LICENSE.
func skipRootFiles(name string) (string, bool) {
	parts := strings.SplitN(strings.Trim(name, "/"), "/", 3)
	if len(parts) < 3 {
		return "", false
	}
	return parts[1] + "/" + parts[2], true
}

// CopyFile copies src to dst, creating dst's folder if needed.
func CopyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
