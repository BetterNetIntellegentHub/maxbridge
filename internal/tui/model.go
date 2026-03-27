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

const backActionID = "__back"

type rowKind int

const (
	rowRecord rowKind = iota
	rowSectionAction
	rowBack
)

type listEntry struct {
	kind   rowKind
	title  string
	detail string
	row    map[string]any
	action menuAction
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
	ret    uiMode
}

type pendingAction struct {
	action menuAction
	entry  listEntry
	values map[string]string
	ret    uiMode
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
	confirmIndex   int

	status string
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
	return nil
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
			m.status = fmt.Sprintf("ошибка: %v", v.err)
			return m, nil
		}
		if v.section == m.currentSection {
			m.rows = m.buildEntries(v.section, v.data.Rows)
			m.sectionContent = strings.TrimSpace(v.data.Content)
			if m.sectionContent == "" && len(m.rows) == 1 && m.rows[0].kind == rowBack {
				if v.section == "Dashboard" {
					m.sectionContent = "Панель пока не вернула данные."
				} else {
					m.sectionContent = "В этом разделе пока нет данных."
				}
			}
			if m.rowIndex >= len(m.rows) {
				m.rowIndex = max(0, len(m.rows)-1)
			}
		}
		return m, nil
	case actionDoneMsg:
		if v.err != nil {
			m.status = fmt.Sprintf("ошибка действия: %v", v.err)
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
	case "enter":
		selected := m.sections[m.index]
		if selected == "Exit" {
			return m, tea.Quit
		}
		m.currentSection = selected
		m.sectionContent = "Загрузка..."
		m.rows = nil
		m.rowIndex = 0
		m.actions = nil
		m.actionIndex = 0
		m.pending = nil
		m.confirmIndex = 0
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
			m.status = "в этом разделе нет пунктов"
			return m, nil
		}
		selected := m.rows[m.rowIndex]
		switch selected.kind {
		case rowBack:
			m.mode = modeSections
			return m, nil
		case rowSectionAction:
			return m.startAction(selected.action, selected)
		case rowRecord:
			m.actions = withBackAction(m.buildRowActions(m.currentSection, selected))
			m.actionIndex = 0
			m.mode = modeActions
			return m, nil
		}
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
		if act.id == backActionID {
			m.mode = modeRows
			m.actions = nil
			m.actionIndex = 0
			return m, nil
		}
		return m.startAction(act, m.rows[m.rowIndex])
	}
	return m, nil
}

func (m Model) startAction(act menuAction, entry listEntry) (tea.Model, tea.Cmd) {
	retMode := m.mode
	if len(act.fields) > 0 {
		values := make([]string, len(act.fields))
		for i, f := range act.fields {
			values[i] = f.defaultVal
		}
		m.form = actionForm{action: act, entry: entry, fields: act.fields, values: values, index: 0, ret: retMode}
		m.mode = modeForm
		return m, nil
	}
	if act.dangerous {
		m.pending = &pendingAction{action: act, entry: entry, values: map[string]string{}, ret: retMode}
		m.confirmIndex = 0
		m.mode = modeConfirm
		return m, nil
	}
	m.mode = modeRows
	if retMode == modeActions {
		m.actions = nil
		m.actionIndex = 0
	}
	return m, m.execActionCmd(act, entry, map[string]string{})
}

func (m Model) updateForm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	saveIndex := len(m.form.fields)
	backIndex := len(m.form.fields) + 1
	total := len(m.form.fields) + 2
	switch key.String() {
	case "esc", "left", "h":
		backMode := m.form.ret
		m.form = actionForm{}
		m.mode = backMode
		return m, nil
	case "up", "k", "shift+tab", "backtab":
		if m.form.index > 0 {
			m.form.index--
		}
		return m, nil
	case "down", "j", "tab":
		if m.form.index < total-1 {
			m.form.index++
		}
		return m, nil
	case "enter":
		if m.form.index == backIndex {
			backMode := m.form.ret
			m.form = actionForm{}
			m.mode = backMode
			return m, nil
		}
		if len(m.form.fields) > 0 && m.form.index < len(m.form.fields)-1 {
			m.form.index++
			return m, nil
		}
		if m.form.index != saveIndex {
			return m, nil
		}

		values := map[string]string{}
		for i, f := range m.form.fields {
			values[f.key] = strings.TrimSpace(m.form.values[i])
		}
		act := m.form.action
		entry := m.form.entry
		retMode := m.form.ret
		m.form = actionForm{}
		if act.dangerous {
			m.pending = &pendingAction{action: act, entry: entry, values: values, ret: retMode}
			m.confirmIndex = 0
			m.mode = modeConfirm
			return m, nil
		}
		m.mode = modeRows
		if retMode == modeActions {
			m.actions = nil
			m.actionIndex = 0
		}
		return m, m.execActionCmd(act, entry, values)
	case "backspace":
		if m.form.index >= len(m.form.fields) {
			return m, nil
		}
		if len(m.form.values[m.form.index]) > 0 {
			m.form.values[m.form.index] = m.form.values[m.form.index][:len(m.form.values[m.form.index])-1]
		}
		return m, nil
	default:
		if key.Type == tea.KeyRunes && m.form.index < len(m.form.fields) {
			m.form.values[m.form.index] += key.String()
		}
		return m, nil
	}
}

func (m Model) updateConfirm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	backMode := modeRows
	if m.pending != nil {
		backMode = m.pending.ret
	}
	switch key.String() {
	case "esc", "left", "h", "n":
		m.pending = nil
		m.mode = backMode
		m.confirmIndex = 1
		return m, nil
	case "up", "k":
		if m.confirmIndex > 0 {
			m.confirmIndex--
		}
		return m, nil
	case "down", "j", "tab":
		if m.confirmIndex < 1 {
			m.confirmIndex++
		}
		return m, nil
	case "y":
		m.confirmIndex = 0
		fallthrough
	case "enter":
		if m.confirmIndex == 1 {
			m.pending = nil
			m.mode = backMode
			return m, nil
		}
		if m.pending == nil {
			m.mode = modeRows
			return m, nil
		}
		p := *m.pending
		m.pending = nil
		m.mode = modeRows
		if p.ret == modeActions {
			m.actions = nil
			m.actionIndex = 0
		}
		m.confirmIndex = 0
		return m, m.execActionCmd(p.action, p.entry, p.values)
	}
	return m, nil
}

func (m Model) View() string {
	out := &strings.Builder{}
	fmt.Fprintln(out, "Операторский интерфейс MAXBridge")
	fmt.Fprintln(out, "====================")
	fmt.Fprintln(out, "")

	switch m.mode {
	case modeSections:
		fmt.Fprintln(out, "Главное меню")
		fmt.Fprintln(out, "------------")
		for i, s := range m.sections {
			printMenuLine(out, i == m.index, i+1, sectionLabel(s), "")
		}
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Enter: открыть  |  q: выход")
	case modeRows:
		fmt.Fprintf(out, "%s\n", sectionLabel(m.currentSection))
		fmt.Fprintln(out, strings.Repeat("-", len(sectionLabel(m.currentSection))))
		if strings.TrimSpace(m.sectionContent) != "" {
			fmt.Fprintln(out, "")
			fmt.Fprintln(out, m.sectionContent)
			fmt.Fprintln(out, "")
		}
		if len(m.rows) == 0 && strings.TrimSpace(m.sectionContent) == "" {
			fmt.Fprintln(out, "<пусто>")
		}
		if len(m.rows) > 0 {
			for i, row := range m.rows {
				printMenuLine(out, i == m.rowIndex, i+1, row.title, row.detail)
			}
		}
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Enter: открыть/выбрать  |  r: обновить  |  Esc: назад")
	case modeActions:
		entry := m.rows[m.rowIndex]
		fmt.Fprintf(out, "Действия: %s\n", sectionLabel(m.currentSection))
		fmt.Fprintln(out, strings.Repeat("-", len(sectionLabel(m.currentSection))+10))
		fmt.Fprintf(out, "Объект: %s\n\n", entry.title)
		for i, act := range m.actions {
			detail := ""
			if act.dangerous {
				detail = "требует подтверждения"
			}
			printMenuLine(out, i == m.actionIndex, i+1, act.label, detail)
		}
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Enter: выбрать  |  Esc: назад")
	case modeForm:
		fmt.Fprintf(out, "Ввод параметров: %s\n", m.form.action.label)
		fmt.Fprintln(out, strings.Repeat("-", len(m.form.action.label)+16))
		fmt.Fprintln(out, "")
		for i, f := range m.form.fields {
			v := m.form.values[i]
			if v == "" {
				v = f.placeholder
			}
			label := fmt.Sprintf("%s: %s", f.label, v)
			printMenuLine(out, i == m.form.index, i+1, label, "")
		}
		printMenuLine(out, m.form.index == len(m.form.fields), len(m.form.fields)+1, "Сохранить", "выполнить действие")
		printMenuLine(out, m.form.index == len(m.form.fields)+1, len(m.form.fields)+2, "Назад", "вернуться к действиям")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Enter: выбрать пункт")
	case modeConfirm:
		fmt.Fprintln(out, "Подтверждение")
		fmt.Fprintln(out, "------------")
		if m.pending != nil {
			fmt.Fprintf(out, "Действие: %s\n", m.pending.action.label)
			fmt.Fprintf(out, "Объект: %s\n\n", m.pending.entry.title)
		}
		printMenuLine(out, m.confirmIndex == 0, 1, "Подтвердить", "выполнить сейчас")
		printMenuLine(out, m.confirmIndex == 1, 2, "Назад", "отмена")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Enter: выбрать  |  y: подтвердить")
	}

	if m.status != "" {
		fmt.Fprintf(out, "\nСтатус: %s\n", m.status)
	}

	return out.String()
}

func sectionLabel(key string) string {
	switch key {
	case "Dashboard":
		return "Панель"
	case "Telegram Groups":
		return "Группы Telegram"
	case "MAX Users":
		return "Пользователи MAX"
	case "Invites":
		return "Инвайты"
	case "Routes":
		return "Маршруты"
	case "Delivery Queue":
		return "Очередь доставки"
	case "Health Checks":
		return "Проверки"
	case "Logs":
		return "Логи"
	case "Settings":
		return "Настройки"
	case "Exit":
		return "Выход"
	default:
		return key
	}
}

func printMenuLine(b *strings.Builder, selected bool, number int, title, detail string) {
	prefix := "  "
	if selected {
		prefix = "> "
	}
	fmt.Fprintf(b, "%s%2d. %s\n", prefix, number, title)
	if detail != "" {
		fmt.Fprintf(b, "     %s\n", detail)
	}
}

func withBackAction(actions []menuAction) []menuAction {
	back := menuAction{id: backActionID, label: "Назад"}
	out := make([]menuAction, 0, len(actions)+1)
	out = append(out, actions...)
	out = append(out, back)
	return out
}

func (m Model) loadSectionCmd(section string) tea.Cmd {
	return func() tea.Msg {
		if m.service == nil {
			return sectionLoadedMsg{section: section, data: SectionData{Content: "Сервис не настроен."}}
		}
		data, err := m.service.LoadSection(section)
		return sectionLoadedMsg{section: section, data: data, err: err}
	}
}

func (m Model) buildEntries(section string, rows []map[string]any) []listEntry {
	sectionActions := m.buildSectionActions(section)
	entries := make([]listEntry, 0, len(rows)+len(sectionActions)+1)

	for _, row := range rows {
		entries = append(entries, listEntry{
			kind:   rowRecord,
			title:  formatRowTitle(section, row),
			detail: formatRowDetail(section, row),
			row:    row,
		})
	}

	for _, act := range sectionActions {
		entries = append(entries, listEntry{
			kind:   rowSectionAction,
			title:  act.label,
			detail: "действие раздела",
			action: act,
		})
	}

	entries = append(entries, listEntry{kind: rowBack, title: "Назад", detail: "вернуться в главное меню"})
	return entries
}

func (m Model) buildSectionActions(section string) []menuAction {
	switch section {
	case "Telegram Groups":
		return []menuAction{
			{
				id:    "group_add",
				label: "Добавить группу",
				fields: []formField{
					{key: "chat_id", label: "ID чата", placeholder: "-1001234567890"},
					{key: "title", label: "Название", placeholder: "Операционная группа"},
				},
			},
			{id: "group_probe_all", label: "Проверить все группы"},
		}
	case "Invites":
		return []menuAction{
			{
				id:    "invite_create",
				label: "Создать инвайт",
				fields: []formField{
					{key: "scope_type", label: "Тип области", placeholder: "group|route|entity", defaultVal: "group"},
					{key: "scope_id", label: "ID области", placeholder: "123"},
					{key: "ttl", label: "TTL", placeholder: "24h", defaultVal: "24h"},
				},
			},
		}
	case "Routes":
		return []menuAction{
			{
				id:    "route_add",
				label: "Добавить маршрут",
				fields: []formField{
					{key: "chat_id", label: "Чат Telegram (ID)", placeholder: "-1001234567890"},
					{key: "max_user_id", label: "Пользователь MAX (ID)", placeholder: "10001"},
				},
			},
		}
	case "Delivery Queue":
		return []menuAction{
			{
				id:        "queue_clear_completed",
				label:     "Очистить завершённые задания",
				dangerous: true,
				fields: []formField{
					{key: "days", label: "Старше дней", placeholder: "7", defaultVal: "7"},
				},
			},
		}
	default:
		return nil
	}
}

func (m Model) buildRowActions(section string, entry listEntry) []menuAction {
	switch section {
	case "Telegram Groups":
		return []menuAction{
			{id: "group_probe", label: "Проверить группу"},
			{
				id:    "group_deeplink",
				label: "Сформировать ссылку",
				fields: []formField{
					{key: "bot_username", label: "Имя бота", placeholder: "my_maxbridge_bot"},
					{key: "payload", label: "Полезная нагрузка", placeholder: "invite_code"},
				},
			},
			{id: "group_disable", label: "Отключить группу", dangerous: true},
		}
	case "MAX Users":
		return []menuAction{
			{id: "user_block", label: "Заблокировать пользователя"},
			{id: "user_unblock", label: "Разблокировать пользователя"},
			{id: "user_test", label: "Отправить тест"},
			{id: "user_remove", label: "Удалить пользователя", dangerous: true},
		}
	case "Invites":
		return []menuAction{{id: "invite_revoke", label: "Отозвать инвайт", dangerous: true}}
	case "Routes":
		return []menuAction{
			{id: "route_pause", label: "Поставить на паузу"},
			{id: "route_resume", label: "Возобновить"},
			{id: "route_delete", label: "Удалить маршрут", dangerous: true},
		}
	case "Delivery Queue":
		return []menuAction{{id: "queue_retry", label: "Повторить доставку"}}
	default:
		return nil
	}
}

func (m Model) execActionCmd(action menuAction, entry listEntry, values map[string]string) tea.Cmd {
	section := m.currentSection
	return func() tea.Msg {
		if m.service == nil {
			return actionDoneMsg{status: "Сервис не настроен."}
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
		return svc.RouteAdd(chatID, userID, "all", true)
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
			return "", fmt.Errorf("некорректное значение дней")
		}
		return svc.QueueClearCompleted(days)
	default:
		return "", fmt.Errorf("неизвестное действие для %s: %s", section, actionID)
	}
}

func formatRowTitle(section string, row map[string]any) string {
	switch section {
	case "Telegram Groups":
		title := strings.TrimSpace(fmt.Sprintf("%v", row["title"]))
		if title == "" || title == "<nil>" {
			title = "Без названия"
		}
		return fmt.Sprintf("%s", title)
	case "MAX Users":
		return fmt.Sprintf("Пользователь MAX")
	case "Invites":
		return fmt.Sprintf("Область: %v", row["scope"])
	case "Routes":
		groupTitle := strings.TrimSpace(fmt.Sprintf("%v", row["group_title"]))
		if groupTitle == "" || groupTitle == "<nil>" {
			groupTitle = fmt.Sprintf("Чат %v", row["chat_id"])
		}
		return fmt.Sprintf("%s -> Пользователь MAX", groupTitle)
	case "Delivery Queue":
		return fmt.Sprintf("Статус: %v", row["status"])
	case "Logs":
		return fmt.Sprintf("[%v] %v", row["level"], row["message"])
	default:
		if id, ok := row["id"]; ok {
			return fmt.Sprintf("ID=%v", id)
		}
		return fmt.Sprintf("Элемент=%v", row)
	}
}

func formatRowDetail(section string, row map[string]any) string {
	switch section {
	case "Telegram Groups":
		return fmt.Sprintf("chat_id=%v id=%v готовность=%v включена=%v", row["chat_id"], row["id"], row["readiness"], row["enabled"])
	case "MAX Users":
		return fmt.Sprintf("max_user_id=%v id=%v заблокирован=%v последнее=%v", row["max_user_id"], row["id"], row["blocked"], row["last"])
	case "Invites":
		return fmt.Sprintf("id=%v до=%v отозван=%v использован=%v", row["id"], row["expires_at"], row["revoked_at"], row["used_at"])
	case "Routes":
		return fmt.Sprintf("id=%v chat_id=%v max_user_id=%v включен=%v фильтр=%v", row["id"], row["chat_id"], row["max_user_id"], row["enabled"], row["filter"])
	case "Delivery Queue":
		return fmt.Sprintf("job_id=%v max_user_id=%v chat_id=%v попытки=%v/%v доступно=%v", row["id"], row["max_user_id"], row["chat_id"], row["attempts"], row["max_attempts"], row["available_at"])
	case "Logs":
		return fmt.Sprintf("источник=%v время=%v", row["source"], row["created_at"])
	default:
		return ""
	}
}

func intFromRow(row map[string]any, key string) (int64, error) {
	if row == nil {
		return 0, fmt.Errorf("отсутствуют данные строки")
	}
	v, ok := row[key]
	if !ok {
		return 0, fmt.Errorf("поле %s отсутствует", key)
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
		return 0, fmt.Errorf("некорректное поле %s", key)
	}
}

func parseInt64(raw, field string) (int64, error) {
	v, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("некорректное значение %s", field)
	}
	return v, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
