package sources

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/etjump/mapping-setup/internal/extract"
	"github.com/etjump/mapping-setup/internal/fetch"
	"github.com/etjump/mapping-setup/internal/platform"
)

func toolPlatform(t Target) string {
	if t.Platform.OS == platform.Linux {
		return "linux-x64"
	}
	return "win-x64"
}

// Pack3r resolves the rolling "latest" build of Pack3r.
func Pack3r(ctx context.Context, c *fetch.Client, t Target) (*Component, error) {
	suffix := "-" + toolPlatform(t) + ".zip"
	asset, err := c.ReleaseAsset(ctx, "ovska/Pack3r", "latest", func(n string) bool {
		return strings.HasPrefix(n, "pack3r-") && strings.HasSuffix(n, suffix)
	})
	if err != nil {
		return nil, err
	}
	// Asset names look like pack3r-<commit>-win-x64.zip.
	commit := strings.TrimSuffix(strings.TrimPrefix(asset.Name, "pack3r-"), suffix)

	dir := toolDir(t.Root, "pack3r")
	return &Component{
		ID:      "pack3r",
		Name:    "Pack3r",
		Version: "build " + shortHash(commit),
		Files:   []fetch.File{fileFromAsset(asset)},
		Install: func(paths []string) error {
			noDebug := func(n string) (string, bool) {
				ext := filepath.Ext(n)
				return n, ext != ".pdb" && ext != ".dbg"
			}
			if _, err := extract.Archive(paths[0], dir, noDebug); err != nil {
				return err
			}
			if t.Platform.OS == platform.Linux {
				return extract.MarkExecutable(dir, "Pack3r")
			}
			return nil
		},
	}, nil
}

// QtPack3r resolves the newest QtPack3r release.
func QtPack3r(ctx context.Context, c *fetch.Client, t Target) (*Component, error) {
	const repo = "Aciz/QtPack3r"
	tag, err := c.LatestReleaseTag(ctx, repo)
	if err != nil {
		return nil, err
	}
	name := "QtPack3r-" + tag + "-" + toolPlatform(t) + ".zip"
	asset, err := c.ReleaseAsset(ctx, repo, tag, func(n string) bool { return n == name })
	if err != nil {
		return nil, err
	}

	dir := toolDir(t.Root, "qtpack3r")
	return &Component{
		ID:      "qtpack3r",
		Name:    "QtPack3r",
		Version: tag,
		Files:   []fetch.File{fileFromAsset(asset)},
		Install: func(paths []string) error {
			if _, err := extract.Archive(paths[0], dir, extract.All()); err != nil {
				return err
			}
			if t.Platform.OS == platform.Linux {
				return extract.MarkExecutable(dir, "QtPack3r")
			}
			return nil
		},
	}, nil
}
