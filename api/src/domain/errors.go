// Package domain holds kobo's data: one type per table, the values its columns
// take, and the check that a value is one of them. Rules live in usecase.
package domain

import "errors"

// ErrNotFound means the row asked for does not exist.
var ErrNotFound = errors.New("not found")
