package ui

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableVirtualTerminal turns on ANSI escape handling in the Windows
// console, which older consoles leave off by default.
func enableVirtualTerminal() bool {
	h := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return false
	}
	return windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}
