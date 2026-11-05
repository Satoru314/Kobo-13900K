package repository

import (
	"encoding/json"
	"fmt"
)

// mcpServer is one MCP server in a provider-neutral shape. URL means a remote
// server; Command means a local stdio server.
type mcpServer struct {
	Name    string
	URL     string
	Command []string
	Env     map[string]string
}

func agentServer(url string) mcpServer {
	return mcpServer{Name: "kobo-agent", URL: url}
}

// claudeMCPConfig builds the JSON passed to Claude Code's --mcp-config.
func claudeMCPConfig(servers []mcpServer) (string, error) {
	entries := map[string]any{}
	for _, s := range servers {
		switch {
		case s.URL != "":
			entries[s.Name] = map[string]any{"type": "http", "url": s.URL}
		case len(s.Command) > 0:
			e := map[string]any{"type": "stdio", "command": s.Command[0], "args": s.Command[1:]}
			if len(s.Env) > 0 {
				e["env"] = s.Env
			}
			entries[s.Name] = e
		}
	}
	b, err := json.Marshal(map[string]any{"mcpServers": entries})
	return string(b), err
}

// openCodeMCPConfig builds the config passed to opencode in
// OPENCODE_CONFIG_CONTENT. opencode merges it with the repository's own
// opencode.json, so nothing is written into the worktree.
func openCodeMCPConfig(servers []mcpServer) (string, error) {
	entries := map[string]any{}
	for _, s := range servers {
		switch {
		case s.URL != "":
			entries[s.Name] = map[string]any{"type": "remote", "url": s.URL}
		case len(s.Command) > 0:
			e := map[string]any{"type": "local", "command": s.Command}
			if len(s.Env) > 0 {
				e["environment"] = s.Env
			}
			entries[s.Name] = e
		default:
			return "", fmt.Errorf("MCP server %q has neither a URL nor a command", s.Name)
		}
	}
	b, err := json.Marshal(map[string]any{"mcp": map[string]any{"servers": entries}})
	return string(b), err
}
