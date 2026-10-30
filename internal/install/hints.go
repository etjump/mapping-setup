package install

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/etjump/mapping-setup/internal/platform"
)

// LinuxHints lists the system packages the installed programs need but
// the installer cannot add itself (that needs sudo), with the command for
// the user's distribution.
func LinuxHints(ch Choices, p platform.Platform, is32Bit bool) []string {
	if p.OS != platform.Linux {
		return nil
	}
	family := distroFamily()
	var hints []string

	if is32Bit && !anyExists("/lib/ld-linux.so.2", "/lib32/ld-linux.so.2", "/usr/lib32/ld-linux.so.2", "/usr/lib/ld-linux.so.2") {
		hints = append(hints, "The 32-bit game needs 32-bit system libraries, which are not installed:\n    "+pick(family,
			"sudo dpkg --add-architecture i386 && sudo apt update && sudo apt install libc6:i386 libx11-6:i386 libxext6:i386 libgl1:i386 libasound2t64:i386",
			"sudo dnf install glibc.i686 libX11.i686 libXext.i686 mesa-libGL.i686 alsa-lib.i686",
			"enable the [multilib] repository, then: sudo pacman -S lib32-glibc lib32-libx11 lib32-libxext lib32-mesa lib32-alsa-lib",
			"install your distribution's 32-bit (i386/i686) versions of glibc, libX11, libXext, OpenGL and ALSA"))
	}

	if slices.Contains(ch.Extras, "qtpack3r") && !libExists("libQt6Widgets.so.6") {
		hints = append(hints, "QtPack3r needs the Qt 6 libraries:\n    "+pick(family,
			"sudo apt install libqt6widgets6",
			"sudo dnf install qt6-qtbase-gui",
			"sudo pacman -S qt6-base",
			"install your distribution's Qt 6 base/widgets package"))
	}
	return hints
}

const (
	familyDebian = "debian"
	familyFedora = "fedora"
	familyArch   = "arch"
)

func pick(family, debian, fedora, arch, other string) string {
	switch family {
	case familyDebian:
		return debian
	case familyFedora:
		return fedora
	case familyArch:
		return arch
	default:
		return other
	}
}

// distroFamily reads /etc/os-release to tell which package manager applies.
func distroFamily() string {
	b, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	var ids []string
	for _, line := range strings.Split(string(b), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok && (key == "ID" || key == "ID_LIKE") {
			ids = append(ids, strings.Fields(strings.Trim(value, `"'`))...)
		}
	}
	for _, id := range ids {
		switch id {
		case "debian", "ubuntu":
			return familyDebian
		case "fedora", "rhel", "centos":
			return familyFedora
		case "arch":
			return familyArch
		}
	}
	return ""
}

var libDirs = []string{
	"/usr/lib/x86_64-linux-gnu", "/lib/x86_64-linux-gnu",
	"/usr/lib64", "/lib64", "/usr/lib", "/lib",
}

func libExists(name string) bool {
	for _, dir := range libDirs {
		if anyExists(filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

func anyExists(paths ...string) bool {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}
