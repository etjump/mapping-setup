package sources

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/etjump/mapping-setup/internal/extract"
	"github.com/etjump/mapping-setup/internal/fetch"
	"github.com/etjump/mapping-setup/internal/platform"
)

const radiantRepo = "Garux/netradiant-custom"

// Radiant resolves the newest NetRadiant-custom release.
func Radiant(ctx context.Context, c *fetch.Client, t Target) (*Component, error) {
	tag, err := c.LatestReleaseTag(ctx, radiantRepo)
	if err != nil {
		return nil, err
	}
	name := "netradiant-custom-" + tag + "-windows-x86_64.zip"
	if t.Platform.OS == platform.Linux {
		name = "netradiant-custom-" + tag + "-linux-x86_64.7z"
	}
	asset, err := c.ReleaseAsset(ctx, radiantRepo, tag, func(n string) bool { return n == name })
	if err != nil {
		return nil, err
	}

	dir := RadiantDir(t.Root)
	return &Component{
		ID:      "netradiant-custom",
		Name:    "NetRadiant-custom",
		Version: tag,
		Files:   []fetch.File{fileFromAsset(asset)},
		Install: func(paths []string) error {
			written, err := extract.Archive(paths[0], dir, extract.All())
			if err != nil {
				return err
			}
			// The Linux build is a single AppImage. It is unpacked so the
			// game pack and settings are real folders the setup can write
			// into, the same as on Windows.
			for _, w := range written {
				if !strings.HasSuffix(w, ".AppImage") {
					continue
				}
				appImage := filepath.Join(dir, filepath.FromSlash(w))
				if _, err := extract.AppImage(appImage, dir); err != nil {
					return err
				}
				if err := os.Remove(appImage); err != nil {
					return err
				}
			}
			return nil
		},
	}, nil
}

// RadiantBinDir is the folder holding the Radiant program itself, where it
// looks for its game packs and portable settings. In an unpacked AppImage
// that is usr/bin.
func RadiantBinDir(dir string) string {
	if bin := filepath.Join(dir, "usr", "bin"); fileExists(filepath.Join(bin, "radiant.x86_64")) {
		return bin
	}
	return dir
}

// RadiantExecutable returns the file that starts the editor inside dir, or
// "" if it cannot be found.
func RadiantExecutable(dir string) string {
	if p := filepath.Join(dir, "radiant.exe"); fileExists(p) {
		return p
	}
	if p := filepath.Join(dir, "AppRun"); fileExists(p) {
		return p // unpacked AppImage
	}
	if matches, _ := filepath.Glob(filepath.Join(dir, "*.AppImage")); len(matches) > 0 {
		return matches[0]
	}
	return ""
}

// RadiantSettingsVersion returns the "1.<major>.<minor>" folder name Radiant
// keeps its settings in, from the version files next to the program.
func RadiantSettingsVersion(binDir string) string {
	major, minor := "6", "0" // values as of release 20260114
	if b, err := os.ReadFile(filepath.Join(binDir, "RADIANT_MAJOR")); err == nil {
		major = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile(filepath.Join(binDir, "RADIANT_MINOR")); err == nil {
		minor = strings.TrimSpace(string(b))
	}
	return "1." + major + "." + minor
}

// EntityDefDir finds the folder an editor loads ET's entity definitions
// from, or "" if dir does not look like a Radiant install with ET support.
func EntityDefDir(dir string) string {
	candidates := []string{
		"gamepacks/et.game/etmain",         // NetRadiant-custom
		"usr/bin/gamepacks/et.game/etmain", // unpacked Linux AppImage
		"squashfs-root/usr/bin/gamepacks/et.game/etmain",
		"et.game/etmain",                              // NetRadiant, GtkRadiant 1.5
		"Contents/Resources/gamepacks/et.game/etmain", // macOS app bundles
		"Contents/MacOS/gamepacks/et.game/etmain",
	}
	for _, rel := range candidates {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p
		}
	}
	return ""
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}
