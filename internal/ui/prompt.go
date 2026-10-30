// Package ui holds the interactive prompts and the console output.
package ui

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ErrAborted is returned when the user cancels a prompt with Ctrl+C or Esc.
var ErrAborted = errors.New("cancelled")

// Item is one choice in a list prompt. Disabled items are shown dimmed
// with their Note and cannot be picked.
type Item struct {
	Label    string
	Disabled bool
	Note     string
}

// Colors; Init clears them when the terminal cannot show them.
var (
	bold  = "\x1b[1m"
	dim   = "\x1b[2m"
	cyan  = "\x1b[36m"
	green = "\x1b[32m"
	red   = "\x1b[31m"
	reset = "\x1b[0m"
)

func question(title string) string { return green + "? " + reset + bold + title + reset }

func run(m tea.Model) (tea.Model, error) {
	final, err := tea.NewProgram(m).Run()
	if errors.Is(err, tea.ErrInterrupted) || errors.Is(err, tea.ErrProgramKilled) {
		return nil, ErrAborted
	}
	return final, err
}

// --- single and multiple choice ---

type listModel struct {
	title   string
	items   []Item
	multi   bool
	cursor  int // -1 when nothing can be picked
	checked []bool
	done    bool
	aborted bool
}

func newList(title string, items []Item, multi bool) *listModel {
	m := &listModel{title: title, items: items, multi: multi, cursor: -1, checked: make([]bool, len(items))}
	for i, it := range items {
		if !it.Disabled {
			m.cursor = i
			break
		}
	}
	return m
}

func (m *listModel) Init() tea.Cmd { return nil }

func (m *listModel) move(step int) {
	if m.cursor < 0 {
		return
	}
	for i := m.cursor + step; i >= 0 && i < len(m.items); i += step {
		if !m.items[i].Disabled {
			m.cursor = i
			return
		}
	}
}

func (m *listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "ctrl+c", "esc":
		m.aborted = true
		return m, tea.Quit
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case " ":
		if m.multi && m.cursor >= 0 {
			m.checked[m.cursor] = !m.checked[m.cursor]
		}
	case "enter":
		if !m.multi && m.cursor < 0 {
			return m, nil // nothing selectable; single choice needs an answer
		}
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m *listModel) View() string {
	var b strings.Builder
	if m.done {
		b.WriteString(question(m.title) + " " + cyan + m.answer() + reset + "\n")
		return b.String()
	}
	if m.aborted {
		return question(m.title) + "\n"
	}

	b.WriteString(question(m.title) + "\n")
	for i, it := range m.items {
		pointer := "  "
		if i == m.cursor {
			pointer = cyan + "> " + reset
		}
		box := ""
		if m.multi {
			switch {
			case it.Disabled:
				box = "[-] "
			case m.checked[i]:
				box = "[x] "
			default:
				box = "[ ] "
			}
		}
		switch {
		case it.Disabled:
			b.WriteString(pointer + dim + box + it.Label)
			if it.Note != "" {
				b.WriteString("  - " + it.Note)
			}
			b.WriteString(reset + "\n")
		case i == m.cursor:
			b.WriteString(pointer + cyan + box + it.Label + reset + "\n")
		default:
			b.WriteString(pointer + box + it.Label + "\n")
		}
	}
	switch {
	case m.multi && m.cursor < 0:
		b.WriteString(dim + "  (nothing to select here, press Enter to continue)" + reset + "\n")
	case m.multi:
		b.WriteString(dim + "  up/down: move   space: select   enter: continue" + reset + "\n")
	default:
		b.WriteString(dim + "  up/down: move   enter: choose" + reset + "\n")
	}
	return b.String()
}

func (m *listModel) answer() string {
	if !m.multi {
		return m.items[m.cursor].Label
	}
	var picked []string
	for i, it := range m.items {
		if m.checked[i] {
			picked = append(picked, it.Label)
		}
	}
	if len(picked) == 0 {
		return "none"
	}
	return strings.Join(picked, ", ")
}

// Select asks for one of items and returns its index. At least one item
// must be enabled.
func Select(title string, items []Item) (int, error) {
	final, err := run(newList(title, items, false))
	if err != nil {
		return 0, err
	}
	m := final.(*listModel)
	if m.aborted || !m.done {
		return 0, ErrAborted
	}
	return m.cursor, nil
}

// MultiSelect asks for any number of items and returns which were picked.
func MultiSelect(title string, items []Item) ([]bool, error) {
	final, err := run(newList(title, items, true))
	if err != nil {
		return nil, err
	}
	m := final.(*listModel)
	if m.aborted || !m.done {
		return nil, ErrAborted
	}
	return m.checked, nil
}

// --- text input ---

type inputModel struct {
	title    string
	def      string
	value    []rune
	validate func(string) (string, error)
	complete func(string) (string, []string) // Tab completion; nil for none
	options  []string                        // completions shown after an ambiguous Tab
	errMsg   string
	result   string
	done     bool
	aborted  bool
}

func (m *inputModel) Init() tea.Cmd { return nil }

func (m *inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		m.aborted = true
		return m, tea.Quit
	case tea.KeyEnter:
		value := strings.TrimSpace(string(m.value))
		if value == "" {
			value = m.def
		}
		result, err := m.validate(value)
		if err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		m.result, m.done = result, true
		return m, tea.Quit
	case tea.KeyTab:
		if m.complete == nil {
			return m, nil
		}
		value := string(m.value)
		if strings.TrimSpace(value) == "" {
			value = m.def // start from the default, as if it had been typed
		}
		completed, options := m.complete(value)
		m.value, m.options, m.errMsg = []rune(completed), options, ""
		return m, nil
	case tea.KeyBackspace, tea.KeyCtrlH:
		if len(m.value) > 0 {
			m.value = m.value[:len(m.value)-1]
		}
	case tea.KeyCtrlU:
		m.value = nil
	case tea.KeySpace:
		m.value = append(m.value, ' ')
	case tea.KeyRunes:
		m.value = append(m.value, key.Runes...)
	default:
		return m, nil
	}
	m.errMsg, m.options = "", nil
	return m, nil
}

func (m *inputModel) View() string {
	if m.done {
		return question(m.title) + " " + cyan + m.result + reset + "\n"
	}
	if m.aborted {
		return question(m.title) + "\n"
	}
	var b strings.Builder
	b.WriteString(question(m.title))
	if m.def != "" {
		b.WriteString(" " + dim + "[" + m.def + "]" + reset)
	}
	b.WriteString("\n  > " + string(m.value) + "_\n")
	switch {
	case m.errMsg != "":
		b.WriteString("  " + red + m.errMsg + reset + "\n")
	case len(m.options) > 0:
		const shown = 12
		list := m.options
		more := ""
		if len(list) > shown {
			list, more = list[:shown], fmt.Sprintf("  (+%d more)", len(m.options)-shown)
		}
		b.WriteString(dim + "  " + strings.Join(list, "   ") + more + reset + "\n")
	case m.def != "":
		b.WriteString(dim + "  Enter: use the folder in brackets   Tab: complete a folder name" + reset + "\n")
	case m.complete != nil:
		b.WriteString(dim + "  Tab: complete a folder name" + reset + "\n")
	}
	return b.String()
}

// Input asks for a line of text. An empty answer means def. validate
// checks the answer and may return a cleaned-up version of it; while it
// returns an error, the prompt stays open and shows the error.
func Input(title, def string, validate func(string) (string, error)) (string, error) {
	return input(&inputModel{title: title, def: def, validate: validate})
}

// InputFolder is Input for a folder path, with Tab completing folder names.
func InputFolder(title, def string, validate func(string) (string, error)) (string, error) {
	return input(&inputModel{title: title, def: def, validate: validate, complete: completeFolder})
}

func input(model *inputModel) (string, error) {
	final, err := run(model)
	if err != nil {
		return "", err
	}
	m := final.(*inputModel)
	if m.aborted || !m.done {
		return "", ErrAborted
	}
	return m.result, nil
}

// --- yes/no ---

type confirmModel struct {
	title   string
	def     bool
	answer  bool
	done    bool
	aborted bool
}

func (m *confirmModel) Init() tea.Cmd { return nil }

func (m *confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch strings.ToLower(key.String()) {
	case "ctrl+c", "esc":
		m.aborted = true
		return m, tea.Quit
	case "y":
		m.answer, m.done = true, true
		return m, tea.Quit
	case "n":
		m.answer, m.done = false, true
		return m, tea.Quit
	case "enter":
		m.answer, m.done = m.def, true
		return m, tea.Quit
	}
	return m, nil
}

func (m *confirmModel) View() string {
	if m.done {
		answer := "no"
		if m.answer {
			answer = "yes"
		}
		return question(m.title) + " " + cyan + answer + reset + "\n"
	}
	if m.aborted {
		return question(m.title) + "\n"
	}
	hint := "(y/N)"
	if m.def {
		hint = "(Y/n)"
	}
	return question(m.title) + " " + dim + hint + reset + "\n"
}

// Confirm asks a yes/no question; Enter picks def.
func Confirm(title string, def bool) (bool, error) {
	final, err := run(&confirmModel{title: title, def: def})
	if err != nil {
		return false, err
	}
	m := final.(*confirmModel)
	if m.aborted || !m.done {
		return false, ErrAborted
	}
	return m.answer, nil
}
