package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type uiMode int

const (
	modeSections uiMode = iota
	modeRows
	modeActions
	modeForm
	modeConfirm
)

type rowKind int

const (
	rowRecord rowKind = iota
	rowSectionActions
)

type listEntry struct {
	kind   rowKind
	title  string
	detail string
	row    map[string]any
}

type formField struct {
	key         string
	label       string
	placeholder string
	defaultVal  string
}

type menuAction struct {
	id        string
	label     string
	dangerous bool
	fields    []formField
}

type actionForm struct {
	action menuAction
	entry  listEntry
	fields []formField
	values []string
	index  int
}

type pendingAction struct {
	action menuAction
	entry  listEntry
	values map[string]string
}

type Model struct {
	sections []string
	index    int
	service  *AdminService

	mode           uiMode
	currentSection string
	sectionContent string
	rows           []listEntry
	rowIndex       int
	actions        []menuAction
	actionIndex    int
	form           actionForm
	pending        *pendingAction

	status        string
	preview       SectionData
	previewLoaded bool
}

type sectionLoadedMsg struct {
	section string
	data    SectionData
	err     error
}

type actionDoneMsg struct {
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
		mode:    modeSections,
	}
}

func (m Model) Init() tea.Cmd {
	if len(m.sections) == 0 {
		return nil
	}
	return m.loadSectionCmd(m.sections[m.index])
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.KeyMsg:
		switch v.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
		switch m.mode {
		case modeSections:
			return m.updateSections(v)
		case modeRows:
			return m.updateRows(v)
		case modeActions:
			return m.updateActions(v)
		case modeForm:
			return m.updateForm(v)
		case modeConfirm:
			return m.updateConfirm(v)
		}
	case sectionLoadedMsg:
		if v.err != nil {
			m.status = fmt.Sprintf("error: %v", v.err)
			return m, nil
		}
		if m.mode == modeSections && v.section == m.sections[m.index] {
			m.preview = v.data
			m.previewLoaded = true
		}
		if v.section == m.currentSection {
			m.sectionContent = v.data.Content
			m.rows = m.buildEntries(v.section, v.data.Rows)
			if m.rowIndex >= len(m.rows) {
				m.rowIndex = max(0, len(m.rows)-1)
			}
		}
		return m, nil
	case actionDoneMsg:
		if v.err != nil {
			m.status = fmt.Sprintf("action error: %v", v.err)
		} else {
			m.status = v.status
		}
		if m.currentSection != "" {
			return m, m.loadSectionCmd(m.currentSection)
		}
		return m, nil
	}
	return m, nil
}

func (m Model) updateSections(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "up", "k":
		if m.index > 0 {
			m.index--
		}
		return m, nil
	case "down", "j":
		if m.index < len(m.sections)-1 {
			m.index++
		}
		return m, nil
	case "tab", "right", "l":
		m.index = (m.index + 1) % len(m.sections)
		return m, nil
	case "left", "h":
		m.index--
		if m.index < 0 {
			m.index = len(m.sections) - 1
		}
		return m, nil
	case "r":
		return m, m.loadSectionCmd(m.sections[m.index])
	case "enter":
		selected := m.sections[m.index]
		if selected == "Exit" {
			return m, tea.Quit
		}
		m.currentSection = selected
		m.sectionContent = ""
		m.rows = nil
		m.rowIndex = 0
		m.actions = nil
		m.actionIndex = 0
		m.pending = nil
		m.status = ""
		m.mode = modeRows
		return m, m.loadSectionCmd(selected)
	}
	return m, nil
}

func (m Model) updateRows(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "backspace", "left", "h":
		m.mode = modeSections
		return m, nil
	case "up", "k":
		if m.rowIndex > 0 {
			m.rowIndex--
		}
		return m, nil
	case "down", "j":
		if m.rowIndex < len(m.rows)-1 {
			m.rowIndex++
		}
		return m, nil
	case "r":
		return m, m.loadSectionCmd(m.currentSection)
	case "enter":
		if len(m.rows) == 0 {
			m.status = "no items in this section"
			return m, nil
		}
		m.actions = m.buildActions(m.currentSection, m.rows[m.rowIndex])
		if len(m.actions) == 0 {
			m.status = "no actions available"
			return m, nil
		}
		m.actionIndex = 0
		m.mode = modeActions
		return m, nil
	}
	return m, nil
}

func (m Model) updateActions(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "backspace", "left", "h":
		m.mode = modeRows
		m.actions = nil
		m.actionIndex = 0
		return m, nil
	case "up", "k":
		if m.actionIndex > 0 {
			m.actionIndex--
		}
		return m, nil
	case "down", "j":
		if m.actionIndex < len(m.actions)-1 {
			m.actionIndex++
		}
		return m, nil
	case "enter":
		if len(m.actions) == 0 {
			return m, nil
		}
		act := m.actions[m.actionIndex]
		entry := m.rows[m.rowIndex]
		if len(act.fields) > 0 {
			values := make([]string, len(act.fields))
			for i, f := range act.fields {
				values[i] = f.defaultVal
			}
			m.form = actionForm{action: act, entry: entry, fields: act.fields, values: values}
			m.mode = modeForm
			return m, nil
		}
		if act.dangerous {
			m.pending = &pendingAction{action: act, entry: entry, values: map[string]string{}}
			m.mode = modeConfirm
			return m, nil
		}
		m.mode = modeRows
		m.actions = nil
		m.actionIndex = 0
		return m, m.execActionCmd(act, entry, map[string]string{})
	}
	return m, nil
}

func (m Model) updateForm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.form = actionForm{}
		m.mode = modeActions
		return m, nil
	case "up", "k", "shift+tab", "backtab":
		if m.form.index > 0 {
			m.form.index--
		}
		return m, nil
	case "down", "j", "tab":
		if m.form.index < len(m.form.fields)-1 {
			m.form.index++
		}
		return m, nil
	case "enter":
		if m.form.index < len(m.form.fields)-1 {
			m.form.index++
			return m, nil
		}
		values := map[string]string{}
		for i, f := range m.form.fields {
			values[f.key] = strings.TrimSpace(m.form.values[i])
		}
		act := m.form.action
		entry := m.form.entry
		m.form = actionForm{}
		if act.dangerous {
			m.pending = &pendingAction{action: act, entry: entry, values: values}
			m.mode = modeConfirm
			return m, nil
		}
		m.mode = modeRows
		m.actions = nil
		m.actionIndex = 0
		return m, m.execActionCmd(act, entry, values)
	case "backspace":
		if len(m.form.values) == 0 {
			return m, nil
		}
		idx := m.form.index
		if idx < 0 || idx >= len(m.form.values) {
			return m, nil
		}
		if len(m.form.values[idx]) > 0 {
			m.form.values[idx] = m.form.values[idx][:len(m.form.values[idx])-1]
		}
		return m, nil
	default:
		if key.Type == tea.KeyRunes {
			idx := m.form.index
			if idx >= 0 && idx < len(m.form.values) {
				m.form.values[idx] += key.String()
			}
		}
		return m, nil
	}
}

func (m Model) updateConfirm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "n":
		m.pending = nil
		m.mode = modeActions
		return m, nil
	case "y", "enter":
		if m.pending == nil {
			m.mode = modeActions
			return m, nil
		}
		p := *m.pending
		m.pending = nil
		m.mode = modeRows
		m.actions = nil
		m.actionIndex = 0
		return m, m.execActionCmd(p.action, p.entry, p.values)
	}
	return m, nil
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
	switch m.mode {
	case modeSections:
		fmt.Fprintf(right, "[Sections] selected: %s\n\n", m.sections[m.index])
		if m.previewLoaded && len(m.preview.Rows) > 0 {
			fmt.Fprintln(right, renderRows(m.preview.Rows))
		} else if m.previewLoaded {
			fmt.Fprintln(right, m.preview.Content)
		}
		fmt.Fprintln(right, "")
		fmt.Fprintln(right, "Keys: arrows/tab navigate | enter open section | r preview | q quit")
	case modeRows:
		fmt.Fprintf(right, "[%s]\n\n", m.currentSection)
		if len(m.rows) == 0 {
			if strings.TrimSpace(m.sectionContent) == "" {
				fmt.Fprintln(right, "<empty>")
			} else {
				fmt.Fprintln(right, m.sectionContent)
			}
		} else {
			for i, row := range m.rows {
				prefix := "  "
				if i == m.rowIndex {
					prefix = "> "
				}
				fmt.Fprintf(right, "%s%s\n", prefix, row.title)
				if row.detail != "" {
					fmt.Fprintf(right, "    %s\n", row.detail)
				}
			}
		}
		fmt.Fprintln(right, "")
		fmt.Fprintln(right, "Keys: arrows navigate | enter actions | esc back | r refresh | q quit")
	case modeActions:
		entry := m.rows[m.rowIndex]
		fmt.Fprintf(right, "[Actions] %s\n\n", entry.title)
		for i, act := range m.actions {
			prefix := "  "
			if i == m.actionIndex {
				prefix = "> "
			}
			danger := ""
			if act.dangerous {
				danger = " [guarded]"
			}
			fmt.Fprintf(right, "%s%s%s\n", prefix, act.label, danger)
		}
		fmt.Fprintln(right, "")
		fmt.Fprintln(right, "Keys: arrows navigate | enter select | esc back | q quit")
	case modeForm:
		fmt.Fprintf(right, "[Input] %s\n\n", m.form.action.label)
		for i, f := range m.form.fields {
			prefix := "  "
			if i == m.form.index {
				prefix = "> "
			}
			value := ""
			if i < len(m.form.values) {
				value = m.form.values[i]
			}
			if value == "" {
				value = f.placeholder
			}
			fmt.Fprintf(right, "%s%s: %s\n", prefix, f.label, value)
		}
		fmt.Fprintln(right, "")
		fmt.Fprintln(right, "Keys: type input | enter next/submit | tab move | esc cancel")
	case modeConfirm:
		if m.pending != nil {
			fmt.Fprintf(right, "[Confirm] %s\n\n", m.pending.action.label)
			fmt.Fprintln(right, "This operation is potentially destructive.")
			fmt.Fprintln(right, "Press y or enter to confirm, n or esc to cancel.")
		}
	}

	if m.status != "" {
		fmt.Fprintf(right, "\nStatus: %s\n", m.status)
	}

	return menu.String() + "\n" + right.String()
}

func (m Model) loadSectionCmd(section string) tea.Cmd {
	return func() tea.Msg {
		if m.service == nil {
			return sectionLoadedMsg{section: section, data: SectionData{Content: "service not configured"}}
		}
		data, err := m.service.LoadSection(section)
		return sectionLoadedMsg{section: section, data: data, err: err}
	}
}

func (m Model) buildEntries(section string, rows []map[string]any) []listEntry {
	entries := make([]listEntry, 0, len(rows)+1)
	if hasSectionActions(section) {
		entries = append(entries, listEntry{kind: rowSectionActions, title: "Section actions", detail: "Open section-level operations"})
	}
	for _, row := range rows {
		entries = append(entries, listEntry{
			kind:   rowRecord,
			title:  formatRowTitle(section, row),
			detail: formatRowDetail(section, row),
			row:    row,
		})
	}
	return entries
}

func hasSectionActions(section string) bool {
	switch section {
	case "Telegram Groups", "Invites", "Routes", "Delivery Queue":
		return true
	default:
		return false
	}
}

func (m Model) buildActions(section string, entry listEntry) []menuAction {
	if entry.kind == rowSectionActions {
		switch section {
		case "Telegram Groups":
			return []menuAction{
				{
					id:    "group_add",
					label: "Add group",
					fields: []formField{
						{key: "chat_id", label: "Chat ID", placeholder: "-1001234567890"},
						{key: "title", label: "Title", placeholder: "Operations group"},
					},
				},
				{id: "group_probe_all", label: "Probe all groups"},
			}
		case "Invites":
			return []menuAction{
				{
					id:    "invite_create",
					label: "Create invite",
					fields: []formField{
						{key: "scope_type", label: "Scope type", placeholder: "group|route|entity", defaultVal: "group"},
						{key: "scope_id", label: "Scope ID", placeholder: "123"},
						{key: "ttl", label: "TTL", placeholder: "24h", defaultVal: "24h"},
					},
				},
			}
		case "Routes":
			return []menuAction{
				{
					id:    "route_add",
					label: "Add route",
					fields: []formField{
						{key: "chat_id", label: "Chat ID", placeholder: "-1001234567890"},
						{key: "max_user_id", label: "MAX User ID", placeholder: "10001"},
						{key: "filter_mode", label: "Filter mode", placeholder: "all|text_only|mentions_only", defaultVal: "all"},
						{key: "ignore_bots", label: "Ignore bots", placeholder: "true|false", defaultVal: "true"},
					},
				},
			}
		case "Delivery Queue":
			return []menuAction{
				{
					id:        "queue_clear_completed",
					label:     "Clear completed jobs",
					dangerous: true,
					fields: []formField{
						{key: "days", label: "Older than days", placeholder: "7", defaultVal: "7"},
					},
				},
			}
		default:
			return nil
		}
	}

	switch section {
	case "Telegram Groups":
		return []menuAction{
			{id: "group_probe", label: "Probe group"},
			{
				id:    "group_deeplink",
				label: "Generate deeplink",
				fields: []formField{
					{key: "bot_username", label: "Bot username", placeholder: "my_maxbridge_bot"},
					{key: "payload", label: "Payload", placeholder: "invite_code"},
				},
			},
			{id: "group_disable", label: "Disable group", dangerous: true},
		}
	case "MAX Users":
		return []menuAction{
			{id: "user_block", label: "Block user"},
			{id: "user_unblock", label: "Unblock user"},
			{id: "user_test", label: "Send test message"},
			{id: "user_remove", label: "Remove user", dangerous: true},
		}
	case "Invites":
		return []menuAction{
			{id: "invite_revoke", label: "Revoke invite", dangerous: true},
		}
	case "Routes":
		return []menuAction{
			{id: "route_pause", label: "Pause route"},
			{id: "route_resume", label: "Resume route"},
			{id: "route_delete", label: "Delete route", dangerous: true},
		}
	case "Delivery Queue":
		return []menuAction{{id: "queue_retry", label: "Retry now"}}
	default:
		return nil
	}
}

func (m Model) execActionCmd(action menuAction, entry listEntry, values map[string]string) tea.Cmd {
	section := m.currentSection
	return func() tea.Msg {
		if m.service == nil {
			return actionDoneMsg{status: "service not configured"}
		}
		status, err := m.executeAction(section, action.id, entry, values)
		return actionDoneMsg{status: status, err: err}
	}
}

func (m Model) executeAction(section, actionID string, entry listEntry, values map[string]string) (string, error) {
	svc := m.service
	switch actionID {
	case "group_add":
		chatID, err := parseInt64(values["chat_id"], "chat_id")
		if err != nil {
			return "", err
		}
		return svc.GroupAdd(chatID, values["title"])
	case "group_probe_all":
		return svc.GroupProbeAll()
	case "group_probe":
		chatID, err := intFromRow(entry.row, "chat_id")
		if err != nil {
			return "", err
		}
		return svc.GroupProbe(chatID)
	case "group_deeplink":
		return svc.GroupDeepLink(values["bot_username"], values["payload"])
	case "group_disable":
		chatID, err := intFromRow(entry.row, "chat_id")
		if err != nil {
			return "", err
		}
		return svc.GroupDisable(chatID)
	case "invite_create":
		return svc.InviteCreate(values["scope_type"], values["scope_id"], values["ttl"])
	case "invite_revoke":
		id, err := intFromRow(entry.row, "id")
		if err != nil {
			return "", err
		}
		return svc.InviteRevoke(id)
	case "route_add":
		chatID, err := parseInt64(values["chat_id"], "chat_id")
		if err != nil {
			return "", err
		}
		userID, err := parseInt64(values["max_user_id"], "max_user_id")
		if err != nil {
			return "", err
		}
		ignoreBots, err := strconv.ParseBool(strings.TrimSpace(values["ignore_bots"]))
		if err != nil {
			return "", fmt.Errorf("invalid ignore_bots")
		}
		return svc.RouteAdd(chatID, userID, strings.TrimSpace(values["filter_mode"]), ignoreBots)
	case "route_pause":
		id, err := intFromRow(entry.row, "id")
		if err != nil {
			return "", err
		}
		return svc.RoutePause(id)
	case "route_resume":
		id, err := intFromRow(entry.row, "id")
		if err != nil {
			return "", err
		}
		return svc.RouteResume(id)
	case "route_delete":
		id, err := intFromRow(entry.row, "id")
		if err != nil {
			return "", err
		}
		return svc.RouteDelete(id)
	case "user_block":
		id, err := intFromRow(entry.row, "max_user_id")
		if err != nil {
			return "", err
		}
		return svc.UserBlock(id)
	case "user_unblock":
		id, err := intFromRow(entry.row, "max_user_id")
		if err != nil {
			return "", err
		}
		return svc.UserUnblock(id)
	case "user_remove":
		id, err := intFromRow(entry.row, "max_user_id")
		if err != nil {
			return "", err
		}
		return svc.UserRemove(id)
	case "user_test":
		id, err := intFromRow(entry.row, "max_user_id")
		if err != nil {
			return "", err
		}
		return svc.UserTest(id)
	case "queue_retry":
		id, err := intFromRow(entry.row, "id")
		if err != nil {
			return "", err
		}
		return svc.QueueRetry(id)
	case "queue_clear_completed":
		days, err := strconv.Atoi(strings.TrimSpace(values["days"]))
		if err != nil {
			return "", fmt.Errorf("invalid days")
		}
		return svc.QueueClearCompleted(days)
	default:
		return "", fmt.Errorf("unknown action for %s: %s", section, actionID)
	}
}

func formatRowTitle(section string, row map[string]any) string {
	switch section {
	case "Telegram Groups":
		return fmt.Sprintf("chat=%v | %v", row["chat_id"], row["title"])
	case "MAX Users":
		return fmt.Sprintf("max_user_id=%v", row["max_user_id"])
	case "Invites":
		return fmt.Sprintf("invite id=%v | scope=%v", row["id"], row["scope"])
	case "Routes":
		return fmt.Sprintf("route id=%v | chat=%v -> user=%v", row["id"], row["chat_id"], row["max_user_id"])
	case "Delivery Queue":
		return fmt.Sprintf("job id=%v | status=%v", row["id"], row["status"])
	case "Logs":
		return fmt.Sprintf("[%v] %v", row["level"], row["message"])
	default:
		if id, ok := row["id"]; ok {
			return fmt.Sprintf("id=%v", id)
		}
		return fmt.Sprintf("item=%v", row)
	}
}

func formatRowDetail(section string, row map[string]any) string {
	switch section {
	case "Telegram Groups":
		return fmt.Sprintf("id=%v readiness=%v enabled=%v", row["id"], row["readiness"], row["enabled"])
	case "MAX Users":
		return fmt.Sprintf("blocked=%v last=%v", row["blocked"], row["last"])
	case "Invites":
		return fmt.Sprintf("expires_at=%v revoked_at=%v used_at=%v", row["expires_at"], row["revoked_at"], row["used_at"])
	case "Routes":
		return fmt.Sprintf("enabled=%v filter=%v ignore_bots=%v", row["enabled"], row["filter"], row["ignore_bots"])
	case "Delivery Queue":
		return fmt.Sprintf("attempts=%v/%v available_at=%v", row["attempts"], row["max_attempts"], row["available_at"])
	case "Logs":
		return fmt.Sprintf("source=%v created_at=%v", row["source"], row["created_at"])
	default:
		return ""
	}
}

func intFromRow(row map[string]any, key string) (int64, error) {
	if row == nil {
		return 0, fmt.Errorf("missing row")
	}
	v, ok := row[key]
	if !ok {
		return 0, fmt.Errorf("missing %s", key)
	}
	switch t := v.(type) {
	case int64:
		return t, nil
	case int32:
		return int64(t), nil
	case int:
		return int64(t), nil
	case float64:
		return int64(t), nil
	case string:
		return parseInt64(t, key)
	default:
		return 0, fmt.Errorf("invalid %s", key)
	}
}

func parseInt64(raw, field string) (int64, error) {
	v, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", field)
	}
	return v, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
