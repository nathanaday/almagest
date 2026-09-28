// Package host installs and inspects the agent plugin in Claude Code and in Codex.
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

// The agent plugin's names.
const (
	Plugin      = "atlas-obsidian-v2-exp"
	Marketplace = "nathanaday-atlas-obsidian"
	PluginID    = Plugin + "@" + Marketplace
	// Source is where the marketplace lives by default.
	Source = "nathanaday/atlas-obsidian"
)

// Hosts are the agent harnesses setup knows.
var Hosts = []string{"claude", "codex"}

// Install is one installed plugin, as its host records it.
type Install struct {
	Version string `json:"version"`
	Path    string `json:"installPath,omitempty"`
	Enabled bool   `json:"enabled"`
}

// ClaudeDir is Claude Code's config folder: $CLAUDE_CONFIG_DIR or ~/.claude.
func ClaudeDir() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude"
	}
	return filepath.Join(home, ".claude")
}

// CLI is the path of a host's command, or "".
func CLI(host string) string {
	p, err := exec.LookPath(host)
	if err != nil {
		return ""
	}
	return p
}

// Installed reads the plugin's install in a host, or nil when it has none.
func Installed(host string) (*Install, error) {
	switch host {
	case "claude":
		return claudeInstalled()
	case "codex":
		return codexInstalled()
	}
	return nil, fmt.Errorf("host %q; the hosts are claude and codex", host)
}

func claudeInstalled() (*Install, error) {
	path := filepath.Join(ClaudeDir(), "plugins", "installed_plugins.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var registry struct {
		Plugins map[string]json.RawMessage `json:"plugins"`
	}
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	raw, ok := registry.Plugins[PluginID]
	if !ok {
		return nil, nil
	}
	var many []Install
	if err := json.Unmarshal(raw, &many); err == nil {
		if len(many) == 0 {
			return nil, nil
		}
		many[0].Enabled = true
		return &many[0], nil
	}
	var one Install
	if err := json.Unmarshal(raw, &one); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	one.Enabled = true
	return &one, nil
}

func codexInstalled() (*Install, error) {
	if CLI("codex") == "" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, "codex", "plugin", "list", "--json", "--marketplace", Marketplace).Output()
	if err != nil {
		return nil, fmt.Errorf("codex plugin list: %w", err)
	}
	var result struct {
		Installed []struct {
			PluginID string `json:"pluginId"`
			Version  string `json:"version"`
			Enabled  bool   `json:"enabled"`
		} `json:"installed"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("codex plugin list: %w", err)
	}
	for _, p := range result.Installed {
		if p.PluginID == PluginID {
			return &Install{Version: p.Version, Enabled: p.Enabled}, nil
		}
	}
	return nil, nil
}

// Commands are the host's commands that add the marketplace and install the plugin.
func Commands(host, source string) [][]string {
	if source == "" {
		source = Source
	}
	install := "install"
	if host == "codex" {
		install = "add"
	}
	return [][]string{
		{host, "plugin", "marketplace", "add", source},
		{host, "plugin", install, PluginID},
	}
}

// InstallPlugin adds the marketplace and installs the plugin in a host, and returns the
// commands it ran. A marketplace already known is not an error.
func InstallPlugin(host, source string) ([]string, error) {
	if CLI(host) == "" {
		return nil, fmt.Errorf("the %s command is not on PATH", host)
	}
	var ran []string
	for i, args := range Commands(host, source) {
		line := strings.Join(args, " ")
		ran = append(ran, line)
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		if err != nil {
			text := strings.ToLower(string(out))
			if i == 0 && (strings.Contains(text, "already") || strings.Contains(text, "exists")) {
				continue
			}
			return ran, fmt.Errorf("%s: %s", line, strings.TrimSpace(string(out)))
		}
	}
	return ran, nil
}

// AllowVault adds a vault to permissions.additionalDirectories in the user's Claude Code
// settings, so a session that starts in a linked repository can reach the vault. It
// keeps every other key, and reports whether it wrote.
func AllowVault(vault string) (bool, error) {
	file := filepath.Join(ClaudeDir(), "settings.json")
	settings := map[string]any{}
	if data, err := os.ReadFile(file); err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return false, fmt.Errorf("%s is not valid JSON; add %s to permissions.additionalDirectories by hand", file, vault)
		}
	}
	perms, _ := settings["permissions"].(map[string]any)
	if perms == nil {
		perms = map[string]any{}
	}
	list, _ := perms["additionalDirectories"].([]any)
	for _, x := range list {
		if s, ok := x.(string); ok && s == vault {
			return false, nil
		}
	}
	perms["additionalDirectories"] = append(list, vault)
	settings["permissions"] = perms
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(file, append(data, '\n'), 0o644)
}
