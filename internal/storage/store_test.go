package storage

import (
	"context"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
)

func TestRemoveUser_DisablesUserAndRoutesInTransaction(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock: %v", err)
	}
	defer mock.Close()

	store := &Store{pool: mock}

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE max_users SET is_active = false, updated_at = now\(\) WHERE max_user_id = \$1`).
		WithArgs(int64(42)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec(`UPDATE routes SET enabled = false, updated_at = now\(\) WHERE max_user_id = \$1`).
		WithArgs(int64(42)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 3))
	mock.ExpectCommit()

	if err := store.RemoveUser(context.Background(), 42); err != nil {
		t.Fatalf("RemoveUser returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestListRoutes_FiltersOutInactiveUsers(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock: %v", err)
	}
	defer mock.Close()

	store := &Store{pool: mock}
	now := time.Now().UTC()

	mock.ExpectQuery(`(?s)FROM routes r.*JOIN max_users mu ON mu.max_user_id = r.max_user_id.*WHERE mu.is_active = true`).
		WillReturnRows(
			pgxmock.NewRows([]string{
				"id",
				"telegram_chat_id",
				"max_user_id",
				"enabled",
				"filter_mode",
				"ignore_bot_messages",
				"last_delivery_status",
				"updated_at",
				"title",
				"coalesce",
			}).AddRow(
				int64(11),
				int64(-1001),
				int64(42),
				true,
				"all",
				true,
				"ok",
				now,
				"Группа A",
				"Иван Петров",
			),
		)

	rows, err := store.ListRoutes(context.Background())
	if err != nil {
		t.Fatalf("ListRoutes returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 route row, got %d", len(rows))
	}
	if rows[0]["max_user_id"] != int64(42) {
		t.Fatalf("unexpected max_user_id: %v", rows[0]["max_user_id"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestListInvites_ReturnsMaxFullNameFromMetadata(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock: %v", err)
	}
	defer mock.Close()

	store := &Store{pool: mock}
	now := time.Now().UTC()

	mock.ExpectQuery(`SELECT id, scope_type, scope_id, expires_at, used_at, revoked_at, single_use, created_at, COALESCE\(metadata->>'raw_code', ''\), COALESCE\(metadata->>'max_full_name', ''\) FROM invites ORDER BY id DESC LIMIT 200`).
		WillReturnRows(
			pgxmock.NewRows([]string{
				"id",
				"scope_type",
				"scope_id",
				"expires_at",
				"used_at",
				"revoked_at",
				"single_use",
				"created_at",
				"raw_code",
				"max_full_name",
			}).AddRow(
				int64(9),
				"entity",
				"general",
				now.Add(24*time.Hour),
				nil,
				nil,
				true,
				now,
				"MB-ABC123",
				"Иван Петров",
			),
		)

	rows, err := store.ListInvites(context.Background())
	if err != nil {
		t.Fatalf("ListInvites returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 invite row, got %d", len(rows))
	}
	if got := rows[0]["max_full_name"]; got != "Иван Петров" {
		t.Fatalf("unexpected max_full_name: %v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
