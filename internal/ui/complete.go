package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// completeFolder completes the last element of a typed path to the
// subfolders that start with it, like a shell's Tab completion. With one
// match it fills in the whole name; with several it fills in what they
// share and returns them all so they can be listed.
func completeFolder(value string) (string, []string) {
	typed := value
	if typed == "~" {
		typed += string(filepath.Separator)
	}
	if strings.HasPrefix(typed, "~/") || strings.HasPrefix(typed, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			typed = home + typed[1:]
		}
	}
	// "D:" alone means the drive's root here, not its current directory.
	if runtime.GOOS == "windows" && len(typed) == 2 && typed[1] == ':' {
		typed += `\`
	}

	dir, prefix := filepath.Split(typed)
	listDir := dir
	if listDir == "" {
		listDir = "."
	}
	entries, err := os.ReadDir(listDir)
	if err != nil {
		return value, nil
	}

	var matches []string
	for _, e := range entries {
		name := e.Name()
		if !isDir(listDir, e) || !hasPrefixFold(name, prefix) {
			continue
		}
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue // hidden folders only when asked for
		}
		if strings.HasPrefix(name, "$") || name == "System Volume Information" {
			continue // Windows system folders
		}
		matches = append(matches, name)
	}
	sort.Slice(matches, func(i, j int) bool { return strings.ToLower(matches[i]) < strings.ToLower(matches[j]) })

	switch len(matches) {
	case 0:
		return value, nil
	case 1:
		return dir + matches[0] + string(filepath.Separator), nil
	default:
		return dir + commonPrefixFold(matches, prefix), matches
	}
}

func isDir(parent string, e os.DirEntry) bool {
	if e.IsDir() {
		return true
	}
	if e.Type()&os.ModeSymlink != 0 {
		fi, err := os.Stat(filepath.Join(parent, e.Name()))
		return err == nil && fi.IsDir()
	}
	return false
}

// Windows and macOS file names are case-insensitive; Linux ones are not.
var caseInsensitive = runtime.GOOS != "linux"

func hasPrefixFold(s, prefix string) bool {
	if caseInsensitive {
		return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
	}
	return strings.HasPrefix(s, prefix)
}

// commonPrefixFold returns the longest start all names share, keeping what
// the user typed when the names differ only in letter case from it.
func commonPrefixFold(names []string, typed string) string {
	common := names[0]
	for _, n := range names[1:] {
		i := 0
		for i < len(common) && i < len(n) && sameChar(common[i], n[i]) {
			i++
		}
		common = common[:i]
	}
	if len(common) <= len(typed) {
		return typed
	}
	return common
}

func sameChar(a, b byte) bool {
	if caseInsensitive {
		return strings.EqualFold(string(a), string(b))
	}
	return a == b
}
