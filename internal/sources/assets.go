package sources

import (
	"context"

	"github.com/etjump/mapping-setup/internal/extract"
	"github.com/etjump/mapping-setup/internal/fetch"
)

// SDK resolves the ET mapping SDK. It has no versions; the file on the
// server is always the current one.
func SDK(t Target) *Component {
	return &Component{
		ID:      "sdk",
		Name:    "ET mapping SDK",
		Version: "current",
		Files: []fetch.File{{
			URL:  "https://aciz.etjump.com/downloads/et-mapping-sdk.zip",
			Name: "et-mapping-sdk.zip",
		}},
		Install: func(paths []string) error {
			// The zip already contains the etmain/ folder.
			_, err := extract.Archive(paths[0], t.Root, extract.All())
			return err
		},
	}
}

// MappingAssets resolves the ETJump mapping assets (entity definitions,
// common textures and shaders) from the repository's master branch.
func MappingAssets(ctx context.Context, c *fetch.Client, t Target) (*Component, error) {
	return repoSnapshot(ctx, c, t, "etjump/mapping", "etjump-mapping", "ETJump mapping assets")
}

// ExtraAssets resolves the stock assets missing from the SDK.
func ExtraAssets(ctx context.Context, c *fetch.Client, t Target) (*Component, error) {
	return repoSnapshot(ctx, c, t, "Aciz/ET-extra-assets", "et-extra-assets", "ET extra assets")
}

// repoSnapshot downloads a repository's master branch at its current commit
// and unpacks its folders into etmain.
func repoSnapshot(ctx context.Context, c *fetch.Client, t Target, repo, id, name string) (*Component, error) {
	commit, err := c.BranchCommit(ctx, repo, "master")
	if err != nil {
		return nil, err
	}
	return &Component{
		ID:      id,
		Name:    name,
		Version: shortHash(commit),
		Files: []fetch.File{{
			URL:  "https://github.com/" + repo + "/archive/" + commit + ".zip",
			Name: id + "-" + shortHash(commit) + ".zip",
		}},
		Install: func(paths []string) error {
			_, err := extract.Archive(paths[0], t.etmain(), skipRootFiles)
			return err
		},
	}, nil
}
