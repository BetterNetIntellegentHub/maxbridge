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

	next, cmd := applyKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected section load command on Enter for non-Exit section")
	}
	if next.mode != modeRows {
		t.Fatalf("expected modeRows after Enter on section, got %v", next.mode)
	}
	if next.currentSection != "Dashboard" {
		t.Fatalf("expected current section Dashboard, got %q", next.currentSection)
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

func TestEnterOnRowOpensActionMenu(t *testing.T) {
	m := NewModel(nil)
	m.mode = modeRows
	m.currentSection = "MAX Users"
	m.rows = []listEntry{
		{
			kind:  rowRecord,
			title: "user",
			row:   map[string]any{"max_user_id": int64(42)},
		},
	}

	next, cmd := applyKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("expected no async command when opening actions")
	}
	if next.mode != modeActions {
		t.Fatalf("expected modeActions, got %v", next.mode)
	}
	if len(next.actions) == 0 {
		t.Fatalf("expected non-empty actions for MAX user row")
	}
}
