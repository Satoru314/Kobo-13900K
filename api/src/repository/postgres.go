// Package repository talks to everything outside kobo: Postgres, the AI, GitHub and git.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Satoru314/Kobo-13900K/api/src/usecase/irepository"
)

// querier is what both the pool and a transaction can do.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txKey struct{}

// Postgres holds the connection pool and implements irepository.Transactor.
type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(ctx context.Context, url string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Close() {
	p.pool.Close()
}

// conn returns the transaction carried by ctx, or the pool.
func (p *Postgres) conn(ctx context.Context) querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return p.pool
}

func (p *Postgres) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// queueLockKey is an arbitrary constant for the advisory lock that guards the queue.
const queueLockKey = 0x6b6f626f

func (p *Postgres) LockQueue(ctx context.Context) error {
	_, err := p.conn(ctx).Exec(ctx, "SELECT pg_advisory_xact_lock($1)", queueLockKey)
	return err
}

// TruncateAll empties every table. Only the tests use it.
func (p *Postgres) TruncateAll(ctx context.Context) error {
	if _, err := p.pool.Exec(ctx, "TRUNCATE event, review_round, session, pull_request, task, project RESTART IDENTITY CASCADE"); err != nil {
		return err
	}
	_, err := p.pool.Exec(ctx, "UPDATE queue_state SET paused = false")
	return err
}

// notFound turns pgx's no-rows error into irepository.ErrNotFound.
func (p *Postgres) notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return irepository.ErrNotFound
	}
	return err
}
