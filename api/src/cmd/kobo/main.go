// Command kobo is the task queue server: REST for the GUI, MCP for the AI, and the workers.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/Satoru314/Kobo-13900K/api/src/controller/mcp"
	"github.com/Satoru314/Kobo-13900K/api/src/controller/rest"
	"github.com/Satoru314/Kobo-13900K/api/src/controller/web"
	"github.com/Satoru314/Kobo-13900K/api/src/controller/worker"
	"github.com/Satoru314/Kobo-13900K/api/src/notify"
	"github.com/Satoru314/Kobo-13900K/api/src/repository"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

func main() {
	if err := run(); err != nil {
		slog.Error("kobo stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := repository.NewDB(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	pending, err := db.PendingMigrations(ctx)
	if err != nil {
		return err
	}
	if len(pending) > 0 {
		return fmt.Errorf("migrations not applied: %v; run make migrate", pending)
	}
	agentURL, err := cfg.AgentURL()
	if err != nil {
		return err
	}
	hub := notify.New()
	uc := usecase.New(db, repository.NewRepoHost(), repository.NewClaudeRunner(cfg.ClaudeBin, agentURL, cfg.LogDir), hub, cfg.Settings)

	e := echo.New()
	e.HideBanner, e.HidePort = true, true
	rest.Register(e, uc, hub)
	e.Any("/mcp", echo.WrapHandler(mcp.UserHandler(uc)))
	e.Any("/mcp/agent", echo.WrapHandler(mcp.AgentHandler(uc)))
	e.GET("/*", echo.WrapHandler(web.Handler()))
	// Requests share ctx, so open /events streams end when kobo shuts down instead of holding Shutdown.
	srv := &http.Server{Addr: cfg.Listen, Handler: e, ReadHeaderTimeout: 10 * time.Second,
		BaseContext: func(net.Listener) context.Context { return ctx }}

	var wg sync.WaitGroup
	for _, run := range []func(){
		func() { worker.RunQueue(ctx, uc, hub.Wake(), 30*time.Second) },
		func() { worker.Poll(ctx, "ci", uc.CheckCI, 30*time.Second) },
		func() { worker.Poll(ctx, "pr", uc.CheckPullRequests, 60*time.Second) },
	} {
		wg.Add(1)
		go func() { defer wg.Done(); run() }()
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	slog.Info("kobo listening", "addr", cfg.Listen)
	select {
	case err := <-errCh:
		stop()
		wg.Wait()
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = srv.Shutdown(shutdownCtx)
	wg.Wait()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
