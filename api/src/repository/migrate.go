package repository

import (
	"context"
	"fmt"
	"io/fs"
	"sort"

	"github.com/Satoru314/Kobo-13900K/api/migrations"
)

// migrationNames lists the embedded SQL files in the order they are applied.
func migrationNames() ([]string, error) {
	names, err := fs.Glob(migrations.FS, "*.sql")
	sort.Strings(names)
	return names, err
}

// Migrate applies the embedded SQL files that have not been applied yet, in name
// order. Only make migrate (cmd/migrate) and the tests call it.
func (p *Postgres) Migrate(ctx context.Context) error {
	if _, err := p.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	names, err := migrationNames()
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := p.applyMigration(ctx, name); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}

func (p *Postgres) applyMigration(ctx context.Context, name string) error {
	return p.InTx(ctx, func(ctx context.Context) error {
		var exists bool
		if err := p.conn(ctx).QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", name).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return nil
		}
		sql, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return err
		}
		if _, err := p.conn(ctx).Exec(ctx, string(sql)); err != nil {
			return err
		}
		_, err = p.conn(ctx).Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", name)
		return err
	})
}

// PendingMigrations lists the embedded migrations that have not been applied.
// kobo refuses to start while any are pending; make migrate applies them.
func (p *Postgres) PendingMigrations(ctx context.Context) ([]string, error) {
	names, err := migrationNames()
	if err != nil {
		return nil, err
	}
	var exists bool
	if err := p.pool.QueryRow(ctx, "SELECT to_regclass('schema_migrations') IS NOT NULL").Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return names, nil
	}
	var pending []string
	for _, name := range names {
		var applied bool
		if err := p.pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", name).Scan(&applied); err != nil {
			return nil, err
		}
		if !applied {
			pending = append(pending, name)
		}
	}
	return pending, nil
}
