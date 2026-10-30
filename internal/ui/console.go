package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/etjump/mapping-setup/internal/fetch"
)

// Console prints install progress and mirrors everything into a log file.
type Console struct {
	log          *os.File
	lastProgress time.Time
	progressLen  int
}

// OpenLog starts appending to the log file at path.
func (c *Console) OpenLog(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	c.log = f
	c.Log("==== %s ====", time.Now().Format(time.RFC1123))
	return nil
}

// Close closes the log file.
func (c *Console) Close() {
	if c.log != nil {
		c.log.Close()
		c.log = nil
	}
}

// Log writes a line to the log file only.
func (c *Console) Log(format string, args ...any) {
	if c.log != nil {
		fmt.Fprintf(c.log, "%s  %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
	}
}

// Linef prints a formatted line and logs it.
func (c *Console) Linef(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(msg)
	c.Log("%s", strings.TrimSpace(msg))
}

// Heading prints a bold line.
func (c *Console) Heading(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(bold + msg + reset)
	c.Log("%s", msg)
}

// Error prints an error in red and logs it.
func (c *Console) Error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(red + "Error: " + msg + reset)
	c.Log("ERROR: %s", msg)
}

// Step announces the next install step.
func (c *Console) Step(n, total int, title string) {
	fmt.Printf("\n%s[%d/%d]%s %s\n", cyan, n, total, reset, title)
	c.Log("[%d/%d] %s", n, total, title)
}

// Info prints a detail line under the current step.
func (c *Console) Info(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println("      " + msg)
	c.Log("%s", msg)
}

// Progress shows how far the current download is.
func (c *Console) Progress(done, total int64) {
	if time.Since(c.lastProgress) < 150*time.Millisecond && done != total {
		return
	}
	c.lastProgress = time.Now()

	line := "      downloading " + fetch.FormatSize(done)
	if total > 0 {
		line += fmt.Sprintf(" / %s  (%d%%)", fetch.FormatSize(total), done*100/total)
	}
	pad := ""
	if n := c.progressLen - len(line); n > 0 {
		pad = strings.Repeat(" ", n)
	}
	fmt.Print("\r" + line + pad)
	c.progressLen = len(line)
}

// ProgressDone ends the progress line.
func (c *Console) ProgressDone() {
	if c.progressLen > 0 {
		fmt.Println()
		c.progressLen = 0
	}
}
