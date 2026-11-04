// Command kobo is the task queue server: REST for the GUI, MCP for the AI, and the workers.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/controller/mcp"
	"github.com/Satoru314/Kobo-13900K/api/src/controller/rest"
	"github.com/Satoru314/Kobo-13900K/api/src/controller/worker"
	"github.com/Satoru314/Kobo-13900K/api/src/domain"
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

	// repository
	pg, err := repository.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pg.Close()
	if err := pg.Migrate(ctx); err != nil {
		return err
	}
	tasks := repository.NewTaskRepository(pg)
	projectRepo := repository.NewProjectRepository(pg)
	sessions := repository.NewSessionRepository(pg)
	prs := repository.NewPullRequestRepository(pg)
	rounds := repository.NewReviewRoundRepository(pg)
	events := repository.NewEventRepository(pg)
	git := repository.NewGit()
	github := repository.NewGitHub()
	runner := repository.NewClaudeRunner(cfg.ClaudeBin, "http://"+cfg.Listen+"/mcp/agent", cfg.LogDir)

	// usecase
	sig := usecase.NewQueueSignal()
	active := usecase.NewActiveRun()
	projects := usecase.NewProjectUsecase(projectRepo, git, cfg.Settings)
	taskUC := usecase.NewTaskUsecase(pg, tasks, prs, rounds, events, projects, sig)
	queueUC := usecase.NewQueueUsecase(pg, tasks, sessions, prs, rounds, events, projects, runner, git,
		domain.Queue{OffPeak: cfg.OffPeak}, cfg.Settings, active, sig)
	completionUC := usecase.NewCompletionUsecase(pg, tasks, sessions, prs, rounds, events, projects, git, cfg.Settings, active, sig)
	ciUC := usecase.NewCIUsecase(pg, tasks, prs, rounds, events, projects, github, cfg.Settings, sig)
	prUC := usecase.NewPRUsecase(pg, tasks, prs, events, projects, github)

	// controller
	mux := http.NewServeMux()
	rest.NewRouter(rest.NewTaskHandler(taskUC), rest.NewProjectHandler(projects),
		rest.NewQueueHandler(queueUC), rest.NewHealthHandler()).Register(mux)
	mcpServer := mcp.NewServer(mcp.NewTaskTools(taskUC), mcp.NewProjectTools(projects),
		mcp.NewQueueTools(queueUC), mcp.NewCompletionTools(completionUC))
	mux.Handle("/mcp", mcpServer.UserHandler())
	mux.Handle("/mcp/agent", mcpServer.AgentHandler())
	srv := &http.Server{Addr: cfg.Listen, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	var wg sync.WaitGroup
	for _, w := range []interface{ Run(context.Context) }{
		worker.NewQueueWorker(queueUC, sig.C(), 30*time.Second),
		worker.NewPoller("ci", ciUC, 30*time.Second),
		worker.NewPoller("pr", prUC, 60*time.Second),
	} {
		wg.Add(1)
		go func() { defer wg.Done(); w.Run(ctx) }()
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
