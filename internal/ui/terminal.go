package ui

import (
	"bufio"
	"os"

	"github.com/mattn/go-isatty"
)

// Interactive reports whether the installer runs in a terminal a person can
// answer prompts in.
func Interactive() bool {
	return isTerminal(os.Stdin) && isTerminal(os.Stdout)
}

func isTerminal(f *os.File) bool {
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// Init prepares the terminal for colored output, or turns colors off when
// that is not possible.
func Init() {
	if !isTerminal(os.Stdout) || !enableVirtualTerminal() {
		bold, dim, cyan, green, red, reset = "", "", "", "", "", ""
	}
}

// WaitForEnter keeps the window open until the user presses Enter. When the
// installer is started by double-clicking, the window would otherwise
// close before the final messages can be read.
func WaitForEnter() {
	if !Interactive() {
		return
	}
	os.Stdout.WriteString("\nPress Enter to exit.")
	bufio.NewReader(os.Stdin).ReadString('\n')
}
