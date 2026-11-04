// Package irepository defines what the usecases need from the outside world.
package irepository

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("not found")

// Transactor runs fn in one transaction. Repository calls made with the ctx
// passed to fn take part in it.
type Transactor interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
	// LockQueue serializes taking tasks out of the queue until the transaction ends.
	LockQueue(ctx context.Context) error
}
