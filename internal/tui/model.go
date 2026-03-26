package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	sections []string
	index    int
	service  *AdminService
	content  string
	status   string
	cmdMode  bool
	cmdInput string
}

type sectionLoadedMsg struct {
	section string
	content string
	err     error
}

type commandDoneMsg struct {
	status string
	err    error
}

func NewModel(service *AdminService) Model {
	return Model{
		sections: []string{
			"Dashboard",
			"Telegram Groups",
			"MAX Users",
			"Invites",
			"Routes",
			"Delivery Queue",
			"Health Checks",
			"Logs",
			"Settings",
			"Exit",
		},
		service: service,
	}
}

func (m Model) Init() tea.Cmd {
	return m.loadSectionCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.KeyMsg:
		if m.cmdMode {
			return m.updateCmdMode(v)
		}
		switch v.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.index > 0 {
				m.index--
			}
			return m, m.loadSectionCmd()
		case "down", "j":
			if m.index < len(m.sections)-1 {
				m.index++
			}
			if m.sections[m.index] == "Exit" {
				return m, tea.Quit
			}
			return m, m.loadSectionCmd()
		case "tab", "right", "l":
			m.index = (m.index + 1) % len(m.sections)
			if m.sections[m.index] == "Exit" {
				m.index = 0
			}
			return m, m.loadSectionCmd()
		case "left", "h":
			m.index--
			if m.index < 0 {
				m.index = len(m.sections) - 2
			}
			return m, m.loadSectionCmd()
		case "r":
			return m, m.loadSectionCmd()
		case ":":
			m.cmdMode = true
			m.cmdInput = ""
			m.status = ""
			return m, nil
		}
	case sectionLoadedMsg:
		if v.err != nil {
			m.status = fmt.Sprintf("error: %v", v.err)
			return m, nil
		}
		m.content = v.content
		return m, nil
	case commandDoneMsg:
		if v.err != nil {
			m.status = fmt.Sprintf("command error: %v", v.err)
		} else {
			m.status = v.status
		}
		return m, m.loadSectionCmd()
	}
	return m, nil
}

func (m Model) updateCmdMode(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.cmdMode = false
		m.cmdInput = ""
		return m, nil
	case "enter":
		cmdText := strings.TrimSpace(m.cmdInput)
		m.cmdMode = false
		m.cmdInput = ""
		if cmdText == "" {
			return m, nil
		}
		return m, func() tea.Msg {
			st, err := m.service.Exec(cmdText)
			return commandDoneMsg{status: st, err: err}
		}
	case "backspace":
		if len(m.cmdInput) > 0 {
			m.cmdInput = m.cmdInput[:len(m.cmdInput)-1]
		}
		return m, nil
	default:
		if key.Type == tea.KeyRunes {
			m.cmdInput += key.String()
		}
		return m, nil
	}
}

func (m Model) View() string {
	menu := &strings.Builder{}
	fmt.Fprintln(menu, "MAXBridge Operator TUI")
	fmt.Fprintln(menu, "====================")
	for i, s := range m.sections {
		prefix := "  "
		if i == m.index {
			prefix = "> "
		}
		fmt.Fprintf(menu, "%s%s\n", prefix, s)
	}

	right := &strings.Builder{}
	fmt.Fprintf(right, "[%s]\n\n", m.sections[m.index])
	fmt.Fprintln(right, m.content)
	fmt.Fprintln(right, "")
	fmt.Fprintln(right, "Keys: arrows/tab navigate | r refresh | : command | q quit")
	if m.cmdMode {
		fmt.Fprintf(right, "> %s", m.cmdInput)
	}
	if m.status != "" {
		fmt.Fprintf(right, "\nStatus: %s\n", m.status)
	}

	return menu.String() + "\n" + right.String()
}

func (m Model) loadSectionCmd() tea.Cmd {
	section := m.sections[m.index]
	return func() tea.Msg {
		c, err := m.service.RenderSection(section)
		return sectionLoadedMsg{section: section, content: c, err: err}
	}
}
