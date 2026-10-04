package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ServerName is the plugin's MCP server, as both manifests name it.
const ServerName = "atlas"

// Server is the command a host runs for the plugin's MCP server, after the host's own
// substitutions.
type Server struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	EnvVars []string          `json:"env_vars"`
	Cwd     string            `json:"cwd"`
}

// Entry reads the server command a host runs for the plugin: Codex's own answer for
// Codex, the installed plugin's .mcp.json for Claude Code.
func Entry(host string) (*Server, error) {
	switch host {
	case "claude":
		inst, err := claudeInstalled()
		if err != nil {
			return nil, err
		}
		if inst == nil {
			return nil, fmt.Errorf("Claude Code has no %s installed", PluginID)
		}
		return claudeEntry(inst.InstallPath)
	case "codex":
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		data, err := exec.CommandContext(ctx, "codex", "mcp", "list", "--json").Output()
		if err != nil {
			return nil, fmt.Errorf("codex mcp list: %w", err)
		}
		return codexEntry(data)
	}
	return nil, fmt.Errorf("host %q; the hosts are claude and codex", host)
}

// claudeEntry reads the plugin's .mcp.json and substitutes ${CLAUDE_PLUGIN_ROOT}, as
// Claude Code does, which also exports the variable to the server.
func claudeEntry(installPath string) (*Server, error) {
	if installPath == "" {
		return nil, errors.New("Claude Code records no installPath for the plugin; reinstall it with atlas-obsidian setup")
	}
	file := filepath.Join(installPath, ".mcp.json")
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var config struct {
		MCPServers map[string]Server `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	s, ok := config.MCPServers[ServerName]
	if !ok {
		return nil, fmt.Errorf("%s has no %s server", file, ServerName)
	}
	expand := func(v string) string { return strings.ReplaceAll(v, "${CLAUDE_PLUGIN_ROOT}", installPath) }
	s.Command = expand(s.Command)
	for i, a := range s.Args {
		s.Args[i] = expand(a)
	}
	env := map[string]string{"CLAUDE_PLUGIN_ROOT": installPath}
	for k, v := range s.Env {
		env[k] = expand(v)
	}
	s.Env = env
	return &s, nil
}

// codexEntry reads the plugin's server from the output of codex mcp list --json.
func codexEntry(data []byte) (*Server, error) {
	var list []struct {
		Name      string `json:"name"`
		Transport struct {
			Type string `json:"type"`
			Server
		} `json:"transport"`
	}
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("codex mcp list: %w", err)
	}
	for _, s := range list {
		if s.Name != ServerName {
			continue
		}
		if s.Transport.Type != "stdio" {
			return nil, fmt.Errorf("Codex runs the %s server over %s, not stdio", ServerName, s.Transport.Type)
		}
		srv := s.Transport.Server
		return &srv, nil
	}
	return nil, fmt.Errorf("Codex lists no %s server", ServerName)
}
