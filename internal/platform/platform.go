// Package platform detects the host system and decides which installer
// options are available on it.
package platform

import (
	"errors"
	"runtime"
)

// OS is a Go GOOS value for the platforms the installer knows about.
type OS string

const (
	Windows OS = "windows"
	Linux   OS = "linux"
	MacOS   OS = "darwin"
)

// Platform is the system the installer is running on.
type Platform struct {
	OS   OS
	Arch string // Go GOARCH value: amd64, arm64, ...
}

// Detect returns the platform this binary was built for, which is the one
// it runs on.
func Detect() Platform {
	return Platform{OS: OS(runtime.GOOS), Arch: runtime.GOARCH}
}

// Name is the OS name shown to the user, e.g. in "not available on macOS".
func (p Platform) Name() string {
	switch p.OS {
	case Windows:
		return "Windows"
	case Linux:
		return "Linux"
	case MacOS:
		return "macOS"
	default:
		return string(p.OS)
	}
}

// String is the full description shown in the startup banner.
func (p Platform) String() string {
	switch {
	case p.OS == MacOS && p.Arch == "arm64":
		return "macOS (Apple Silicon)"
	case p.OS == MacOS:
		return "macOS (Intel)"
	case p.Arch == "amd64":
		return p.Name() + " x64"
	default:
		return p.Name() + " " + p.Arch
	}
}

// CheckSupported reports why the whole setup cannot work on this platform,
// or nil if it can.
func (p Platform) CheckSupported() error {
	switch {
	case p.OS == MacOS:
		return nil
	case (p.OS == Windows || p.OS == Linux) && p.Arch == "amd64":
		return nil
	case p.OS == Linux && p.Arch == "arm64":
		return errors.New("Linux on ARM is not supported: ETJump and NetRadiant-custom have no ARM builds")
	default:
		return errors.New(p.String() + " is not supported")
	}
}
