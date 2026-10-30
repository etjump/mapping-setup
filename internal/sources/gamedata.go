package sources

import (
	"path/filepath"

	"github.com/etjump/mapping-setup/internal/fetch"
)

// The original game data never changes, so its checksums are fixed.
var (
	paks = []fetch.File{
		{Name: "pak0.pk3", Size: 228138631, SHA256: "712966b20e06523fe81419516500e499c86b2b4fec823856ddbd333fcb3d26e5"},
		{Name: "pak1.pk3", Size: 51616, SHA256: "5610fd749024405b4425a7ce6397e58187b941d22092ef11d4844b427df53e5d"},
		{Name: "pak2.pk3", Size: 89910, SHA256: "a48ab749a1a12ab4d9137286b1f23d642c29da59845b2bafc8f64e052cf06f3e"},
	}
	// mp_bin.pk3 holds the original 32-bit menus and game code for etmain.
	mpBin = fetch.File{Name: "mp_bin.pk3", Size: 1638102, SHA256: "cf0a7ce662421c766f93cc196841849eb66905b047d209dd5f3ed0b1396cd42e"}
)

const mirror = "https://mirror.etlegacy.com/etmain/"

// GameData resolves the original game files: pak0-2.pk3, plus mp_bin.pk3
// for 32-bit engines.
func GameData(t Target) *Component {
	files := append([]fetch.File(nil), paks...)
	if t.Is32Bit() {
		files = append(files, mpBin)
	}
	for i := range files {
		files[i].URL = mirror + files[i].Name
	}

	return &Component{
		ID:      "gamedata",
		Name:    "Game data (pak0-2.pk3)",
		Version: "2.60b",
		Files:   files,
		Install: func(paths []string) error {
			for i, p := range paths {
				if err := CopyFile(p, filepath.Join(t.etmain(), files[i].Name)); err != nil {
					return err
				}
			}
			if looseClientModules(t) && t.Is32Bit() {
				return extractClientModules(filepath.Join(t.etmain(), mpBin.Name), t.etmain())
			}
			return nil
		},
	}
}
