package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"maxbridge/internal/domain"
)

var ErrInviteNotFound = errors.New("invite not found")

type Store struct {
	pool *pgxpool.Pool
}

type QueueStats struct {
	PendingDepth      int64
	RetryDepth        int64
	DeadLetterDepth   int64
	OldestPendingAgeS int64
}

func NewStore(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) CreateInvite(ctx context.Context, scopeType, scopeID, codeHash string, expiresAt time.Time, singleUse bool, metadata map[string]any) (int64, error) {
	meta, err := json.Marshal(metadata)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.pool.QueryRow(ctx, `
		INSERT INTO invites (scope_type, scope_id, code_hash, expires_at, single_use, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, scopeType, scopeID, codeHash, expiresAt, singleUse, meta).Scan(&id)
	return id, err
}

func (s *Store) RevokeInvite(ctx context.Context, inviteID int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM invites WHERE id = $1`, inviteID)
	return err
}

func (s *Store) ConsumeInvite(ctx context.Context, codeHash string) (domain.Invite, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Invite{}, err
	}
	defer tx.Rollback(ctx)

	var inv domain.Invite
	var meta []byte
	err = tx.QueryRow(ctx, `
		SELECT id, scope_type, scope_id, code_hash, expires_at, used_at, revoked_at, created_at, single_use, metadata
		FROM invites
		WHERE code_hash = $1
		FOR UPDATE
	`, codeHash).Scan(
		&inv.ID,
		&inv.ScopeType,
		&inv.ScopeID,
		&inv.CodeHash,
		&inv.ExpiresAt,
		&inv.UsedAt,
		&inv.RevokedAt,
		&inv.CreatedAt,
		&inv.SingleUse,
		&meta,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Invite{}, ErrInviteNotFound
		}
		return domain.Invite{}, err
	}
	_ = json.Unmarshal(meta, &inv.Metadata)

	now := time.Now().UTC()
	if inv.RevokedAt != nil || inv.ExpiresAt.Before(now) || (inv.SingleUse && inv.UsedAt != nil) {
		return domain.Invite{}, ErrInviteNotFound
	}

	_, err = tx.Exec(ctx, `DELETE FROM invites WHERE id = $1`, inv.ID)
	if err != nil {
		return domain.Invite{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Invite{}, err
	}

	return inv, nil
}

func (s *Store) UpsertLinkedUser(ctx context.Context, maxUserID int64, firstName, lastName string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO max_users (max_user_id, first_name, last_name, is_active, is_blocked, linked_at)
		VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), true, false, now())
		ON CONFLICT (max_user_id)
		DO UPDATE SET
			is_active = true,
			first_name = CASE WHEN NULLIF($2, '') IS NOT NULL THEN NULLIF($2, '') ELSE max_users.first_name END,
			last_name = CASE WHEN NULLIF($3, '') IS NOT NULL THEN NULLIF($3, '') ELSE max_users.last_name END,
			updated_at = now()
		RETURNING id
	`, maxUserID, strings.TrimSpace(firstName), strings.TrimSpace(lastName)).Scan(&id)
	return id, err
}

func (s *Store) UpdateMaxUserDeliveryStatus(ctx context.Context, maxUserID int64, status, detail string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE max_users
		SET last_delivery_status = $2,
			last_delivery_error = $3,
			last_delivery_at = now(),
			updated_at = now()
		WHERE max_user_id = $1
	`, maxUserID, status, detail)
	return err
}

func (s *Store) AddTelegramGroup(ctx context.Context, chatID int64, title string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO telegram_groups (telegram_chat_id, title, readiness)
		VALUES ($1, $2, 'LIMITED')
		ON CONFLICT (telegram_chat_id)
		DO UPDATE SET title = EXCLUDED.title, updated_at = now()
	`, chatID, title)
	return err
}

func (s *Store) UpdateGroupReadiness(ctx context.Context, chatID int64, readiness, reason string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE telegram_groups
		SET readiness = $2, readiness_reason = $3, updated_at = now()
		WHERE telegram_chat_id = $1
	`, chatID, readiness, reason)
	return err
}

func (s *Store) CreateRoute(ctx context.Context, telegramChatID, maxUserID int64, filterMode string, ignoreBots bool) (int64, error) {
	var routeID int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO routes (telegram_chat_id, max_user_id, enabled, filter_mode, ignore_bot_messages)
		VALUES ($1, $2, true, $3, $4)
		RETURNING id
	`, telegramChatID, maxUserID, filterMode, ignoreBots).Scan(&routeID)
	return routeID, err
}

func (s *Store) UpdateRouteState(ctx context.Context, routeID int64, enabled bool) error {
	_, err := s.pool.Exec(ctx, `UPDATE routes SET enabled = $2, updated_at = now() WHERE id = $1`, routeID, enabled)
	return err
}

func routePassesFilter(routeFilter string, ignoreBot bool, msg *domain.TelegramMessage) bool {
	if msg == nil {
		return false
	}
	if ignoreBot && msg.From != nil && msg.From.IsBot {
		return false
	}
	switch routeFilter {
	case string(domain.RouteFilterTextOnly):
		return domain.IsTextMessage(msg)
	case string(domain.RouteFilterMentions):
		return domain.IsMentionMessage(msg)
	default:
		return true
	}
}

func (s *Store) EnqueueTelegramUpdate(ctx context.Context, upd domain.TelegramUpdate, maxAttempts int) (int, error) {
	if upd.Message == nil {
		return 0, nil
	}
	msg := upd.Message

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT r.id, r.max_user_id, r.filter_mode, r.ignore_bot_messages
		FROM routes r
		JOIN max_users u ON u.max_user_id = r.max_user_id
		WHERE r.telegram_chat_id = $1
		  AND r.enabled = true
		  AND u.is_active = true
		  AND u.is_blocked = false
	`, msg.Chat.ID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type routeCandidate struct {
		routeID    int64
		maxUserID  int64
		filterMode string
		ignoreBots bool
	}

	candidates := make([]routeCandidate, 0, 8)
	for rows.Next() {
		var c routeCandidate
		if err := rows.Scan(&c.routeID, &c.maxUserID, &c.filterMode, &c.ignoreBots); err != nil {
			return 0, err
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	rows.Close()

	enqueued := 0
	payload, _ := json.Marshal(msg)
	for _, c := range candidates {
		if !routePassesFilter(c.filterMode, c.ignoreBots, msg) {
			continue
		}

		dedupe := domain.DedupeKey(c.routeID, msg.Chat.ID, msg.MessageID)
		var dedupeID int64
		err = tx.QueryRow(ctx, `
			INSERT INTO dedupe_records (dedupe_key, route_id, telegram_chat_id, telegram_message_id, expires_at)
			VALUES ($1, $2, $3, $4, now() + interval '14 days')
			ON CONFLICT (dedupe_key) DO NOTHING
			RETURNING id
		`, dedupe, c.routeID, msg.Chat.ID, msg.MessageID).Scan(&dedupeID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return 0, err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO delivery_jobs (
				route_id, telegram_chat_id, telegram_message_id, max_user_id, payload_json,
				status, attempts, max_attempts, available_at
			)
			VALUES ($1, $2, $3, $4, $5, 'pending', 0, $6, now())
		`, c.routeID, msg.Chat.ID, msg.MessageID, c.maxUserID, payload, maxAttempts)
		if err != nil {
			return 0, err
		}
		enqueued++
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	return enqueued, nil
}

func (s *Store) ClaimJobs(ctx context.Context, batch int, lease time.Duration) ([]domain.DeliveryJob, error) {
	interval := fmt.Sprintf("%d seconds", int(lease.Seconds()))
	rows, err := s.pool.Query(ctx, `
		WITH cte AS (
			SELECT id
			FROM delivery_jobs
			WHERE status IN ('pending', 'retry')
			  AND available_at <= now()
			  AND (leased_until IS NULL OR leased_until < now())
			ORDER BY available_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE delivery_jobs j
		SET status = 'processing', leased_until = now() + $2::interval, updated_at = now()
		FROM cte
		WHERE j.id = cte.id
		RETURNING j.id, j.route_id, j.telegram_chat_id, j.telegram_message_id, j.max_user_id,
			j.payload_json, j.status, j.attempts, j.max_attempts, j.available_at, j.leased_until,
			j.last_error, j.created_at, j.updated_at
	`, batch, interval)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]domain.DeliveryJob, 0, batch)
	for rows.Next() {
		var j domain.DeliveryJob
		if err := rows.Scan(
			&j.ID,
			&j.RouteID,
			&j.TelegramChatID,
			&j.TelegramMessageID,
			&j.MaxUserID,
			&j.PayloadJSON,
			&j.Status,
			&j.Attempts,
			&j.MaxAttempts,
			&j.AvailableAt,
			&j.LeasedUntil,
			&j.LastError,
			&j.CreatedAt,
			&j.UpdatedAt,
		); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (s *Store) RecordAttempt(ctx context.Context, jobID int64, result domain.DeliveryResult, errorClass, detail string, latencyMs int64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO delivery_attempts (job_id, result, error_class, error_detail, latency_ms)
		VALUES ($1, $2, $3, $4, $5)
	`, jobID, string(result), errorClass, detail, latencyMs)
	return err
}

func (s *Store) MarkJobCompleted(ctx context.Context, jobID int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE delivery_jobs
		SET status = 'completed', leased_until = NULL, last_error = '', updated_at = now()
		WHERE id = $1
	`, jobID)
	return err
}

func (s *Store) MarkJobRetryOrDead(ctx context.Context, jobID int64, nextAttempt int, maxAttempts int, nextRun time.Time, errMsg string) (domain.DeliveryJobStatus, error) {
	if nextAttempt >= maxAttempts {
		_, err := s.pool.Exec(ctx, `
			UPDATE delivery_jobs
			SET status = 'dead_letter', attempts = $2, leased_until = NULL, last_error = $3, updated_at = now()
			WHERE id = $1
		`, jobID, nextAttempt, errMsg)
		return domain.JobDeadLetter, err
	}

	_, err := s.pool.Exec(ctx, `
		UPDATE delivery_jobs
		SET status = 'retry', attempts = $2, available_at = $3, leased_until = NULL, last_error = $4, updated_at = now()
		WHERE id = $1
	`, jobID, nextAttempt, nextRun, errMsg)
	if err != nil {
		return "", err
	}
	return domain.JobRetry, nil
}

func (s *Store) RequeueStaleProcessing(ctx context.Context) (int64, error) {
	res, err := s.pool.Exec(ctx, `
		UPDATE delivery_jobs
		SET status = 'retry', leased_until = NULL, available_at = now(), updated_at = now()
		WHERE status = 'processing' AND leased_until < now()
	`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected(), nil
}

func (s *Store) CleanupRetention(ctx context.Context, jobsDays, dedupeDays, payloadHours int) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE delivery_jobs
		SET payload_json = '{}'::jsonb,
			updated_at = now()
		WHERE status = 'completed'
		  AND payload_json <> '{}'::jsonb
		  AND updated_at < now() - make_interval(hours => $1)
	`, payloadHours)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, `DELETE FROM delivery_attempts WHERE created_at < now() - make_interval(days => $1)`, jobsDays)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM delivery_jobs WHERE status IN ('completed', 'dead_letter') AND updated_at < now() - make_interval(days => $1)`, jobsDays)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM dedupe_records WHERE expires_at < now() OR created_at < now() - make_interval(days => $1)`, dedupeDays)
	if err != nil {
		return err
	}
	return nil
}

func (s *Store) GetQueueStats(ctx context.Context) (QueueStats, error) {
	var st QueueStats
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM delivery_jobs WHERE status = 'pending') as pending,
			(SELECT COUNT(*) FROM delivery_jobs WHERE status = 'retry') as retry,
			(SELECT COUNT(*) FROM delivery_jobs WHERE status = 'dead_letter') as dead,
			COALESCE((SELECT EXTRACT(EPOCH FROM (now() - MIN(available_at)))::bigint FROM delivery_jobs WHERE status in ('pending', 'retry')), 0) as oldest
	`).Scan(&st.PendingDepth, &st.RetryDepth, &st.DeadLetterDepth, &st.OldestPendingAgeS)
	if err != nil {
		return QueueStats{}, err
	}
	return st, nil
}

func (s *Store) ListTelegramGroups(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, telegram_chat_id, title, readiness, readiness_reason, is_enabled, updated_at FROM telegram_groups ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, chatID int64
		var title, readiness, reason string
		var enabled bool
		var updated time.Time
		if err := rows.Scan(&id, &chatID, &title, &readiness, &reason, &enabled, &updated); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id, "chat_id": chatID, "title": title, "readiness": readiness, "reason": reason, "enabled": enabled, "updated_at": updated,
		})
	}
	return out, rows.Err()
}

func (s *Store) ListMaxUsers(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, max_user_id, first_name, last_name, is_active, is_blocked, linked_at, last_delivery_status, updated_at
		FROM max_users
		WHERE is_active = true
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, uid int64
		var firstName, lastName string
		var active, blocked bool
		var linked, updated time.Time
		var last string
		if err := rows.Scan(&id, &uid, &firstName, &lastName, &active, &blocked, &linked, &last, &updated); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id":          id,
			"max_user_id": uid,
			"first_name":  firstName,
			"last_name":   lastName,
			"active":      active,
			"blocked":     blocked,
			"linked_at":   linked,
			"last":        last,
			"updated_at":  updated,
		})
	}
	return out, rows.Err()
}

func (s *Store) ListInvites(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, scope_type, scope_id, expires_at, used_at, revoked_at, single_use, created_at, COALESCE(metadata->>'raw_code', '') FROM invites ORDER BY id DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var scopeType, scopeID string
		var exp, created time.Time
		var used, revoked *time.Time
		var single bool
		var rawCode string
		if err := rows.Scan(&id, &scopeType, &scopeID, &exp, &used, &revoked, &single, &created, &rawCode); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id":         id,
			"scope":      scopeType + ":" + scopeID,
			"expires_at": exp,
			"used_at":    used,
			"revoked_at": revoked,
			"single_use": single,
			"created_at": created,
			"raw_code":   rawCode,
		})
	}
	return out, rows.Err()
}

func (s *Store) ListRoutes(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			r.id,
			r.telegram_chat_id,
			r.max_user_id,
			r.enabled,
			r.filter_mode,
			r.ignore_bot_messages,
			r.last_delivery_status,
			r.updated_at,
			tg.title,
			COALESCE(mu.first_name, ''),
			COALESCE(mu.last_name, '')
		FROM routes r
		LEFT JOIN telegram_groups tg ON tg.telegram_chat_id = r.telegram_chat_id
		LEFT JOIN max_users mu ON mu.max_user_id = r.max_user_id
		ORDER BY r.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, chatID, userID int64
		var enabled, ignore bool
		var filter, status, groupTitle, firstName, lastName string
		var updated time.Time
		if err := rows.Scan(&id, &chatID, &userID, &enabled, &filter, &ignore, &status, &updated, &groupTitle, &firstName, &lastName); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id":          id,
			"chat_id":     chatID,
			"max_user_id": userID,
			"first_name":  firstName,
			"last_name":   lastName,
			"enabled":     enabled,
			"filter":      filter,
			"ignore_bots": ignore,
			"status":      status,
			"updated_at":  updated,
			"group_title": groupTitle,
		})
	}
	return out, rows.Err()
}

func (s *Store) ListQueue(ctx context.Context, status string, limit int) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			j.id,
			j.route_id,
			j.telegram_chat_id,
			j.telegram_message_id,
			j.max_user_id,
			j.status,
			j.attempts,
			j.max_attempts,
			j.available_at,
			j.last_error,
			j.updated_at,
			COALESCE(mu.first_name, ''),
			COALESCE(mu.last_name, '')
		FROM delivery_jobs j
		LEFT JOIN max_users mu ON mu.max_user_id = j.max_user_id
		WHERE ($1 = '' OR status = $1)
		ORDER BY j.available_at ASC
		LIMIT $2
	`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, routeID, chatID, msgID, userID int64
		var jobStatus, lastErr, firstName, lastName string
		var attempts, maxAttempts int
		var available, updated time.Time
		if err := rows.Scan(&id, &routeID, &chatID, &msgID, &userID, &jobStatus, &attempts, &maxAttempts, &available, &lastErr, &updated, &firstName, &lastName); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id":          id,
			"route_id":    routeID,
			"chat_id":     chatID,
			"message_id":  msgID,
			"max_user_id": userID,
			"first_name":  firstName,
			"last_name":   lastName,
			"status":      jobStatus,
			"attempts":    attempts,
			"max_attempts": maxAttempts,
			"available_at": available,
			"last_error":   lastErr,
			"updated_at":   updated,
		})
	}
	return out, rows.Err()
}

func (s *Store) RetryJobNow(ctx context.Context, jobID int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE delivery_jobs SET status = 'retry', available_at = now(), leased_until = NULL, updated_at = now() WHERE id = $1`, jobID)
	return err
}

func (s *Store) ListRecentEvents(ctx context.Context, limit int) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, level, source, message, created_at FROM app_events ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var level, source, message string
		var created time.Time
		if err := rows.Scan(&id, &level, &source, &message, &created); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "level": level, "source": source, "message": message, "created_at": created})
	}
	return out, rows.Err()
}

func (s *Store) LogEvent(ctx context.Context, level, source, message string) {
	_, _ = s.pool.Exec(ctx, `INSERT INTO app_events(level, source, message) VALUES ($1, $2, $3)`, level, source, message)
}

func (s *Store) UpdateRouteDeliveryStatus(ctx context.Context, routeID int64, status, detail string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE routes
		SET last_delivery_status = $2,
			last_delivery_error = $3,
			updated_at = now()
		WHERE id = $1
	`, routeID, status, detail)
	return err
}

func (s *Store) CloneRouteToUser(ctx context.Context, sourceRouteID, maxUserID int64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO routes (telegram_chat_id, max_user_id, enabled, filter_mode, ignore_bot_messages)
		SELECT telegram_chat_id, $2, true, filter_mode, ignore_bot_messages
		FROM routes
		WHERE id = $1
		ON CONFLICT (telegram_chat_id, max_user_id)
		DO UPDATE SET updated_at = now()
	`, sourceRouteID, maxUserID)
	return err
}

func (s *Store) EnsureAttemptPartitions(ctx context.Context, monthsAhead int) error {
	_, err := s.pool.Exec(ctx, `SELECT ensure_delivery_attempt_partitions($1)`, monthsAhead)
	return err
}

func (s *Store) CountCore(ctx context.Context) (groups, users, routes int64, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM telegram_groups),
			(SELECT COUNT(*) FROM max_users WHERE is_active = true),
			(SELECT COUNT(*) FROM routes)
	`).Scan(&groups, &users, &routes)
	return
}

func (s *Store) SetUserBlocked(ctx context.Context, maxUserID int64, blocked bool) error {
	_, err := s.pool.Exec(ctx, `UPDATE max_users SET is_blocked = $2, updated_at = now() WHERE max_user_id = $1`, maxUserID, blocked)
	return err
}

func (s *Store) RemoveUser(ctx context.Context, maxUserID int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE max_users SET is_active = false, updated_at = now() WHERE max_user_id = $1`, maxUserID)
	return err
}

func (s *Store) RemoveGroup(ctx context.Context, chatID int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE telegram_groups SET is_enabled = false, updated_at = now() WHERE telegram_chat_id = $1`, chatID)
	return err
}

func (s *Store) DeleteRoute(ctx context.Context, routeID int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM routes WHERE id = $1`, routeID)
	return err
}

func (s *Store) ClearOldCompleted(ctx context.Context, olderThanDays int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM delivery_jobs WHERE status = 'completed' AND updated_at < now() - make_interval(days => $1)`, olderThanDays)
	return err
}

func (s *Store) ClearCompletedNow(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM delivery_jobs WHERE status = 'completed'`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
