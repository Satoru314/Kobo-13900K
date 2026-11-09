package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseVersion(t *testing.T) {
	for _, c := range []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "gh",
			output: "gh version 2.45.0 (2025-07-18 Ubuntu 2.45.0-1ubuntu0.3)\nhttps://github.com/cli/cli/releases/tag/v2.45.0",
			want:   "2.45.0",
		},
		{name: "git", output: "git version 2.43.0", want: "2.43.0"},
		{name: "git windows", output: "git version 2.43.0.windows.1", want: "2.43.0"},
		{name: "claude", output: "2.1.318 (Claude Code)", want: "2.1.318"},
		{name: "opencode", output: "opencode v2.2.5", want: "2.2.5"},
		{name: "playwright-mcp", output: "Version 0.0.84", want: "0.0.84"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseVersion(c.output)
			if err != nil {
				t.Fatalf("parseVersion(%q): %v", c.output, err)
			}
			if got != c.want {
				t.Errorf("parseVersion(%q) = %q, want %q", c.output, got, c.want)
			}
		})
	}
}

func TestParseVersionError(t *testing.T) {
	for _, output := range []string{"", "playwright-mcp version unknown", "v2"} {
		if _, err := parseVersion(output); err == nil {
			t.Errorf("parseVersion(%q) = nil error, want error", output)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	for _, c := range []struct {
		found    string
		required string
		want     int
	}{
		{found: "2.45.0", required: "2.0.0", want: 1},
		{found: "2.0.0", required: "2.0.0", want: 0},
		{found: "1.0.52", required: "1.0.53", want: -1},
		{found: "2.0.19", required: "2.0.19", want: 0},
		{found: "2.1.318", required: "1.0.53", want: 1},
		{found: "0.0.24", required: "0.0.25", want: -1},
		{found: "10.0.0", required: "9.9.9", want: 1},
		{found: "2.43.0.windows.1", required: "2.17.0", want: 1},
	} {
		t.Run(c.found+"_"+c.required, func(t *testing.T) {
			got, err := compareVersions(c.found, c.required)
			if err != nil {
				t.Fatalf("compareVersions(%q, %q): %v", c.found, c.required, err)
			}
			if got != c.want {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", c.found, c.required, got, c.want)
			}
		})
	}
}

func TestCheckVersion(t *testing.T) {
	for _, c := range []struct {
		name    string
		command string
		role    string
		output  string
		wantErr []string
	}{
		{name: "gh", command: "gh", role: "gh", output: "gh version 2.45.0 (2025-07-18 Ubuntu 2.45.0-1ubuntu0.3)"},
		{name: "git", command: "git", role: "git", output: "git version 2.17.0"},
		{name: "claude", command: "claude", role: "claude", output: "1.0.53 (Claude Code)"},
		{name: "opencode", command: "opencode", role: "opencode", output: "opencode v2.0.19"},
		{name: "playwright-mcp", command: "playwright-mcp", role: "playwright-mcp", output: "Version 0.0.25"},
		{
			name:    "old gh",
			command: "gh",
			role:    "gh",
			output:  "gh version 1.14.0 (2021-07-20)",
			wantErr: []string{"gh", "1.14.0", "2.0.0"},
		},
		{
			name:    "old git",
			command: "git",
			role:    "git",
			output:  "git version 2.16.0",
			wantErr: []string{"git", "2.16.0", "2.17.0"},
		},
		{
			name:    "old claude",
			command: "claude",
			role:    "claude",
			output:  "1.0.52 (Claude Code)",
			wantErr: []string{"claude", "1.0.52", "1.0.53"},
		},
		{
			name:    "old opencode",
			command: "opencode",
			role:    "opencode",
			output:  "opencode v2.0.18",
			wantErr: []string{"opencode", "2.0.18", "2.0.19"},
		},
		{
			name:    "opencode path",
			command: "/usr/local/bin/opencode",
			role:    "opencode",
			output:  "opencode v2.0.18",
			wantErr: []string{"/usr/local/bin/opencode", "2.0.18", "2.0.19"},
		},
		{
			name:    "old playwright-mcp",
			command: "playwright-mcp",
			role:    "playwright-mcp",
			output:  "Version 0.0.24",
			wantErr: []string{"playwright-mcp", "0.0.24", "0.0.25"},
		},
		{
			name:    "renamed old playwright-mcp",
			command: "mcp-server-playwright",
			role:    "playwright-mcp",
			output:  "Version 0.0.10",
			wantErr: []string{"mcp-server-playwright", "0.0.10", "0.0.25"},
		},
		{
			name:    "renamed old opencode",
			command: "opencode-old",
			role:    "opencode",
			output:  "opencode v0.1.0",
			wantErr: []string{"opencode-old", "0.1.0", "2.0.19"},
		},
		{
			name:    "renamed old claude",
			command: "claude-code",
			role:    "claude",
			output:  "0.2.0 (Claude Code)",
			wantErr: []string{"claude-code", "0.2.0", "1.0.53"},
		},
		{
			name:    "renamed new playwright-mcp",
			command: "mcp-server-playwright",
			role:    "playwright-mcp",
			output:  "Version 0.0.84",
		},
		{
			name:    "unknown role",
			command: "opencode-old",
			role:    "unknown",
			output:  "opencode v0.1.0",
			wantErr: []string{"opencode-old", "no minimum version", "unknown"},
		},
		{
			name:    "unreadable",
			command: "opencode",
			role:    "opencode",
			output:  "opencode version unknown",
			wantErr: []string{"opencode", "cannot read a version"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := checkVersion(c.command, c.role, c.output)
			if len(c.wantErr) == 0 {
				if err != nil {
					t.Fatalf("checkVersion(%q, %q, %q): %v", c.command, c.role, c.output, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("checkVersion(%q, %q, %q) = nil error, want %v", c.command, c.role, c.output, c.wantErr)
			}
			for _, want := range c.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("checkVersion(%q, %q, %q) = %q, want it to contain %q", c.command, c.role, c.output, err, want)
				}
			}
		})
	}
}

func TestVersionChecksRenamedBinary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode-old")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho 'opencode v0.1.0'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := version(context.Background(), "opencode-old", "opencode")
	if err == nil {
		t.Fatal("version() = nil error, want an error for an old renamed binary")
	}
	for _, want := range []string{"opencode-old", "0.1.0", "2.0.19"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("version() = %q, want it to contain %q", err, want)
		}
	}
}

func TestMinimumVersionsAreValid(t *testing.T) {
	for name, v := range minimumVersions {
		if _, err := parseVersion(v); err != nil {
			t.Errorf("minimumVersions[%q] = %q: %v", name, v, err)
		}
	}
}
