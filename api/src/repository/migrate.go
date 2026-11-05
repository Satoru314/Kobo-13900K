package repository

import (
	"context"
	"fmt"
	"io/fs"
	"slices"
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
func (d *DB) Migrate(ctx context.Context) error {
	if err := d.gorm.WithContext(ctx).Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`).Error; err != nil {
		return err
	}
	names, err := migrationNames()
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := d.applyMigration(ctx, name); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}

func (d *DB) applyMigration(ctx context.Context, name string) error {
	return d.InTx(ctx, func(ctx context.Context) error {
		var applied bool
		if err := d.conn(ctx).Raw("SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = ?)", name).
			Scan(&applied).Error; err != nil {
			return err
		}
		if applied {
			return nil
		}
		sql, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return err
		}
		if err := d.conn(ctx).Exec(string(sql)).Error; err != nil {
			return err
		}
		return d.conn(ctx).Exec("INSERT INTO schema_migrations (version) VALUES (?)", name).Error
	})
}

// PendingMigrations lists the embedded migrations that have not been applied.
// kobo refuses to start while any are pending; make migrate applies them.
func (d *DB) PendingMigrations(ctx context.Context) ([]string, error) {
	names, err := migrationNames()
	if err != nil {
		return nil, err
	}
	var exists bool
	if err := d.gorm.WithContext(ctx).Raw("SELECT to_regclass('schema_migrations') IS NOT NULL").Scan(&exists).Error; err != nil {
		return nil, err
	}
	if !exists {
		return names, nil
	}
	var applied []string
	if err := d.gorm.WithContext(ctx).Raw("SELECT version FROM schema_migrations").Scan(&applied).Error; err != nil {
		return nil, err
	}
	var pending []string
	for _, name := range names {
		if !slices.Contains(applied, name) {
			pending = append(pending, name)
		}
	}
	return pending, nil
}
