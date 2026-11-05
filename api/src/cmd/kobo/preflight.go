package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/repository"
)

const versionTimeout = 15 * time.Second

// Preflight checks the commands and directories kobo needs. It runs before the
// server starts and in make check, so a missing install is a startup failure
// instead of a task that fails after it was taken out of the queue.
func Preflight(ctx context.Context, cfg Config, catalog *repository.AICatalog) error {
	var problems []string
	add := func(err error) {
		if err != nil {
			problems = append(problems, err.Error())
		}
	}

	if info, err := os.Stat(cfg.Settings.ProjectsDir); err != nil {
		add(fmt.Errorf("KOBO_PROJECTS_DIR %s: %w", cfg.Settings.ProjectsDir, err))
	} else if !info.IsDir() {
		add(fmt.Errorf("KOBO_PROJECTS_DIR %s is not a directory", cfg.Settings.ProjectsDir))
	}

	seen := map[string]bool{}
	for _, p := range catalog.Providers() {
		if seen[p.Bin] {
			continue
		}
		seen[p.Bin] = true
		add(version(ctx, p.Bin))
	}
	add(version(ctx, "git"))
	add(version(ctx, "gh"))
	if _, err := exec.LookPath("gh"); err == nil {
		add(runCommand(ctx, "gh", "auth", "status"))
	}
	if cfg.Browser {
		add(executable(cfg.PlaywrightMCP))
	}

	if len(problems) > 0 {
		return fmt.Errorf("setup check failed:\n- %s", strings.Join(problems, "\n- "))
	}
	return nil
}

// version runs name --version to prove the command is installed and usable.
func version(ctx context.Context, name string) error {
	if err := executable(name); err != nil {
		return err
	}
	return runCommand(ctx, name, "--version")
}

// executable checks the path without running it.
func executable(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("%s is not installed or not in PATH: %w", name, err)
	}
	return nil
}

// runCommand runs a short command and reports its output when it fails.
func runCommand(ctx context.Context, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, versionTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		text := strings.TrimSpace(out.String())
		if text == "" {
			return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, text)
	}
	return nil
}
