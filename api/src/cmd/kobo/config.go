package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

type Config struct {
	DatabaseURL string
	Listen      string
	ClaudeBin   string
	LogDir      string
	Settings    usecase.Settings
}

// envReader reads KOBO_* variables, collecting the first error.
type envReader struct {
	err error
}

func (r *envReader) str(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	if def == "" && r.err == nil {
		r.err = fmt.Errorf("%s is required", key)
	}
	return def
}

func (r *envReader) duration(key, def string) time.Duration {
	d, err := time.ParseDuration(r.str(key, def))
	if err != nil && r.err == nil {
		r.err = fmt.Errorf("%s: %w", key, err)
	}
	return d
}

func (r *envReader) int(key, def string) int {
	n, err := strconv.Atoi(r.str(key, def))
	if err != nil && r.err == nil {
		r.err = fmt.Errorf("%s: %w", key, err)
	}
	return n
}

func LoadConfig() (Config, error) {
	home, _ := os.UserHomeDir()
	r := &envReader{}
	c := Config{
		DatabaseURL: r.str("KOBO_DATABASE_URL", ""),
		Listen:      r.str("KOBO_LISTEN", "127.0.0.1:8787"),
		ClaudeBin:   r.str("KOBO_CLAUDE_BIN", "claude"),
		LogDir:      r.str("KOBO_LOG_DIR", home+"/.kobo/logs"),
		Settings: usecase.Settings{
			ProjectsDir:     r.str("KOBO_PROJECTS_DIR", home+"/programF"),
			WorktreesDir:    r.str("KOBO_WORKTREES_DIR", home+"/.kobo/worktrees"),
			MaxReviewRounds: r.int("KOBO_MAX_REVIEW_ROUNDS", "2"),
			DevTimeout:      r.duration("KOBO_DEV_TIMEOUT", "60m"),
			ReviewTimeout:   r.duration("KOBO_REVIEW_TIMEOUT", "30m"),
			MaxAutoRetries:  r.int("KOBO_MAX_AUTO_RETRIES", "2"),
			RetryDelay:      r.duration("KOBO_RETRY_DELAY", "1m"),
		},
	}
	window, err := usecase.ParseOffPeakWindow(r.str("KOBO_OFF_PEAK", "01:00-07:00"))
	if err != nil && r.err == nil {
		r.err = err
	}
	c.Settings.OffPeak = window
	return c, r.err
}

// AgentURL is where the AI, running on this machine, reaches kobo's agent MCP
// server. Listening on all interfaces (0.0.0.0, ::, or no host) is reached
// through loopback: the MCP server rejects a Host header such as 0.0.0.0.
func (c Config) AgentURL() (string, error) {
	host, port, err := net.SplitHostPort(c.Listen)
	if err != nil {
		return "", fmt.Errorf("KOBO_LISTEN: %w", err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + "/mcp/agent", nil
}
