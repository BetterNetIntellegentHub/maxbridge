package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func applyKey(m Model, key tea.KeyMsg) (Model, tea.Cmd) {
	next, cmd := m.Update(key)
	return next.(Model), cmd
}

func TestNavigationDoesNotQuitOnExitSelection(t *testing.T) {
	m := NewModel(nil)

	for range len(m.sections) - 1 {
		var cmd tea.Cmd
		m, cmd = applyKey(m, tea.KeyMsg{Type: tea.KeyDown})
		if cmd != nil {
			t.Fatalf("expected no command while navigating, got command at index=%d", m.index)
		}
	}

	if got, want := m.sections[m.index], "Exit"; got != want {
		t.Fatalf("expected selected section %q, got %q", want, got)
	}
}

func TestEnterOnExitQuits(t *testing.T) {
	m := NewModel(nil)
	m.index = len(m.sections) - 1

	_, cmd := applyKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected quit command on Enter at Exit section")
	}
}

func TestEnterOnRegularSectionTriggersSelection(t *testing.T) {
	m := NewModel(nil)
	m.index = 0

	_, cmd := applyKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected section load command on Enter for non-Exit section")
	}
}

func TestTabCanMoveToExit(t *testing.T) {
	m := NewModel(nil)
	m.index = len(m.sections) - 2

	var cmd tea.Cmd
	m, cmd = applyKey(m, tea.KeyMsg{Type: tea.KeyTab})
	if cmd != nil {
		t.Fatalf("expected no command while navigating by tab")
	}

	if got, want := m.sections[m.index], "Exit"; got != want {
		t.Fatalf("expected selected section %q, got %q", want, got)
	}
}
