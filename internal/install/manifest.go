package install

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// ManifestName is the file in the install folder that records what this
// installer put there.
const ManifestName = "etmapping-setup.json"

const (
	statusInstalling = "installing"
	statusComplete   = "complete"
)

// Manifest is the record of an install, used to recognise and update it.
type Manifest struct {
	SetupVersion string               `json:"setupVersion"`
	Status       string               `json:"status"`
	Updated      time.Time            `json:"updated"`
	Choices      Choices              `json:"choices"`
	Components   []InstalledComponent `json:"components"`
}

// InstalledComponent records one installed part and its version.
type InstalledComponent struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Complete reports whether the recorded install finished.
func (m *Manifest) Complete() bool { return m.Status == statusComplete }

func newManifest(version string, plan *Plan) *Manifest {
	m := &Manifest{SetupVersion: version, Choices: plan.Choices}
	for _, comp := range plan.Components {
		m.Components = append(m.Components, InstalledComponent{ID: comp.ID, Name: comp.Name, Version: comp.Version})
	}
	return m
}

// ReadManifest loads the manifest from root. It returns nil, nil if there
// is none.
func ReadManifest(root string) (*Manifest, error) {
	b, err := os.ReadFile(filepath.Join(root, ManifestName))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *Manifest) write(root string) error {
	m.Updated = time.Now().UTC()
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, ManifestName), append(b, '\n'), 0o644)
}
