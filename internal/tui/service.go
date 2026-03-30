package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"maxbridge/internal/domain"
	"maxbridge/internal/invites"
	maxapi "maxbridge/internal/max"
	"maxbridge/internal/storage"
	"maxbridge/internal/telegram"
)

type SectionData struct {
	Rows    []map[string]any
	Content string
}

type AdminService struct {
	store   *storage.Store
	tg      *telegram.Client
	mx      *maxapi.Client
	invites *invites.Service
	timeout time.Duration
}

func NewAdminService(store *storage.Store, tg *telegram.Client, mx *maxapi.Client, inv *invites.Service) *AdminService {
	return &AdminService{store: store, tg: tg, mx: mx, invites: inv, timeout: 5 * time.Second}
}

func (s *AdminService) LoadSection(section string) (SectionData, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	return s.loadSection(ctx, section)
}

func (s *AdminService) loadSection(ctx context.Context, section string) (SectionData, error) {
	switch section {
	case "Dashboard":
		groups, users, routes, err := s.store.CountCore(ctx)
		if err != nil {
			return SectionData{}, err
		}
		queue, err := s.store.GetQueueStats(ctx)
		if err != nil {
			return SectionData{}, err
		}
		events, _ := s.store.ListRecentEvents(ctx, 8)
		b := &strings.Builder{}
		fmt.Fprintf(b, "Группы: %d\nПользователи MAX: %d\nМаршруты: %d\nГлубина очереди: %d\nВ повторе: %d\nВ DLQ (dead-letter): %d\nВозраст старейшего ожидающего задания: %ds\n\n",
			groups, users, routes, queue.PendingDepth, queue.RetryDepth, queue.DeadLetterDepth, queue.OldestPendingAgeS)
		fmt.Fprintln(b, "Последние события:")
		for _, e := range events {
			fmt.Fprintf(b, "- [%v] %v: %v\n", e["level"], e["source"], e["message"])
		}
		return SectionData{Content: b.String()}, nil
	case "Telegram Groups":
		rows, err := s.store.ListTelegramGroups(ctx)
		if err != nil {
			return SectionData{}, err
		}
		return SectionData{Rows: rows}, nil
	case "MAX Users":
		rows, err := s.store.ListMaxUsers(ctx)
		if err != nil {
			return SectionData{}, err
		}
		return SectionData{Rows: rows}, nil
	case "Invites":
		rows, err := s.store.ListInvites(ctx)
		if err != nil {
			return SectionData{}, err
		}
		return SectionData{Rows: rows}, nil
	case "Routes":
		rows, err := s.store.ListRoutes(ctx)
		if err != nil {
			return SectionData{}, err
		}
		return SectionData{Rows: rows}, nil
	case "Delivery Queue":
		rows, err := s.store.ListQueue(ctx, "", 200)
		if err != nil {
			return SectionData{}, err
		}
		return SectionData{Rows: rows}, nil
	case "Health Checks":
		b := &strings.Builder{}
		if err := s.store.Ping(ctx); err != nil {
			fmt.Fprintln(b, "БД: ОШИБКА")
		} else {
			fmt.Fprintln(b, "БД: ОК")
		}
		if err := s.tg.Ping(ctx); err != nil {
			fmt.Fprintln(b, "Telegram API: ДЕГРАДИРОВАНО")
		} else {
			fmt.Fprintln(b, "Telegram API: ОК")
		}
		if err := s.mx.Ping(ctx); err != nil {
			fmt.Fprintln(b, "MAX API: ДЕГРАДИРОВАНО")
		} else {
			fmt.Fprintln(b, "MAX API: ОК")
		}
		return SectionData{Content: b.String()}, nil
	case "Logs":
		rows, err := s.store.ListRecentEvents(ctx, 100)
		if err != nil {
			return SectionData{}, err
		}
		return SectionData{Rows: rows}, nil
	case "Settings":
		return SectionData{Content: "Настройки управляются env/secrets файлами и Ansible.\nПараметры см. в docs/operations.md."}, nil
	default:
		return SectionData{}, nil
	}
}

func (s *AdminService) RenderSection(section string) (string, error) {
	data, err := s.LoadSection(section)
	if err != nil {
		return "", err
	}
	if len(data.Rows) > 0 {
		return renderRows(data.Rows), nil
	}
	return data.Content, nil
}

func (s *AdminService) GroupAdd(chatID int64, title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", fmt.Errorf("название обязательно")
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.AddTelegramGroup(ctx, chatID, title); err != nil {
		return "", err
	}
	return "Группа добавлена.", nil
}

func (s *AdminService) GroupProbe(chatID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	res, err := s.tg.ProbeGroup(ctx, chatID)
	if err != nil {
		_ = s.store.UpdateGroupReadiness(ctx, chatID, string(domain.GroupBlocked), err.Error())
		return "Проверка неуспешна: " + err.Error(), nil
	}
	_ = s.store.UpdateGroupReadiness(ctx, chatID, string(res.Readiness), res.Reason)
	return fmt.Sprintf("Проверка: %s, причина: %s", res.Readiness, res.Reason), nil
}

func (s *AdminService) GroupProbeAll() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	groups, err := s.store.ListTelegramGroups(ctx)
	if err != nil {
		return "", err
	}
	okCount := 0
	for _, g := range groups {
		chatID, ok := g["chat_id"].(int64)
		if !ok {
			continue
		}
		res, probeErr := s.tg.ProbeGroup(ctx, chatID)
		if probeErr != nil {
			_ = s.store.UpdateGroupReadiness(ctx, chatID, string(domain.GroupBlocked), probeErr.Error())
			continue
		}
		_ = s.store.UpdateGroupReadiness(ctx, chatID, string(res.Readiness), res.Reason)
		okCount++
	}
	return fmt.Sprintf("Проверка всех групп завершена, успешных проверок: %d", okCount), nil
}

func (s *AdminService) GroupDisable(chatID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.RemoveGroup(ctx, chatID); err != nil {
		return "", err
	}
	return "Группа отключена.", nil
}

func (s *AdminService) GroupDeepLink(botUsername, payload string) (string, error) {
	botUsername = strings.TrimSpace(botUsername)
	payload = strings.TrimSpace(payload)
	if botUsername == "" || payload == "" {
		return "", fmt.Errorf("bot_username и payload обязательны")
	}
	return s.tg.DeepLinkStartGroup(botUsername, payload), nil
}

func (s *AdminService) InviteCreate(scopeType, scopeID, ttlRaw, maxFullName string) (string, error) {
	ttl, err := time.ParseDuration(strings.TrimSpace(ttlRaw))
	if err != nil {
		return "", fmt.Errorf("некорректный ttl (пример: 24h)")
	}
	maxFullName = strings.TrimSpace(maxFullName)
	if maxFullName == "" {
		return "", fmt.Errorf("имя пользователя MAX обязательно")
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	out, err := s.invites.CreateInvite(ctx, invites.CreateInviteInput{
		ScopeType: strings.TrimSpace(scopeType),
		ScopeID:   strings.TrimSpace(scopeID),
		TTL:       ttl,
		SingleUse: true,
		Metadata: map[string]any{
			"created_by":    "tui",
			"max_full_name": maxFullName,
		},
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Инвайт создан: id=%d, имя=%s, истекает=%s, raw=%s (показать один раз)", out.InviteID, maxFullName, out.Expires.Format(time.RFC3339), out.RawCode), nil
}

func (s *AdminService) InviteRevoke(inviteID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.RevokeInvite(ctx, inviteID); err != nil {
		return "", err
	}
	return "Инвайт отозван.", nil
}

func (s *AdminService) RouteAdd(chatID, maxUserID int64, filterMode string, ignoreBots bool) (string, error) {
	mode, err := normalizeFilterMode(filterMode)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	id, err := s.store.CreateRoute(ctx, chatID, maxUserID, mode, ignoreBots)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Маршрут создан: id=%d", id), nil
}

func (s *AdminService) RoutePause(routeID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.UpdateRouteState(ctx, routeID, false); err != nil {
		return "", err
	}
	return "Маршрут поставлен на паузу.", nil
}

func (s *AdminService) RouteResume(routeID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.UpdateRouteState(ctx, routeID, true); err != nil {
		return "", err
	}
	return "Маршрут возобновлён.", nil
}

func (s *AdminService) RouteDelete(routeID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.DeleteRoute(ctx, routeID); err != nil {
		return "", err
	}
	return "Маршрут удалён.", nil
}

func (s *AdminService) UserBlock(maxUserID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.SetUserBlocked(ctx, maxUserID, true); err != nil {
		return "", err
	}
	return "Пользователь заблокирован.", nil
}

func (s *AdminService) UserUnblock(maxUserID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.SetUserBlocked(ctx, maxUserID, false); err != nil {
		return "", err
	}
	return "Пользователь разблокирован.", nil
}

func (s *AdminService) UserRemove(maxUserID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.RemoveUser(ctx, maxUserID); err != nil {
		return "", err
	}
	return "Пользователь удалён.", nil
}

func (s *AdminService) UserTest(maxUserID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.mx.SendMessage(ctx, maxUserID, "Тестовое сообщение из MaxBridge"); err != nil {
		return "Тестовая отправка неуспешна.", err
	}
	return "Тестовая отправка выполнена успешно.", nil
}

func (s *AdminService) UserRename(maxUserID int64, fullName string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return "", fmt.Errorf("имя пользователя MAX обязательно")
	}
	if err := s.store.UpdateMaxUserName(ctx, maxUserID, fullName); err != nil {
		return "", err
	}
	return fmt.Sprintf("Имя пользователя обновлено: %s", fullName), nil
}

func (s *AdminService) QueueRetry(jobID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.RetryJobNow(ctx, jobID); err != nil {
		return "", err
	}
	return "Задание поставлено на повтор.", nil
}

func (s *AdminService) QueueClearCompleted(days int) (string, error) {
	if days < 1 {
		return "", fmt.Errorf("значение days должно быть >= 1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.ClearOldCompleted(ctx, days); err != nil {
		return "", err
	}
	return fmt.Sprintf("Удалены завершённые задания старше %d дн.", days), nil
}

func (s *AdminService) QueueClearCompletedAll() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	deleted, err := s.store.ClearCompletedNow(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Удалено завершённых задач: %d.", deleted), nil
}

func normalizeFilterMode(mode string) (string, error) {
	mode = strings.TrimSpace(mode)
	switch mode {
	case string(domain.RouteFilterAll), string(domain.RouteFilterTextOnly), string(domain.RouteFilterMentions):
		return mode, nil
	default:
		return "", fmt.Errorf("некорректный filter_mode")
	}
}

func (s *AdminService) Exec(raw string) (string, error) {
	parts := strings.Fields(strings.TrimSpace(raw))
	if len(parts) == 0 {
		return "", nil
	}

	switch parts[0] {
	case "help":
		return helpText(), nil
	case "group":
		return s.execGroup(parts)
	case "invite":
		return s.execInvite(parts)
	case "route":
		return s.execRoute(parts)
	case "queue":
		return s.execQueue(parts)
	case "user":
		return s.execUser(parts)
	default:
		return "Неизвестная команда, используйте help.", nil
	}
}

func (s *AdminService) execGroup(p []string) (string, error) {
	if len(p) < 3 {
		return "Использование: group <add|probe|probeall|remove|deeplink> ...", nil
	}
	switch p[1] {
	case "add":
		if len(p) < 4 {
			return "Использование: group add <chat_id> <title>", nil
		}
		chatID, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "Некорректный chat_id.", nil
		}
		title := strings.Join(p[3:], " ")
		return s.GroupAdd(chatID, title)
	case "probe":
		chatID, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "Некорректный chat_id.", nil
		}
		return s.GroupProbe(chatID)
	case "probeall":
		return s.GroupProbeAll()
	case "remove":
		chatID, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "Некорректный chat_id.", nil
		}
		return s.GroupDisable(chatID)
	case "deeplink":
		if len(p) < 4 {
			return "Использование: group deeplink <bot_username> <payload>", nil
		}
		return s.GroupDeepLink(p[2], p[3])
	default:
		return "Неизвестная подкоманда group.", nil
	}
}

func (s *AdminService) execInvite(p []string) (string, error) {
	if len(p) < 3 {
		return "Использование: invite <create|revoke> ...", nil
	}
	switch p[1] {
	case "create":
		if len(p) < 6 {
			return "Использование: invite create <group|route|entity> <scope_id> <ttl> <имя_пользователя_MAX>", nil
		}
		return s.InviteCreate(p[2], p[3], p[4], strings.Join(p[5:], " "))
	case "revoke":
		id, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "Некорректный invite_id.", nil
		}
		return s.InviteRevoke(id)
	default:
		return "Неизвестная подкоманда invite.", nil
	}
}

func (s *AdminService) execRoute(p []string) (string, error) {
	if len(p) < 3 {
		return "Использование: route <add|pause|resume|delete> ...", nil
	}
	switch p[1] {
	case "add":
		if len(p) < 6 {
			return "Использование: route add <chat_id> <max_user_id> <all|text_only|mentions_only> <ignore_bots:true|false>", nil
		}
		chatID, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "Некорректный chat_id.", nil
		}
		userID, err := strconv.ParseInt(p[3], 10, 64)
		if err != nil {
			return "Некорректный max_user_id.", nil
		}
		ignore, err := strconv.ParseBool(p[5])
		if err != nil {
			return "Некорректный ignore_bots.", nil
		}
		return s.RouteAdd(chatID, userID, p[4], ignore)
	case "pause":
		id, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "Некорректный route_id.", nil
		}
		return s.RoutePause(id)
	case "resume":
		id, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "Некорректный route_id.", nil
		}
		return s.RouteResume(id)
	case "delete":
		id, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "Некорректный route_id.", nil
		}
		return s.RouteDelete(id)
	default:
		return "Неизвестная подкоманда route.", nil
	}
}

func (s *AdminService) execQueue(p []string) (string, error) {
	if len(p) < 3 {
		return "Использование: queue <retry|clear-completed> ...", nil
	}
	switch p[1] {
	case "retry":
		id, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "Некорректный job_id.", nil
		}
		return s.QueueRetry(id)
	case "clear-completed":
		days, err := strconv.Atoi(p[2])
		if err != nil {
			return "Некорректное значение days.", nil
		}
		return s.QueueClearCompleted(days)
	default:
		return "Неизвестная подкоманда queue.", nil
	}
}

func (s *AdminService) execUser(p []string) (string, error) {
	if len(p) < 3 {
		return "Использование: user <block|unblock|remove|test|rename> ...", nil
	}
	id, err := strconv.ParseInt(p[2], 10, 64)
	if err != nil {
		return "Некорректный max_user_id.", nil
	}
	switch p[1] {
	case "block":
		return s.UserBlock(id)
	case "unblock":
		return s.UserUnblock(id)
	case "remove":
		return s.UserRemove(id)
	case "test":
		return s.UserTest(id)
	case "rename":
		if len(p) < 4 {
			return "Использование: user rename <max_user_id> <имя>", nil
		}
		return s.UserRename(id, strings.Join(p[3:], " "))
	default:
		return "Неизвестная подкоманда user.", nil
	}
}

func renderRows(rows []map[string]any) string {
	if len(rows) == 0 {
		return "<пусто>"
	}
	b := &strings.Builder{}
	for _, r := range rows {
		pairs := make([]string, 0, len(r))
		for k, v := range r {
			pairs = append(pairs, fmt.Sprintf("%s=%v", k, v))
		}
		fmt.Fprintf(b, "- %s\n", strings.Join(pairs, " | "))
	}
	return b.String()
}

func helpText() string {
	return strings.TrimSpace(`
Справка (устаревший командный режим):

group add <chat_id> <title>
group probe <chat_id>
group probeall now
group remove <chat_id>
group deeplink <bot_username> <payload>

invite create <group|route|entity> <scope_id> <ttl> <имя_пользователя_MAX>
invite revoke <invite_id>

route add <chat_id> <max_user_id> <all|text_only|mentions_only> <ignore_bots:true|false>
route pause <route_id>
route resume <route_id>
route delete <route_id>

user block <max_user_id>
user unblock <max_user_id>
user remove <max_user_id>
user test <max_user_id>
user rename <max_user_id> <имя>

queue retry <job_id>
queue clear-completed <days>
`)
}
