package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/repository"
)

const versionTimeout = 15 * time.Second

// minimumVersions is the oldest version of each command kobo runs. The command
// names are the binary names; a configured path is compared by its base name.
var minimumVersions = map[string]string{
	"gh":             "2.0.0",  // pr view --json statusCheckRollup
	"git":            "2.17.0", // worktree remove
	"claude":         "1.0.53", // --session-id
	"opencode":       "2.0.19", // run --standalone --session, OPENCODE_CONFIG_CONTENT, mcp.servers
	"playwright-mcp": "0.0.25", // --headless --browser chromium --isolated --output-dir
}

// versionPattern finds a numeric version such as 2.45.0 in --version output.
// Extra numeric components are kept, so 2.43.0.1 compares as 2.43.0.1.
var versionPattern = regexp.MustCompile(`\d+\.\d+\.\d+(?:\.\d+)*`)

// Preflight checks the commands and directories kobo needs. It runs before the
// server starts and in make check, so a missing install or an old command is a
// startup failure instead of a task that fails after it was taken out of the
// queue.
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
		add(version(ctx, cfg.PlaywrightMCP))
	}

	if len(problems) > 0 {
		return fmt.Errorf("setup check failed:\n- %s", strings.Join(problems, "\n- "))
	}
	return nil
}

// version runs name --version and compares the reported version with the
// minimum for that command.
func version(ctx context.Context, name string) error {
	if err := executable(name); err != nil {
		return err
	}
	out, err := commandOutput(ctx, name, "--version")
	if err != nil {
		return err
	}
	return checkVersion(name, out)
}

// checkVersion reads the first version from --version output and compares it
// with the minimum for name. A command missing from the table still has to
// print a readable version, but has no minimum.
func checkVersion(name, output string) error {
	found, err := parseVersion(output)
	if err != nil {
		return fmt.Errorf("%s --version: %w", name, err)
	}
	required, ok := minimumVersions[filepath.Base(name)]
	if !ok {
		return nil
	}
	cmp, err := compareVersions(found, required)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if cmp < 0 {
		return fmt.Errorf("%s: version %s is older than required %s", name, found, required)
	}
	return nil
}

// parseVersion extracts the first x.y.z version from --version output.
func parseVersion(output string) (string, error) {
	found := versionPattern.FindString(output)
	if found == "" {
		return "", fmt.Errorf("cannot read a version from %q", strings.TrimSpace(output))
	}
	return found, nil
}

// compareVersions compares two versions component by component and returns -1,
// 0 or 1.
func compareVersions(found, required string) (int, error) {
	a, err := parseVersion(found)
	if err != nil {
		return 0, err
	}
	b, err := parseVersion(required)
	if err != nil {
		return 0, err
	}
	ap, err := versionParts(a)
	if err != nil {
		return 0, err
	}
	bp, err := versionParts(b)
	if err != nil {
		return 0, err
	}
	for i := 0; i < len(ap) || i < len(bp); i++ {
		var x, y int
		if i < len(ap) {
			x = ap[i]
		}
		if i < len(bp) {
			y = bp[i]
		}
		if x < y {
			return -1, nil
		}
		if x > y {
			return 1, nil
		}
	}
	return 0, nil
}

func versionParts(v string) ([]int, error) {
	text := strings.Split(v, ".")
	parts := make([]int, len(text))
	for i, p := range text {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid version %q: %w", v, err)
		}
		parts[i] = n
	}
	return parts, nil
}

// executable checks the path without running it.
func executable(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("%s is not installed or not in PATH: %w", name, err)
	}
	return nil
}

// commandOutput runs a short command and returns its combined output. A failed
// command returns its output in the error.
func commandOutput(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, versionTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		text := strings.TrimSpace(out.String())
		if text == "" {
			return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, text)
	}
	return strings.TrimSpace(out.String()), nil
}

// runCommand runs a short command and reports its output when it fails.
func runCommand(ctx context.Context, name string, args ...string) error {
	_, err := commandOutput(ctx, name, args...)
	return err
}
