// Package extract unpacks zip, tar.gz and 7z archives safely: entries can
// never be written outside the destination folder.
package extract

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/bodgit/sevenzip"
)

// Rule maps an archive entry (slash-separated path) to its destination
// relative to the target folder, or reports false to skip the entry.
type Rule func(name string) (dest string, keep bool)

// StripComponents drops the first n path elements, like tar's
// --strip-components. Entries with fewer elements are skipped.
func StripComponents(n int) Rule {
	return func(name string) (string, bool) {
		parts := strings.Split(strings.Trim(name, "/"), "/")
		if len(parts) <= n {
			return "", false
		}
		return strings.Join(parts[n:], "/"), true
	}
}

// All keeps every entry where it is.
func All() Rule {
	return func(name string) (string, bool) { return name, true }
}

// Only keeps the listed entries (matched after stripping n components) and
// places them at the given destinations.
func Only(n int, files map[string]string) Rule {
	strip := StripComponents(n)
	return func(name string) (string, bool) {
		rel, ok := strip(name)
		if !ok {
			return "", false
		}
		dest, ok := files[rel]
		return dest, ok
	}
}

// Archive unpacks archivePath into dir according to rule and returns the
// destination paths it wrote, relative to dir.
func Archive(archivePath, dir string, rule Rule) ([]string, error) {
	lower := strings.ToLower(archivePath)
	var written []string
	var err error
	switch {
	case strings.HasSuffix(lower, ".zip"), strings.HasSuffix(lower, ".pk3"):
		written, err = unzip(archivePath, dir, rule)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		written, err = untar(archivePath, dir, rule)
	case strings.HasSuffix(lower, ".7z"):
		written, err = un7z(archivePath, dir, rule)
	default:
		return nil, fmt.Errorf("%s: unsupported archive type", filepath.Base(archivePath))
	}
	if err != nil {
		return written, fmt.Errorf("extracting %s: %w", filepath.Base(archivePath), err)
	}
	return written, nil
}

func unzip(archivePath, dir string, rule Rule) ([]string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var written []string
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if f.Mode()&fs.ModeSymlink != 0 {
			continue
		}
		dest, keep := rule(f.Name)
		if !keep {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return written, err
		}
		err = writeFile(dir, dest, f.Mode(), rc)
		rc.Close()
		if err != nil {
			return written, err
		}
		written = append(written, dest)
	}
	return written, nil
}

func untar(archivePath, dir string, rule Rule) ([]string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	var written []string
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return written, nil
		}
		if err != nil {
			return written, err
		}
		dest, keep := rule(h.Name)
		if !keep {
			continue
		}
		switch h.Typeflag {
		case tar.TypeReg:
			if err := writeFile(dir, dest, h.FileInfo().Mode(), tr); err != nil {
				return written, err
			}
		case tar.TypeSymlink:
			if err := writeSymlink(dir, dest, h.Linkname); err != nil {
				return written, err
			}
		default:
			continue // directories are created on demand; nothing else is needed
		}
		written = append(written, dest)
	}
}

func un7z(archivePath, dir string, rule Rule) ([]string, error) {
	r, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var written []string
	for _, f := range r.File {
		info := f.FileInfo()
		if info.IsDir() || f.Mode()&fs.ModeSymlink != 0 {
			continue
		}
		dest, keep := rule(f.Name)
		if !keep {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return written, err
		}
		err = writeFile(dir, dest, f.Mode(), rc)
		rc.Close()
		if err != nil {
			return written, err
		}
		written = append(written, dest)
	}
	return written, nil
}

// safeJoin resolves rel inside dir, refusing anything that would escape it.
func safeJoin(dir, rel string) (string, error) {
	rel = strings.ReplaceAll(rel, "\\", "/")
	clean := path.Clean("/" + rel)
	if clean == "/" || strings.Contains(rel, "\x00") {
		return "", fmt.Errorf("invalid path %q in archive", rel)
	}
	for _, part := range strings.Split(rel, "/") {
		if part == ".." {
			return "", fmt.Errorf("unsafe path %q in archive", rel)
		}
	}
	if filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" {
		return "", fmt.Errorf("unsafe path %q in archive", rel)
	}
	return filepath.Join(dir, filepath.FromSlash(clean[1:])), nil
}

func writeFile(dir, rel string, mode fs.FileMode, r io.Reader) error {
	target, err := safeJoin(dir, rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	// Replace rather than write through an existing file or link.
	if fi, err := os.Lstat(target); err == nil && !fi.Mode().IsRegular() {
		if err := os.Remove(target); err != nil {
			return err
		}
	}

	perm := fs.FileMode(0o644)
	if mode&0o111 != 0 {
		perm = 0o755
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(target, perm) // OpenFile keeps the mode of existing files
}

func writeSymlink(dir, rel, linkTarget string) error {
	target, err := safeJoin(dir, rel)
	if err != nil {
		return err
	}
	if filepath.IsAbs(linkTarget) {
		return fmt.Errorf("unsafe link %q in archive", rel)
	}
	resolved := filepath.Join(filepath.Dir(target), filepath.FromSlash(linkTarget))
	within, err := filepath.Rel(dir, resolved)
	if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
		return fmt.Errorf("unsafe link %q in archive", rel)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := os.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.Symlink(linkTarget, target)
}

// MarkExecutable sets the executable bits on files that archives store
// without them. It does nothing on Windows.
func MarkExecutable(dir string, rels ...string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	for _, rel := range rels {
		target, err := safeJoin(dir, rel)
		if err != nil {
			return err
		}
		if err := os.Chmod(target, 0o755); err != nil {
			return err
		}
	}
	return nil
}
