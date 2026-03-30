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

func TestRowsListEndsWithBack(t *testing.T) {
	m := NewModel(nil)
	rows := []map[string]any{{"max_user_id": int64(1)}}

	entries := m.buildEntries("MAX Users", rows)
	if len(entries) == 0 {
		t.Fatalf("expected entries")
	}
	if entries[len(entries)-1].kind != rowBack {
		t.Fatalf("expected last entry to be Back")
	}
}

func TestBuildEntriesIncludeSectionActionAndBack(t *testing.T) {
	m := NewModel(nil)
	entries := m.buildEntries("Invites", nil)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (section action + back), got %d", len(entries))
	}
	if entries[0].kind != rowSectionAction || entries[0].action.id != "invite_create" {
		t.Fatalf("expected invite_create section action as first entry")
	}
	if entries[1].kind != rowBack {
		t.Fatalf("expected back as last entry")
	}
}

func TestEnterOnRowsBackReturnsToSections(t *testing.T) {
	m := NewModel(nil)
	m.mode = modeRows
	m.currentSection = "MAX Users"
	m.rows = []listEntry{{kind: rowBack, title: "Back"}}
	m.rowIndex = 0

	next, cmd := applyKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("expected no async command on back")
	}
	if next.mode != modeSections {
		t.Fatalf("expected modeSections after back, got %v", next.mode)
	}
}

func TestActionsListEndsWithBack(t *testing.T) {
	m := NewModel(nil)
	actions := m.buildRowActions("MAX Users", listEntry{
		kind: rowRecord,
		row:  map[string]any{"max_user_id": int64(1)},
	})
	withBack := withBackAction(actions)
	if len(withBack) == 0 {
		t.Fatalf("expected actions")
	}
	if withBack[len(withBack)-1].id != backActionID {
		t.Fatalf("expected last action to be Back")
	}
}

func TestMaxUsersHasRenameAction(t *testing.T) {
	m := NewModel(nil)
	actions := m.buildRowActions("MAX Users", listEntry{
		kind: rowRecord,
		row:  map[string]any{"max_user_id": int64(1)},
	})
	found := false
	for _, a := range actions {
		if a.id == "user_rename" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected user_rename action in MAX Users")
	}
}

func TestSectionActionOpensFormDirectlyFromRows(t *testing.T) {
	m := NewModel(nil)
	m.mode = modeRows
	m.currentSection = "Invites"
	m.rows = []listEntry{
		{
			kind:   rowSectionAction,
			title:  "Создать инвайт",
			action: m.buildSectionActions("Invites")[0],
		},
		{kind: rowBack, title: "Назад"},
	}

	next, cmd := applyKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("expected no async command when opening section action form")
	}
	if next.mode != modeForm {
		t.Fatalf("expected modeForm, got %v", next.mode)
	}
}

func TestSectionActionFormEscReturnsToRows(t *testing.T) {
	m := NewModel(nil)
	action := m.buildSectionActions("Invites")[0]
	m.mode = modeForm
	m.form = actionForm{
		action: action,
		fields: action.fields,
		values: []string{"Иван Петров"},
		ret:    modeRows,
	}

	next, cmd := applyKey(m, tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatalf("expected no async command on form back")
	}
	if next.mode != modeRows {
		t.Fatalf("expected return to modeRows, got %v", next.mode)
	}
}

func TestFormHasSaveThenBackButtons(t *testing.T) {
	m := NewModel(nil)
	action := menuAction{
		id:    "test_action",
		label: "Тест",
		fields: []formField{
			{key: "f1", label: "Поле 1"},
			{key: "f2", label: "Поле 2"},
		},
	}
	m.mode = modeForm
	m.form = actionForm{
		action: action,
		fields: action.fields,
		values: []string{"a", "b"},
		index:  len(action.fields),
		ret:    modeRows,
	}

	next, cmd := applyKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected action command from Save button")
	}
	if next.mode != modeRows {
		t.Fatalf("expected modeRows after save, got %v", next.mode)
	}
}

func TestRouteAddHasNoIgnoreBotsInput(t *testing.T) {
	m := NewModel(nil)
	acts := m.buildSectionActions("Routes")
	if len(acts) == 0 {
		t.Fatalf("expected route section actions")
	}
	fields := acts[0].fields
	for _, f := range fields {
		if f.key == "ignore_bots" {
			t.Fatalf("ignore_bots should not be shown in route form")
		}
	}
}

func TestRouteAddStartsPickerMode(t *testing.T) {
	m := NewModel(nil)
	m.mode = modeRows
	m.currentSection = "Routes"
	m.rows = []listEntry{
		{kind: rowSectionAction, title: "Добавить маршрут", action: menuAction{id: "route_add", label: "Добавить маршрут"}},
		{kind: rowBack, title: "Назад"},
	}

	next, cmd := applyKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected options loading command")
	}
	if next.mode != modeRouteAddPicker {
		t.Fatalf("expected modeRouteAddPicker, got %v", next.mode)
	}
}

func TestFormatMaxUserName_FullName(t *testing.T) {
	row := map[string]any{
		"max_user_id": int64(42),
		"full_name":   "Иван Петров",
	}
	got := formatMaxUserName(row)
	if got != "Иван Петров" {
		t.Fatalf("unexpected display name: %q", got)
	}
}

func TestFormatMaxUserName_FallbackToID(t *testing.T) {
	row := map[string]any{
		"max_user_id": int64(42),
	}
	got := formatMaxUserName(row)
	if got != "Пользователь MAX 42" {
		t.Fatalf("unexpected fallback display name: %q", got)
	}
}

func TestFormatRowTitle_UsesDisplayNameForRoutesAndQueue(t *testing.T) {
	routeRow := map[string]any{
		"group_title": "Группа A",
		"chat_id":     int64(-1001),
		"max_user_id": int64(77),
		"full_name":   "Иван Петров",
	}
	title := formatRowTitle("Routes", routeRow)
	if title != "Группа A -> Иван Петров" {
		t.Fatalf("unexpected route title: %q", title)
	}

	queueRow := map[string]any{
		"status":      "retry",
		"max_user_id": int64(77),
		"full_name":   "Иван Петров",
	}
	queueTitle := formatRowTitle("Delivery Queue", queueRow)
	if queueTitle != "Статус: retry | Иван Петров" {
		t.Fatalf("unexpected queue title: %q", queueTitle)
	}
}

func TestFormatRowTitle_InvitesIncludesNameWhenPresent(t *testing.T) {
	row := map[string]any{
		"raw_code":      "MB-ABC123",
		"max_full_name": "Иван Петров",
	}

	got := formatRowTitle("Invites", row)
	if got != "Код: MB-ABC123 | Иван Петров" {
		t.Fatalf("unexpected invite title: %q", got)
	}
}

func TestFormatRowDetail_InvitesIncludesNameWhenPresent(t *testing.T) {
	row := map[string]any{
		"id":            int64(7),
		"scope":         "entity:general",
		"expires_at":    "2026-03-30T10:00:00Z",
		"max_full_name": "Иван Петров",
	}

	got := formatRowDetail("Invites", row)
	if got != "id=7 имя=Иван Петров scope=entity:general до=2026-03-30T10:00:00Z" {
		t.Fatalf("unexpected invite detail: %q", got)
	}
}

func TestFormatRowTitleAndDetail_InvitesFallbackWithoutName(t *testing.T) {
	row := map[string]any{
		"id":         int64(8),
		"raw_code":   "MB-XYZ789",
		"scope":      "entity:general",
		"expires_at": "2026-03-30T11:00:00Z",
	}

	title := formatRowTitle("Invites", row)
	if title != "Код: MB-XYZ789" {
		t.Fatalf("unexpected invite title fallback: %q", title)
	}

	detail := formatRowDetail("Invites", row)
	if detail != "id=8 scope=entity:general до=2026-03-30T11:00:00Z" {
		t.Fatalf("unexpected invite detail fallback: %q", detail)
	}
}
