package install

import (
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"

	"github.com/etjump/mapping-setup/internal/sources"
)

// configureRadiant writes NetRadiant-custom's settings so it starts straight
// into ET with the engine path set, skipping the game selection dialog.
//
// A settings/ folder next to the Radiant program makes it keep its settings
// there ("portable mode"), so nothing outside the install folder is touched.
// Settings from an earlier run are kept, since the user may have changed them.
func configureRadiant(plan *Plan) error {
	root := plan.Choices.Root
	bin := sources.RadiantBinDir(sources.RadiantDir(root))
	settings := filepath.Join(bin, "settings", sources.RadiantSettingsVersion(bin))

	global := filepath.Join(settings, "global.pref")
	if _, err := os.Stat(global); err == nil {
		return nil
	}
	if err := writePrefs(global, map[string]string{
		"gamefile":   "et.game",
		"gamePrompt": "false",
	}); err != nil {
		return err
	}
	return writePrefs(filepath.Join(settings, "et.game", "local.pref"), map[string]string{
		"EnginePath": RadiantEnginePath(root),
	})
}

// RadiantEnginePath is the install root the way Radiant stores it: forward
// slashes and a trailing slash.
func RadiantEnginePath(root string) string {
	p := filepath.ToSlash(root)
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

// writePrefs writes a Radiant .pref file (the "qpref" XML format).
func writePrefs(file string, values map[string]string) error {
	var b bytes.Buffer
	b.WriteString("<?xml version=\"1.0\"?>\n<qpref version=\"1.0\">\n")
	for name, value := range values {
		b.WriteString("<epair name=\"")
		xml.EscapeText(&b, []byte(name))
		b.WriteString("\">")
		xml.EscapeText(&b, []byte(value))
		b.WriteString("</epair>\n")
	}
	b.WriteString("</qpref>\n")

	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, b.Bytes(), 0o644)
}
