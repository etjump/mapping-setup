// Package install turns the user's choices into a plan of components and
// carries it out.
package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/etjump/mapping-setup/internal/fetch"
	"github.com/etjump/mapping-setup/internal/platform"
	"github.com/etjump/mapping-setup/internal/sources"
)

// Choices are the user's answers, stored in the manifest so an update can
// reuse them.
type Choices struct {
	Root           string   `json:"root"`
	Engine         string   `json:"engine"`
	InstallRadiant bool     `json:"installRadiant"`
	EditorDir      string   `json:"editorDir,omitempty"` // existing editor, when not installing one
	Extras         []string `json:"extras"`
}

// editorDir is the editor folder the setup configures, installed or not.
func (ch Choices) editorDir() string {
	if ch.InstallRadiant {
		return sources.RadiantDir(ch.Root)
	}
	return ch.EditorDir
}

// Plan is a resolved install: every component with a concrete version.
type Plan struct {
	Choices    Choices
	Target     sources.Target
	Components []*sources.Component
}

// DownloadSize is the total size of all downloads, and whether every size
// was known.
func (p *Plan) DownloadSize() (total int64, complete bool) {
	complete = true
	for _, comp := range p.Components {
		for _, f := range comp.Files {
			if f.Size == 0 {
				complete = false
			}
			total += f.Size
		}
	}
	return total, complete
}

// Resolve looks up the latest version of every component the choices call
// for. status is told which component is being looked up.
func Resolve(ctx context.Context, c *fetch.Client, ch Choices, p platform.Platform, status func(name string)) (*Plan, error) {
	t := sources.Target{Platform: p, Engine: ch.Engine, Root: ch.Root}
	plan := &Plan{Choices: ch, Target: t}

	type resolver struct {
		name string
		fn   func() (*sources.Component, error)
	}
	steps := []resolver{
		{"game engine", func() (*sources.Component, error) { return sources.Engine(ctx, c, t) }},
		{"game data", func() (*sources.Component, error) { return sources.GameData(t), nil }},
		{"ETJump", func() (*sources.Component, error) { return sources.ETJump(ctx, c, t) }},
	}
	if ch.InstallRadiant {
		steps = append(steps, resolver{"NetRadiant-custom", func() (*sources.Component, error) { return sources.Radiant(ctx, c, t) }})
	}
	steps = append(steps,
		resolver{"ET mapping SDK", func() (*sources.Component, error) { return sources.SDK(t), nil }},
		resolver{"ETJump mapping assets", func() (*sources.Component, error) { return sources.MappingAssets(ctx, c, t) }},
		resolver{"ET extra assets", func() (*sources.Component, error) { return sources.ExtraAssets(ctx, c, t) }},
	)
	if slices.Contains(ch.Extras, "pack3r") {
		steps = append(steps, resolver{"Pack3r", func() (*sources.Component, error) { return sources.Pack3r(ctx, c, t) }})
	}
	if slices.Contains(ch.Extras, "qtpack3r") {
		steps = append(steps, resolver{"QtPack3r", func() (*sources.Component, error) { return sources.QtPack3r(ctx, c, t) }})
	}

	for _, s := range steps {
		status(s.name)
		comp, err := s.fn()
		if err != nil {
			return nil, err
		}
		plan.Components = append(plan.Components, comp)
	}

	fillSizes(ctx, c, plan)
	return plan, nil
}

// fillSizes asks the servers for download sizes that are not known yet.
// Failures only leave the size unknown.
func fillSizes(ctx context.Context, c *fetch.Client, plan *Plan) {
	var wg sync.WaitGroup
	for _, comp := range plan.Components {
		for i := range comp.Files {
			f := &comp.Files[i]
			if f.Size > 0 {
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				if size, err := c.Size(ctx, f.URL); err == nil {
					f.Size = size
				}
			}()
		}
	}
	wg.Wait()
}

// Reporter shows install progress to the user and records it in the log.
type Reporter interface {
	Step(n, total int, title string)
	Progress(done, total int64)
	ProgressDone()
	Info(format string, args ...any) // shown and logged
	Log(format string, args ...any)  // logged only
}

const cacheDirName = ".etmapping-cache"

// Run downloads and installs every component of the plan, then applies the
// file edits the mapping setup needs.
func Run(ctx context.Context, c *fetch.Client, plan *Plan, version string, r Reporter) error {
	root := plan.Choices.Root
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}

	manifest := newManifest(version, plan)
	manifest.Status = statusInstalling
	if err := manifest.write(root); err != nil {
		return err
	}

	cache := filepath.Join(root, cacheDirName)
	total := len(plan.Components) + 1
	for i, comp := range plan.Components {
		r.Step(i+1, total, fmt.Sprintf("%s %s", comp.Name, comp.Version))
		var paths []string
		for _, f := range comp.Files {
			r.Log("downloading %s", f.URL)
			path, err := c.Download(ctx, f, cache, r.Progress)
			r.ProgressDone()
			if err != nil {
				return fmt.Errorf("downloading %s: %w", comp.Name, err)
			}
			paths = append(paths, path)
		}
		r.Log("installing %s %s", comp.Name, comp.Version)
		if err := comp.Install(paths); err != nil {
			return fmt.Errorf("installing %s: %w", comp.Name, err)
		}
	}

	r.Step(total, total, "Configuring the mapping setup")
	if err := configure(plan, r); err != nil {
		return err
	}

	manifest.Status = statusComplete
	if err := manifest.write(root); err != nil {
		return err
	}
	if err := os.RemoveAll(cache); err != nil {
		r.Log("could not remove the download cache: %v", err)
	}
	return nil
}

func configure(plan *Plan, r Reporter) error {
	root := plan.Choices.Root
	etmain := filepath.Join(root, "etmain")

	r.Info("Adding lightblock and blitz_sd_mm to shaderlist.txt")
	if err := ensureShaderlist(etmain, "lightblock", "blitz_sd_mm"); err != nil {
		return fmt.Errorf("updating shaderlist.txt: %w", err)
	}

	r.Info("Fixing the editor image in chateau.shader")
	if err := fixChateauShader(etmain); err != nil {
		return fmt.Errorf("updating chateau.shader: %w", err)
	}

	r.Info("Installing ETJump entity definitions into the editor")
	if err := installEntityDefs(etmain, plan.Choices.editorDir()); err != nil {
		return fmt.Errorf("installing et_entities.ent: %w", err)
	}

	if plan.Choices.InstallRadiant {
		r.Info("Configuring NetRadiant-custom")
		if err := configureRadiant(plan); err != nil {
			return fmt.Errorf("configuring NetRadiant-custom: %w", err)
		}
	}
	return nil
}
