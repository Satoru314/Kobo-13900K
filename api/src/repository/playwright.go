package repository

import (
	"os"
	"path/filepath"
)

// playwright gives the AI a browser through an already installed Playwright MCP
// server. kobo never installs it; the startup check makes sure the binary is there.
// Runs are serial, so the output directory can be per run without a lock.
type playwright struct {
	bin  string
	path string
}

func NewPlaywright(bin string) *playwright {
	return &playwright{bin: bin, path: os.Getenv("PATH")}
}

// server returns the MCP server entry. outputDir keeps its snapshots and logs out
// of the worktree, where the AI could commit them by accident.
func (p *playwright) server(outputDir string) mcpServer {
	return mcpServer{
		Name: "kobo-playwright",
		Command: []string{p.bin,
			"--headless",
			"--browser", "chromium", // the default is the system's Google Chrome
			"--isolated", // no profile shared between tasks, and no stale profile lock
			"--output-dir", outputDir,
		},
		Env: map[string]string{"PATH": p.path},
	}
}

// outputDir is where one run's browser output goes.
func (p *playwright) outputDir(logDir, logName string) string {
	return filepath.Join(logDir, "playwright", logName)
}
