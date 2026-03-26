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

type AdminService struct {
	store    *storage.Store
	tg       *telegram.Client
	mx       *maxapi.Client
	invites  *invites.Service
	timeout  time.Duration
}

func NewAdminService(store *storage.Store, tg *telegram.Client, mx *maxapi.Client, inv *invites.Service) *AdminService {
	return &AdminService{store: store, tg: tg, mx: mx, invites: inv, timeout: 5 * time.Second}
}

func (s *AdminService) RenderSection(section string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	switch section {
	case "Dashboard":
		groups, users, routes, err := s.store.CountCore(ctx)
		if err != nil {
			return "", err
		}
		queue, err := s.store.GetQueueStats(ctx)
		if err != nil {
			return "", err
		}
		events, _ := s.store.ListRecentEvents(ctx, 8)
		b := &strings.Builder{}
		fmt.Fprintf(b, "Группы: %d\nMAX users: %d\nRoutes: %d\nQueue depth: %d\nRetry: %d\nDead-letter: %d\nOldest pending: %ds\n\n",
			groups, users, routes, queue.PendingDepth, queue.RetryDepth, queue.DeadLetterDepth, queue.OldestPendingAgeS)
		fmt.Fprintln(b, "Последние события:")
		for _, e := range events {
			fmt.Fprintf(b, "- [%v] %v: %v\n", e["level"], e["source"], e["message"])
		}
		return b.String(), nil
	case "Telegram Groups":
		rows, err := s.store.ListTelegramGroups(ctx)
		if err != nil {
			return "", err
		}
		return renderRows(rows), nil
	case "MAX Users":
		rows, err := s.store.ListMaxUsers(ctx)
		if err != nil {
			return "", err
		}
		return renderRows(rows), nil
	case "Invites":
		rows, err := s.store.ListInvites(ctx)
		if err != nil {
			return "", err
		}
		return renderRows(rows), nil
	case "Routes":
		rows, err := s.store.ListRoutes(ctx)
		if err != nil {
			return "", err
		}
		return renderRows(rows), nil
	case "Delivery Queue":
		rows, err := s.store.ListQueue(ctx, "", 200)
		if err != nil {
			return "", err
		}
		return renderRows(rows), nil
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
		return b.String(), nil
	case "Logs":
		rows, err := s.store.ListRecentEvents(ctx, 100)
		if err != nil {
			return "", err
		}
		return renderRows(rows), nil
	case "Settings":
		return "Settings управляются env/secrets файлами и Ansible.\nИспользуйте docs/operations.md для параметров.", nil
	default:
		return "", nil
	}
}

func (s *AdminService) Exec(raw string) (string, error) {
	parts := strings.Fields(strings.TrimSpace(raw))
	if len(parts) == 0 {
		return "", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	switch parts[0] {
	case "help":
		return helpText(), nil
	case "group":
		return s.execGroup(ctx, parts)
	case "invite":
		return s.execInvite(ctx, parts)
	case "route":
		return s.execRoute(ctx, parts)
	case "queue":
		return s.execQueue(ctx, parts)
	case "user":
		return s.execUser(ctx, parts)
	default:
		return "unknown command; use help", nil
	}
}

func (s *AdminService) execGroup(ctx context.Context, p []string) (string, error) {
	if len(p) < 3 {
		return "usage: group <add|probe|probeall|remove> ...", nil
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
		if err := s.store.AddTelegramGroup(ctx, chatID, title); err != nil {
			return "", err
		}
		return "group added", nil
	case "probe":
		chatID, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "invalid chat_id", nil
		}
		res, err := s.tg.ProbeGroup(ctx, chatID)
		if err != nil {
			_ = s.store.UpdateGroupReadiness(ctx, chatID, string(domain.GroupBlocked), err.Error())
			return "probe failed: " + err.Error(), nil
		}
		_ = s.store.UpdateGroupReadiness(ctx, chatID, string(res.Readiness), res.Reason)
		return fmt.Sprintf("probe=%s reason=%s", res.Readiness, res.Reason), nil
	case "probeall":
		groups, err := s.store.ListTelegramGroups(ctx)
		if err != nil {
			return "", err
		}
		okCount := 0
		for _, g := range groups {
			chatID, _ := g["chat_id"].(int64)
			res, probeErr := s.tg.ProbeGroup(ctx, chatID)
			if probeErr != nil {
				_ = s.store.UpdateGroupReadiness(ctx, chatID, string(domain.GroupBlocked), probeErr.Error())
				continue
			}
			_ = s.store.UpdateGroupReadiness(ctx, chatID, string(res.Readiness), res.Reason)
			okCount++
		}
		return fmt.Sprintf("probeall done, successful probes: %d", okCount), nil
	case "remove":
		chatID, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "invalid chat_id", nil
		}
		if err := s.store.RemoveGroup(ctx, chatID); err != nil {
			return "", err
		}
		return "group disabled", nil
	case "deeplink":
		if len(p) < 4 {
			return "usage: group deeplink <bot_username> <payload>", nil
		}
		return s.tg.DeepLinkStartGroup(p[2], p[3]), nil
	default:
		return "unknown group command", nil
	}
}

func (s *AdminService) execInvite(ctx context.Context, p []string) (string, error) {
	if len(p) < 3 {
		return "usage: invite <create|revoke> ...", nil
	}
	switch p[1] {
	case "create":
		if len(p) < 5 {
			return "usage: invite create <group|route|entity> <scope_id> <ttl>", nil
		}
		ttl, err := time.ParseDuration(p[4])
		if err != nil {
			return "invalid ttl (example: 24h)", nil
		}
		out, err := s.invites.CreateInvite(ctx, invites.CreateInviteInput{
			ScopeType: p[2],
			ScopeID:   p[3],
			TTL:       ttl,
			SingleUse: true,
			Metadata:  map[string]any{"created_by": "tui"},
		})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("invite created id=%d expires=%s raw=%s (show once)", out.InviteID, out.Expires.Format(time.RFC3339), out.RawCode), nil
	case "revoke":
		if len(p) < 3 {
			return "usage: invite revoke <invite_id>", nil
		}
		id, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "invalid invite_id", nil
		}
		if err := s.store.RevokeInvite(ctx, id); err != nil {
			return "", err
		}
		return "invite revoked", nil
	default:
		return "unknown invite command", nil
	}
}

func (s *AdminService) execRoute(ctx context.Context, p []string) (string, error) {
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
		id, err := s.store.CreateRoute(ctx, chatID, userID, p[4], ignore)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("route created id=%d", id), nil
	case "pause":
		id, _ := strconv.ParseInt(p[2], 10, 64)
		if err := s.store.UpdateRouteState(ctx, id, false); err != nil {
			return "", err
		}
		return "route paused", nil
	case "resume":
		id, _ := strconv.ParseInt(p[2], 10, 64)
		if err := s.store.UpdateRouteState(ctx, id, true); err != nil {
			return "", err
		}
		return "route resumed", nil
	case "delete":
		id, _ := strconv.ParseInt(p[2], 10, 64)
		if err := s.store.DeleteRoute(ctx, id); err != nil {
			return "", err
		}
		return "route deleted", nil
	default:
		return "unknown route command", nil
	}
}

func (s *AdminService) execQueue(ctx context.Context, p []string) (string, error) {
	if len(p) < 3 {
		return "usage: queue <retry|clear-completed> ...", nil
	}
	switch p[1] {
	case "retry":
		id, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			return "invalid job_id", nil
		}
		if err := s.store.RetryJobNow(ctx, id); err != nil {
			return "", err
		}
		return "job scheduled for retry", nil
	case "clear-completed":
		days, err := strconv.Atoi(p[2])
		if err != nil {
			return "invalid days", nil
		}
		if err := s.store.ClearOldCompleted(ctx, days); err != nil {
			return "", err
		}
		return "completed jobs cleanup triggered", nil
	default:
		return "unknown queue command", nil
	}
}

func (s *AdminService) execUser(ctx context.Context, p []string) (string, error) {
	if len(p) < 3 {
		return "usage: user <block|unblock|remove|test> ...", nil
	}
	id, err := strconv.ParseInt(p[2], 10, 64)
	if err != nil {
		return "invalid max_user_id", nil
	}
	switch p[1] {
	case "block":
		return "user blocked", s.store.SetUserBlocked(ctx, id, true)
	case "unblock":
		return "user unblocked", s.store.SetUserBlocked(ctx, id, false)
	case "remove":
		return "user removed", s.store.RemoveUser(ctx, id)
	case "test":
		err := s.mx.SendMessage(ctx, id, "Bridge test send")
		if err != nil {
			return "test send failed", err
		}
		return "test send success", nil
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


