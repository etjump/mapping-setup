package sources

import (
	"context"
	"fmt"
	"regexp"

	"github.com/etjump/mapping-setup/internal/extract"
	"github.com/etjump/mapping-setup/internal/fetch"
	"github.com/etjump/mapping-setup/internal/platform"
)

// Engine resolves the game engine the user picked.
func Engine(ctx context.Context, c *fetch.Client, t Target) (*Component, error) {
	switch t.Engine {
	case "etlegacy-64", "etlegacy-32":
		return etLegacy(ctx, c, t)
	case "ete-64", "ete-32":
		return ete(ctx, c, t)
	case "et260b":
		return et260b(t)
	default:
		return nil, fmt.Errorf("unknown engine %q", t.Engine)
	}
}

// GameExecutable is the engine's program file, relative to the install root.
func GameExecutable(t Target) string {
	p := t.Platform.OS
	switch t.Engine {
	case "etlegacy-64":
		switch p {
		case platform.Windows:
			return "etl.exe"
		case platform.MacOS:
			return "ET Legacy.app"
		default:
			return "etl.x86_64"
		}
	case "etlegacy-32":
		if p == platform.Windows {
			return "etl.exe"
		}
		return "etl.i386"
	case "ete-64":
		if p == platform.Windows {
			return "ETe.x64.exe"
		}
		return "ete.x86_64"
	case "ete-32":
		if p == platform.Windows {
			return "ETe.exe"
		}
		return "ete.x86"
	default:
		if p == platform.Windows {
			return "ET.exe"
		}
		return "et.x86"
	}
}

// ET: Legacy publishes its downloads only on etlegacy.com, as links with
// changing numeric IDs. The download page labels each link with its file
// name, which carries the version.

var (
	anchorRe   = regexp.MustCompile(`(?s)<a\s[^>]*>`)
	dataHrefRe = regexp.MustCompile(`data-href="([^"]+)"`)
	titleRe    = regexp.MustCompile(`title="([^"]+)"`)
)

func etLegacy(ctx context.Context, c *fetch.Client, t Target) (*Component, error) {
	var suffix string
	switch {
	case t.Platform.OS == platform.Windows && t.Is32Bit():
		suffix = "-x86.zip"
	case t.Platform.OS == platform.Windows:
		suffix = "-x64.zip"
	case t.Platform.OS == platform.Linux && t.Is32Bit():
		suffix = "-i386.tar.gz"
	case t.Platform.OS == platform.Linux:
		suffix = "-x86_64.tar.gz"
	default:
		suffix = "-macOS.tar.gz"
	}

	page, err := c.GetBytes(ctx, "https://www.etlegacy.com/download", nil)
	if err != nil {
		return nil, fmt.Errorf("reading the ET: Legacy download page: %w", err)
	}

	fileRe := regexp.MustCompile(`^etlegacy-v(\d+(?:\.\d+)*)` + regexp.QuoteMeta(suffix) + `$`)
	var bestName, bestURL, bestVersion string
	for _, a := range anchorRe.FindAll(page, -1) {
		href, title := dataHrefRe.FindSubmatch(a), titleRe.FindSubmatch(a)
		if href == nil || title == nil {
			continue
		}
		m := fileRe.FindStringSubmatch(string(title[1]))
		if m == nil {
			continue
		}
		if fetch.CompareVersions(fetch.ParseVersion(m[1]), fetch.ParseVersion(bestVersion)) > 0 {
			bestName, bestURL, bestVersion = m[0], string(href[1]), m[1]
		}
	}
	if bestURL == "" {
		return nil, fmt.Errorf("could not find the ET: Legacy download (etlegacy-v*%s) on etlegacy.com; the page may have changed", suffix)
	}

	return &Component{
		ID:      "etlegacy",
		Name:    "ET: Legacy",
		Version: bestVersion,
		Files:   []fetch.File{{URL: bestURL, Name: bestName}},
		Install: func(paths []string) error {
			_, err := extract.Archive(paths[0], t.Root, extract.StripComponents(1))
			return err
		},
	}, nil
}

func ete(ctx context.Context, c *fetch.Client, t Target) (*Component, error) {
	const repo, tag = "etfdevs/ETe", "latest"

	var name string
	var executables []string
	switch {
	case t.Platform.OS == platform.Windows && t.Is32Bit():
		name = "ete-windows-msvc-x86.zip"
	case t.Platform.OS == platform.Windows:
		name = "ete-windows-msvc-x64.zip"
	case t.Is32Bit():
		name, executables = "ete-linux-x86.zip", []string{"ete.x86", "ete-ded.x86"}
	default:
		name, executables = "ete-linux-x86_64.zip", []string{"ete.x86_64", "ete-ded.x86_64"}
	}
	engine, err := c.ReleaseAsset(ctx, repo, tag, func(n string) bool { return n == name })
	if err != nil {
		return nil, err
	}
	files := []fetch.File{fileFromAsset(engine)}

	// The original etmain game code is 32-bit only. ETe ships a replacement
	// with 64-bit builds so the stock menus work on the 64-bit engine.
	if !t.Is32Bit() {
		const replacement = "ete-etmain-mod-replacement-allplatform.zip"
		etmain, err := c.ReleaseAsset(ctx, repo, tag, func(n string) bool { return n == replacement })
		if err != nil {
			return nil, err
		}
		files = append(files, fileFromAsset(etmain))
	}

	return &Component{
		ID:      "ete",
		Name:    "ETe",
		Version: "latest build",
		Files:   files,
		Install: func(paths []string) error {
			skipSteam := func(n string) (string, bool) { return n, n != "steam_appid.txt" }
			if _, err := extract.Archive(paths[0], t.Root, skipSteam); err != nil {
				return err
			}
			if err := extract.MarkExecutable(t.Root, executables...); err != nil {
				return err
			}
			if len(paths) > 1 {
				rule := extract.Only(0, map[string]string{"mp_bina.pk3": "etmain/mp_bina.pk3"})
				if _, err := extract.Archive(paths[1], t.Root, rule); err != nil {
					return err
				}
			}
			return nil
		},
	}, nil
}

func et260b(t Target) (*Component, error) {
	files := map[string]string{"win32/ET.exe": "ET.exe", "win32/ETDED.exe": "ETDED.exe"}
	var executables []string
	if t.Platform.OS == platform.Linux {
		files = map[string]string{"linux/et.x86": "et.x86", "linux/etded.x86": "etded.x86"}
		executables = []string{"et.x86", "etded.x86"}
	}

	return &Component{
		ID:      "et260b",
		Name:    "ET 2.60b",
		Version: "2.60b",
		Files: []fetch.File{{
			URL:    "https://mirror.etlegacy.com/ET-2.60b.zip",
			Name:   "ET-2.60b.zip",
			SHA256: "6fb77e1ed0314b2da5c2a28807392ac638204faec8ef86361893937f182b22f8",
			Size:   4143401,
		}},
		Install: func(paths []string) error {
			if _, err := extract.Archive(paths[0], t.Root, extract.Only(1, files)); err != nil {
				return err
			}
			return extract.MarkExecutable(t.Root, executables...)
		},
	}, nil
}
