package storage

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

type MigrationDirection string

const (
	Up   MigrationDirection = "up"
	Down MigrationDirection = "down"
)

type migrationFile struct {
	Version string
	Path    string
}

func (s *Store) RunMigrations(ctx context.Context, dir string, direction MigrationDirection) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)
	`); err != nil {
		return err
	}

	files, err := collectMigrationFiles(dir, direction)
	if err != nil {
		return err
	}

	for _, mf := range files {
		if direction == Up {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, mf.Version).Scan(&exists); err != nil {
				return err
			}
			if exists {
				continue
			}
		}

		raw, err := os.ReadFile(mf.Path)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(raw)); err != nil {
			return fmt.Errorf("migration %s failed: %w", mf.Path, err)
		}

		if direction == Up {
			if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, mf.Version); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, mf.Version); err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

func collectMigrationFiles(dir string, direction MigrationDirection) ([]migrationFile, error) {
	out := make([]migrationFile, 0)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if direction == Up && strings.HasSuffix(name, ".up.sql") {
			parts := strings.SplitN(name, "_", 2)
			if len(parts) < 2 {
				return nil
			}
			out = append(out, migrationFile{Version: parts[0], Path: path})
		}
		if direction == Down && strings.HasSuffix(name, ".down.sql") {
			parts := strings.SplitN(name, "_", 2)
			if len(parts) < 2 {
				return nil
			}
			out = append(out, migrationFile{Version: parts[0], Path: path})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool {
		if direction == Up {
			return out[i].Version < out[j].Version
		}
		return out[i].Version > out[j].Version
	})
	return out, nil
}
