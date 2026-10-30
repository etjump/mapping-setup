package sources

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/etjump/mapping-setup/internal/extract"
	"github.com/etjump/mapping-setup/internal/fetch"
	"github.com/etjump/mapping-setup/internal/platform"
)

// ETJump resolves the newest numbered ETJump release. The rolling "latest"
// tag is a development build and is deliberately ignored.
func ETJump(ctx context.Context, c *fetch.Client, t Target) (*Component, error) {
	const repo = "etjump/etjump"
	tag, err := c.NewestVersionTag(ctx, repo)
	if err != nil {
		return nil, err
	}
	zipName := "etjump-" + tag + ".zip"
	asset, err := c.ReleaseAsset(ctx, repo, tag, func(n string) bool { return n == zipName })
	if err != nil {
		return nil, err
	}

	// The release zip holds the mod pk3 plus the server module for every
	// platform; only the one matching this engine is needed.
	qagame := qagameFile(t)
	rule := func(name string) (string, bool) {
		dir, file := path.Split(strings.Trim(name, "/"))
		if dir != "etjump/" {
			return "", false
		}
		isPk3 := strings.HasPrefix(file, "etjump-") && strings.HasSuffix(file, ".pk3")
		return "etjump/" + file, isPk3 || file == qagame
	}

	return &Component{
		ID:      "etjump",
		Name:    "ETJump",
		Version: tag,
		Files:   []fetch.File{fileFromAsset(asset)},
		Install: func(paths []string) error {
			written, err := extract.Archive(paths[0], t.Root, rule)
			if err != nil {
				return err
			}
			dir := filepath.Join(t.Root, "etjump")
			if err := removeOldPk3s(dir, written); err != nil {
				return err
			}
			if !looseClientModules(t) {
				return nil
			}
			for _, w := range written {
				if strings.HasSuffix(w, ".pk3") {
					return extractClientModules(filepath.Join(t.Root, filepath.FromSlash(w)), dir)
				}
			}
			return nil
		},
	}, nil
}

// looseClientModules reports whether the client game and menu modules must
// sit next to their pk3s. ET 2.60b only extracts them itself when joining a
// server, so a local game (like testing a map) cannot load them from inside
// the pk3. ETe installs get them too, so the folder also works if the user
// switches to ET.exe later.
func looseClientModules(t Target) bool {
	return t.Platform.OS == platform.Windows && (t.Engine == "et260b" || strings.HasPrefix(t.Engine, "ete-"))
}

// extractClientModules unpacks the 32-bit client game and menu modules from
// pk3 into dir.
func extractClientModules(pk3, dir string) error {
	names := []string{"cgame_mp_x86.dll", "ui_mp_x86.dll"}
	files := map[string]string{}
	for _, n := range names {
		files[n] = n
	}
	written, err := extract.Archive(pk3, dir, extract.Only(0, files))
	if err != nil {
		return err
	}
	if len(written) != len(names) {
		return fmt.Errorf("%s does not contain %s", filepath.Base(pk3), strings.Join(names, " and "))
	}
	return nil
}

func qagameFile(t Target) string {
	switch {
	case t.Platform.OS == platform.MacOS:
		return "qagame_mac"
	case t.Platform.OS == platform.Windows && t.Is32Bit():
		return "qagame_mp_x86.dll"
	case t.Platform.OS == platform.Windows:
		return "qagame_mp_x64.dll"
	case t.Is32Bit():
		return "qagame.mp.i386.so"
	default:
		return "qagame.mp.x86_64.so"
	}
}

// removeOldPk3s deletes etjump-*.pk3 files from earlier versions, which
// the game would otherwise load alongside the new one.
func removeOldPk3s(dir string, written []string) error {
	keep := map[string]bool{}
	for _, w := range written {
		keep[path.Base(w)] = true
	}
	old, err := filepath.Glob(filepath.Join(dir, "etjump-*.pk3"))
	if err != nil {
		return err
	}
	for _, f := range old {
		if !keep[filepath.Base(f)] {
			if err := os.Remove(f); err != nil {
				return err
			}
		}
	}
	return nil
}
