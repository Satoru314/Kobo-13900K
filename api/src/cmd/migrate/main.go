// Command migrate applies the database migrations. Run it with make migrate.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/Satoru314/Kobo-13900K/api/src/repository"
)

func main() {
	if err := run(); err != nil {
		slog.Error("migrate failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	url := os.Getenv("KOBO_DATABASE_URL")
	if url == "" {
		return errors.New("KOBO_DATABASE_URL is required")
	}
	ctx := context.Background()
	db, err := repository.NewDB(ctx, url)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		return err
	}
	slog.Info("migrations applied")
	return nil
}
