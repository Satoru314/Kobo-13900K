// Command kobo is the task queue server: REST for the GUI, MCP for the AI, and the workers.
package main

import (
	"context"
	"errors"
	"flag"
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
	check := flag.Bool("check", false, "check the setup and exit")
	flag.Parse()
	if *check {
		if err := checkSetup(); err != nil {
			slog.Error("setup check failed", "err", err)
			os.Exit(1)
		}
		fmt.Println("setup ok")
		return
	}
	if err := run(); err != nil {
		slog.Error("kobo stopped", "err", err)
		os.Exit(1)
	}
}

type app struct {
	cfg Config
	db  *repository.DB
	uc  *usecase.Usecase
	hub *notify.Hub
}

// setup loads the config and catalog, checks the environment and opens the
// database. Both the server and make check use it, so they check the same things.
func setup(ctx context.Context) (a *app, err error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	catalog, err := repository.LoadAICatalog(cfg.AIConfigPath)
	if err != nil {
		return nil, err
	}
	agentURL, err := cfg.AgentURL()
	if err != nil {
		return nil, err
	}
	db, err := repository.NewDB(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			db.Close()
		}
	}()
	pending, err := db.PendingMigrations(ctx)
	if err != nil {
		return nil, err
	}
	if len(pending) > 0 {
		return nil, fmt.Errorf("migrations not applied: %v; run make migrate", pending)
	}
	if err := Preflight(ctx, cfg, catalog); err != nil {
		return nil, err
	}

	browser := repository.NewPlaywright(cfg.PlaywrightMCP)
	if !cfg.Browser {
		browser = nil
	}
	ais := map[string]usecase.AI{}
	for _, p := range catalog.Providers() {
		switch p.Runner {
		case "claude":
			ais[p.Name] = repository.NewClaudeRunner(p.Bin, agentURL, cfg.LogDir, browser)
		case "opencode":
			ais[p.Name] = repository.NewOpenCodeRunner(p.Bin, agentURL, cfg.LogDir, p.Agent, browser)
		default:
			return nil, fmt.Errorf("AI provider %q: unknown runner %q", p.Name, p.Runner)
		}
	}
	for _, m := range catalog.List() {
		if _, ok := ais[m.Provider]; !ok {
			return nil, fmt.Errorf("AI model %q uses provider %q, which has no runner", m.ID, m.Provider)
		}
	}

	hub := notify.New()
	uc := usecase.New(db, repository.NewRepoHost(), ais, catalog, hub, cfg.Settings)
	if err := uc.ValidateAISetting(ctx); err != nil {
		return nil, err
	}
	return &app{cfg: cfg, db: db, uc: uc, hub: hub}, nil
}

func checkSetup() error {
	ctx := context.Background()
	a, err := setup(ctx)
	if err != nil {
		return err
	}
	return a.db.Close()
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := setup(ctx)
	if err != nil {
		return err
	}
	defer a.db.Close()

	e := echo.New()
	e.HideBanner, e.HidePort = true, true
	rest.Register(e, a.uc, a.hub)
	e.Any("/mcp", echo.WrapHandler(mcp.UserHandler(a.uc)))
	e.Any("/mcp/agent", echo.WrapHandler(mcp.AgentHandler(a.uc)))
	e.GET("/*", echo.WrapHandler(web.Handler()))
	// Requests share ctx, so open /events streams end when kobo shuts down instead of holding Shutdown.
	srv := &http.Server{Addr: a.cfg.Listen, Handler: e, ReadHeaderTimeout: 10 * time.Second,
		BaseContext: func(net.Listener) context.Context { return ctx }}

	var wg sync.WaitGroup
	for _, run := range []func(){
		func() { worker.RunQueue(ctx, a.uc, a.hub.Wake(), 30*time.Second) },
		func() { worker.Poll(ctx, "ci", a.uc.CheckCI, 30*time.Second) },
		func() { worker.Poll(ctx, "pr", a.uc.CheckPullRequests, 60*time.Second) },
	} {
		wg.Add(1)
		go func() { defer wg.Done(); run() }()
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	slog.Info("kobo listening", "addr", a.cfg.Listen)
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
