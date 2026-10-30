package install

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/etjump/mapping-setup/internal/fetch"
	"github.com/etjump/mapping-setup/internal/platform"
	"github.com/etjump/mapping-setup/internal/sources"
)

// DefaultRoot is the default install folder: ETMapping in the user's home.
func DefaultRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "ETMapping"
	}
	return filepath.Join(home, "ETMapping")
}

// CheckNotRoot refuses to run as root on Linux and macOS, where every
// installed file would end up owned by root.
func CheckNotRoot(p platform.Platform) error {
	if p.OS != platform.Windows && os.Geteuid() == 0 {
		return errors.New("please run the installer as your normal user, not as root or with sudo")
	}
	return nil
}

// CheckConnectivity reports the hosts the installer cannot reach.
func CheckConnectivity(ctx context.Context, c *fetch.Client) []string {
	hosts := []string{
		"https://github.com",
		"https://www.etlegacy.com",
		"https://mirror.etlegacy.com",
		"https://aciz.etjump.com",
	}
	var mu sync.Mutex
	var failed []string
	var wg sync.WaitGroup
	for _, h := range hosts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.Reachable(ctx, h); err != nil {
				mu.Lock()
				failed = append(failed, strings.TrimPrefix(h, "https://"))
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return failed
}

// RootCheck is the outcome of validating an install folder.
type RootCheck struct {
	Path     string    // cleaned absolute path
	Existing *Manifest // set when the folder holds an install made by this tool
}

// CheckRoot validates the folder the user wants to install into.
func CheckRoot(input string, p platform.Platform) (*RootCheck, error) {
	path, err := absPath(input)
	if err != nil {
		return nil, err
	}
	if !isASCII(path) {
		return nil, errors.New("the path contains non-English characters, which NetRadiant and the map compiler cannot handle; choose another folder")
	}
	if p.OS == platform.Windows {
		for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "ProgramW6432"} {
			if pf := os.Getenv(env); pf != "" && isWithin(path, pf) {
				return nil, errors.New("Program Files is write-protected and NetRadiant needs to write next to itself; choose another folder")
			}
		}
	}

	fi, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// Fine: it will be created. Make sure that is possible.
	case err != nil:
		return nil, err
	case !fi.IsDir():
		return nil, errors.New("that path is a file, not a folder")
	default:
		m, err := ReadManifest(path)
		if err != nil {
			return nil, fmt.Errorf("reading the existing %s: %w", ManifestName, err)
		}
		if m != nil {
			return &RootCheck{Path: path, Existing: m}, nil
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		if len(entries) > 0 {
			return nil, errors.New("the folder is not empty and was not created by this installer; choose an empty or new folder")
		}
	}

	if err := checkWritable(path); err != nil {
		return nil, err
	}
	return &RootCheck{Path: path}, nil
}

// CheckEditor validates an existing editor folder and returns its cleaned
// absolute path.
func CheckEditor(input string) (string, error) {
	path, err := absPath(input)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(path)
	if err != nil || !fi.IsDir() {
		return "", errors.New("that folder does not exist")
	}
	if sources.EntityDefDir(path) == "" {
		return "", errors.New("no ET game pack found there (expected gamepacks/et.game/etmain); pick the editor's main folder")
	}
	return path, nil
}

// CheckDiskSpace makes sure the drive holding root has room for the
// downloads plus everything unpacked from them.
func CheckDiskSpace(root string, downloadSize int64) error {
	dir := nearestExisting(root)
	free, err := freeSpace(dir)
	if err != nil {
		return nil // unknown; let the install find out
	}
	need := uint64(downloadSize) * 5 / 2 // cache + unpacked files
	if free < need {
		return fmt.Errorf("not enough disk space: about %s needed, %s free", fetch.FormatSize(int64(need)), fetch.FormatSize(int64(free)))
	}
	return nil
}

func absPath(input string) (string, error) {
	input = strings.TrimSpace(input)
	input = strings.Trim(input, `"'`) // pasted paths often come quoted
	if input == "" {
		return "", errors.New("please enter a folder")
	}
	if input == "~" || strings.HasPrefix(input, "~/") || strings.HasPrefix(input, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		input = filepath.Join(home, input[1:])
	}
	return filepath.Abs(input)
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

func isWithin(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func nearestExisting(path string) string {
	for {
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			return path
		}
		path = parent
	}
}

func checkWritable(path string) error {
	dir := nearestExisting(path)
	f, err := os.CreateTemp(dir, ".etmapping-write-test-*")
	if err != nil {
		return fmt.Errorf("cannot write to %s; choose another folder", dir)
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return nil
}
