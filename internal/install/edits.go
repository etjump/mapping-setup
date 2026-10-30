package install

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/etjump/mapping-setup/internal/sources"
)

// ensureShaderlist makes sure scripts/shaderlist.txt exists and lists the
// given shader files. NetRadiant only creates shaderlist.txt on its first
// start, so it is created here from the SDK's default list.
func ensureShaderlist(etmain string, entries ...string) error {
	scripts := filepath.Join(etmain, "scripts")
	list := filepath.Join(scripts, "shaderlist.txt")

	content, err := os.ReadFile(list)
	if errors.Is(err, fs.ErrNotExist) {
		content, err = os.ReadFile(filepath.Join(scripts, "default_shaderlist.txt"))
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	text := string(content)
	eol := "\n"
	if strings.Contains(text, "\r\n") {
		eol = "\r\n"
	}
	present := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		present[strings.ToLower(strings.TrimSpace(line))] = true
	}

	for _, e := range entries {
		if present[strings.ToLower(e)] {
			continue
		}
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += eol
		}
		text += e + eol
	}

	if err := os.MkdirAll(scripts, 0o755); err != nil {
		return err
	}
	return os.WriteFile(list, []byte(text), 0o644)
}

const (
	chateauShader      = "textures/chateau/walltest_c07a_vertex"
	chateauEditorImage = "qer_editorImage textures/chateau/walltest_c07a_vertex"
)

var editorImageRe = regexp.MustCompile(`(?i)^(\s*)qer_editorimage\s+(\S+)`)

// fixChateauShader points the walltest_c07a_vertex shader at the editor
// image shipped with the ET extra assets, as their README asks. The stock
// shader references an image that does not exist.
func fixChateauShader(etmain string) error {
	file := filepath.Join(etmain, "scripts", "chateau.shader")
	content, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	inShader, depth, changed := false, 0, false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inShader {
			inShader = depth == 0 && strings.EqualFold(trimmed, chateauShader)
			depth += strings.Count(line, "{") - strings.Count(line, "}")
			continue
		}
		// Only the shader's own top-level lines, not its stages.
		if depth == 1 {
			if m := editorImageRe.FindStringSubmatch(line); m != nil {
				if !strings.EqualFold(m[2], chateauShader) {
					cr := ""
					if strings.HasSuffix(line, "\r") {
						cr = "\r"
					}
					lines[i] = m[1] + chateauEditorImage + cr
					changed = true
				}
				break
			}
		}
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		if depth == 0 && strings.Contains(line, "}") {
			break // end of the shader without an editor image line
		}
	}

	if !changed {
		return nil
	}
	return os.WriteFile(file, []byte(strings.Join(lines, "\n")), 0o644)
}

// installEntityDefs copies ETJump's et_entities.ent into the editor,
// keeping the editor's original file as et_entities.ent.orig.
func installEntityDefs(etmain, editorDir string) error {
	dir := sources.EntityDefDir(editorDir)
	if dir == "" {
		return errors.New("could not find gamepacks/et.game/etmain in " + editorDir)
	}
	dst := filepath.Join(dir, "et_entities.ent")
	backup := dst + ".orig"
	if _, err := os.Stat(backup); errors.Is(err, fs.ErrNotExist) {
		if _, err := os.Stat(dst); err == nil {
			if err := sources.CopyFile(dst, backup); err != nil {
				return err
			}
		}
	}
	return sources.CopyFile(filepath.Join(etmain, "scripts", "et_entities.ent"), dst)
}
