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
		fmt.Fprintf(b, "Группы: %d\nMAX users: %d\nRoutes: %d\nQueue depth: %d\nRetry: %d\nDead-letter: %d\nOldest pending: %ds\n\n",
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
			fmt.Fprintln(b, "DB: FAIL")
		} else {
			fmt.Fprintln(b, "DB: OK")
		}
		if err := s.tg.Ping(ctx); err != nil {
			fmt.Fprintln(b, "Telegram API: DEGRADED")
		} else {
			fmt.Fprintln(b, "Telegram API: OK")
		}
		if err := s.mx.Ping(ctx); err != nil {
			fmt.Fprintln(b, "MAX API: DEGRADED")
		} else {
			fmt.Fprintln(b, "MAX API: OK")
		}
		return SectionData{Content: b.String()}, nil
	case "Logs":
		rows, err := s.store.ListRecentEvents(ctx, 100)
		if err != nil {
			return SectionData{}, err
		}
		return SectionData{Rows: rows}, nil
	case "Settings":
		return SectionData{Content: "Settings управляются env/secrets файлами и Ansible.\nИспользуйте docs/operations.md для параметров."}, nil
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
		return "", fmt.Errorf("title is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.AddTelegramGroup(ctx, chatID, title); err != nil {
		return "", err
	}
	return "group added", nil
}

func (s *AdminService) GroupProbe(chatID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	res, err := s.tg.ProbeGroup(ctx, chatID)
	if err != nil {
		_ = s.store.UpdateGroupReadiness(ctx, chatID, string(domain.GroupBlocked), err.Error())
		return "probe failed: " + err.Error(), nil
	}
	_ = s.store.UpdateGroupReadiness(ctx, chatID, string(res.Readiness), res.Reason)
	return fmt.Sprintf("probe=%s reason=%s", res.Readiness, res.Reason), nil
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
	return fmt.Sprintf("probeall done, successful probes: %d", okCount), nil
}

func (s *AdminService) GroupDisable(chatID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.RemoveGroup(ctx, chatID); err != nil {
		return "", err
	}
	return "group disabled", nil
}

func (s *AdminService) GroupDeepLink(botUsername, payload string) (string, error) {
	botUsername = strings.TrimSpace(botUsername)
	payload = strings.TrimSpace(payload)
	if botUsername == "" || payload == "" {
		return "", fmt.Errorf("bot_username and payload are required")
	}
	return s.tg.DeepLinkStartGroup(botUsername, payload), nil
}

func (s *AdminService) InviteCreate(scopeType, scopeID, ttlRaw string) (string, error) {
	ttl, err := time.ParseDuration(strings.TrimSpace(ttlRaw))
	if err != nil {
		return "", fmt.Errorf("invalid ttl (example: 24h)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	out, err := s.invites.CreateInvite(ctx, invites.CreateInviteInput{
		ScopeType: strings.TrimSpace(scopeType),
		ScopeID:   strings.TrimSpace(scopeID),
		TTL:       ttl,
		SingleUse: true,
		Metadata:  map[string]any{"created_by": "tui"},
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("invite created id=%d expires=%s raw=%s (show once)", out.InviteID, out.Expires.Format(time.RFC3339), out.RawCode), nil
}

func (s *AdminService) InviteRevoke(inviteID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.RevokeInvite(ctx, inviteID); err != nil {
		return "", err
	}
	return "invite revoked", nil
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
	return fmt.Sprintf("route created id=%d", id), nil
}

func (s *AdminService) RoutePause(routeID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.UpdateRouteState(ctx, routeID, false); err != nil {
		return "", err
	}
	return "route paused", nil
}

func (s *AdminService) RouteResume(routeID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.UpdateRouteState(ctx, routeID, true); err != nil {
		return "", err
	}
	return "route resumed", nil
}

func (s *AdminService) RouteDelete(routeID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.DeleteRoute(ctx, routeID); err != nil {
		return "", err
	}
	return "route deleted", nil
}

func (s *AdminService) UserBlock(maxUserID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.SetUserBlocked(ctx, maxUserID, true); err != nil {
		return "", err
	}
	return "user blocked", nil
}

func (s *AdminService) UserUnblock(maxUserID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.SetUserBlocked(ctx, maxUserID, false); err != nil {
		return "", err
	}
	return "user unblocked", nil
}

func (s *AdminService) UserRemove(maxUserID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.RemoveUser(ctx, maxUserID); err != nil {
		return "", err
	}
	return "user removed", nil
}

func (s *AdminService) UserTest(maxUserID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.mx.SendMessage(ctx, maxUserID, "Bridge test send"); err != nil {
		return "test send failed", err
	}
	return "test send success", nil
}

func (s *AdminService) QueueRetry(jobID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.RetryJobNow(ctx, jobID); err != nil {
		return "", err
	}
	return "job scheduled for retry", nil
}

func (s *AdminService) QueueClearCompleted(days int) (string, error) {
	if days < 1 {
		return "", fmt.Errorf("days must be >= 1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if err := s.store.ClearOldCompleted(ctx, days); err != nil {
		return "", err
	}
	return "completed jobs cleanup triggered", nil
}

func normalizeFilterMode(mode string) (string, error) {
	mode = strings.TrimSpace(mode)
	switch mode {
	case string(domain.RouteFilterAll), string(domain.RouteFilterTextOnly), string(domain.RouteFilterMentions):
		return mode, nil
	default:
		return "", fmt.Errorf("invalid filter_mode")
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
		return "unknown command; use help", nil
	}
}

func (s *AdminService) execGroup(p []string) (string, error) {
	if len(p) < 3 {
		return "usage: group <add|probe|probeall|remove|deeplink> ...", nil
	}
	switch p[1] {
	case "add":
		if len(p) < 4 {
			return "usage: group add <chat_id> <title>", nil
		}
		chatID, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "invalid chat_id", nil
		}
		title := strings.Join(p[3:], " ")
		return s.GroupAdd(chatID, title)
	case "probe":
		chatID, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "invalid chat_id", nil
		}
		return s.GroupProbe(chatID)
	case "probeall":
		return s.GroupProbeAll()
	case "remove":
		chatID, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "invalid chat_id", nil
		}
		return s.GroupDisable(chatID)
	case "deeplink":
		if len(p) < 4 {
			return "usage: group deeplink <bot_username> <payload>", nil
		}
		return s.GroupDeepLink(p[2], p[3])
	default:
		return "unknown group command", nil
	}
}

func (s *AdminService) execInvite(p []string) (string, error) {
	if len(p) < 3 {
		return "usage: invite <create|revoke> ...", nil
	}
	switch p[1] {
	case "create":
		if len(p) < 5 {
			return "usage: invite create <group|route|entity> <scope_id> <ttl>", nil
		}
		return s.InviteCreate(p[2], p[3], p[4])
	case "revoke":
		id, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "invalid invite_id", nil
		}
		return s.InviteRevoke(id)
	default:
		return "unknown invite command", nil
	}
}

func (s *AdminService) execRoute(p []string) (string, error) {
	if len(p) < 3 {
		return "usage: route <add|pause|resume|delete> ...", nil
	}
	switch p[1] {
	case "add":
		if len(p) < 6 {
			return "usage: route add <chat_id> <max_user_id> <all|text_only|mentions_only> <ignore_bots:true|false>", nil
		}
		chatID, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "invalid chat_id", nil
		}
		userID, err := strconv.ParseInt(p[3], 10, 64)
		if err != nil {
			return "invalid max_user_id", nil
		}
		ignore, err := strconv.ParseBool(p[5])
		if err != nil {
			return "invalid ignore_bots", nil
		}
		return s.RouteAdd(chatID, userID, p[4], ignore)
	case "pause":
		id, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "invalid route_id", nil
		}
		return s.RoutePause(id)
	case "resume":
		id, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "invalid route_id", nil
		}
		return s.RouteResume(id)
	case "delete":
		id, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "invalid route_id", nil
		}
		return s.RouteDelete(id)
	default:
		return "unknown route command", nil
	}
}

func (s *AdminService) execQueue(p []string) (string, error) {
	if len(p) < 3 {
		return "usage: queue <retry|clear-completed> ...", nil
	}
	switch p[1] {
	case "retry":
		id, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "invalid job_id", nil
		}
		return s.QueueRetry(id)
	case "clear-completed":
		days, err := strconv.Atoi(p[2])
		if err != nil {
			return "invalid days", nil
		}
		return s.QueueClearCompleted(days)
	default:
		return "unknown queue command", nil
	}
}

func (s *AdminService) execUser(p []string) (string, error) {
	if len(p) < 3 {
		return "usage: user <block|unblock|remove|test> ...", nil
	}
	id, err := strconv.ParseInt(p[2], 10, 64)
	if err != nil {
		return "invalid max_user_id", nil
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
	default:
		return "unknown user command", nil
	}
}

func renderRows(rows []map[string]any) string {
	if len(rows) == 0 {
		return "<empty>"
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
help

group add <chat_id> <title>
group probe <chat_id>
group probeall now
group remove <chat_id>
group deeplink <bot_username> <payload>

invite create <group|route|entity> <scope_id> <ttl>
invite revoke <invite_id>

route add <chat_id> <max_user_id> <all|text_only|mentions_only> <ignore_bots:true|false>
route pause <route_id>
route resume <route_id>
route delete <route_id>

user block <max_user_id>
user unblock <max_user_id>
user remove <max_user_id>
user test <max_user_id>

queue retry <job_id>
queue clear-completed <days>
`)
}
