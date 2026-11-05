// Package repository talks to everything outside kobo: Postgres (DB), the
// target repositories through git and GitHub (RepoHost), and the AI.
package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

// DB is kobo's Postgres database. It implements usecase.DB.
type DB struct {
	gorm *gorm.DB
}

func NewDB(ctx context.Context, url string) (*DB, error) {
	g, err := gorm.Open(postgres.Open(url), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
		Logger:         logger.Discard,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	sqlDB, err := g.DB()
	if err != nil {
		return nil, err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	return &DB{gorm: g}, nil
}

func (d *DB) Close() error {
	sqlDB, err := d.gorm.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

type txKey struct{}

// conn returns the transaction carried by ctx, or the pool.
func (d *DB) conn(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx.WithContext(ctx)
	}
	return d.gorm.WithContext(ctx)
}

// InTx runs fn in one transaction. Calls made with the ctx passed to fn take
// part in it; a nested InTx joins the outer transaction.
func (d *DB) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(ctx)
	}
	return d.gorm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// notFound turns GORM's missing-row error into domain.ErrNotFound.
func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	return err
}

// TruncateAll empties every table. Only the tests use it.
func (d *DB) TruncateAll(ctx context.Context) error {
	if err := d.gorm.WithContext(ctx).Exec("TRUNCATE event, review_round, session, pull_request, task, project RESTART IDENTITY CASCADE").Error; err != nil {
		return err
	}
	return d.gorm.WithContext(ctx).Exec("UPDATE queue_state SET paused = false").Error
}
