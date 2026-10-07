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
	Plugin      = "almagest"
	Marketplace = "nathanaday-almagest"
	PluginID    = Plugin + "@" + Marketplace
	// Source is where the marketplace lives by default.
	Source = "nathanaday/almagest"
)

// Hosts are the agent harnesses setup knows.
var Hosts = []string{"claude", "codex"}

// Install is one installed plugin, as its host records it.
type Install struct {
	Version string `json:"version"`
	Enabled bool   `json:"-"`
	// InstallPath is the plugin's folder in Claude Code's cache; Codex does not report it.
	InstallPath string `json:"installPath"`
}

// EnableHint says how to enable the plugin in a host that has it disabled.
func EnableHint(host string) string {
	if host == "codex" {
		return fmt.Sprintf("set enabled = true under [plugins.%q] in ~/.codex/config.toml", PluginID)
	}
	return "run: claude plugin enable " + PluginID
}

// UpdateHint says how to fetch the plugin again in a host, so its files match the
// marketplace.
func UpdateHint(host string) string {
	if host == "codex" {
		kind, source := codexMarketplace()
		return codexUpdateHint(kind, source)
	}
	return fmt.Sprintf("run: claude plugin marketplace update %s && claude plugin update %s", Marketplace, PluginID)
}

// codexUpdateHint fits the steps to the marketplace's source: codex plugin marketplace
// upgrade refreshes only a Git marketplace, and fails for a local one.
func codexUpdateHint(kind, source string) string {
	readd := fmt.Sprintf("codex plugin remove %s && codex plugin add %s", PluginID, PluginID)
	switch kind {
	case "git":
		return fmt.Sprintf("run: codex plugin marketplace upgrade %s && %s", Marketplace, readd)
	case "local":
		return fmt.Sprintf("bring the marketplace folder %s up to date, then run: %s", source, readd)
	}
	return fmt.Sprintf("run: codex plugin marketplace upgrade %s (a Git marketplace only); codex plugin remove %s; codex plugin add %s", Marketplace, PluginID, PluginID)
}

// codexMarketplace reads the source type (git or local) and the source of the plugin's
// marketplace from codex plugin marketplace list --json, or "" when Codex does not say.
func codexMarketplace() (kind, source string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, "codex", "plugin", "marketplace", "list", "--json").Output()
	if err != nil {
		return "", ""
	}
	return codexMarketplaceSource(data)
}

func codexMarketplaceSource(data []byte) (kind, source string) {
	var list struct {
		Marketplaces []struct {
			Name   string `json:"name"`
			Source struct {
				Type   string `json:"sourceType"`
				Source string `json:"source"`
			} `json:"marketplaceSource"`
		} `json:"marketplaces"`
	}
	if json.Unmarshal(data, &list) != nil {
		return "", ""
	}
	for _, m := range list.Marketplaces {
		if m.Name == Marketplace {
			return m.Source.Type, m.Source.Source
		}
	}
	return "", ""
}

// ClaudeDir is Claude Code's config folder: $CLAUDE_CONFIG_DIR or ~/.claude. It never
// falls back to a folder relative to the working directory.
func ClaudeDir() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("the home folder is unknown (%v); set CLAUDE_CONFIG_DIR to Claude Code's config folder", err)
	}
	return filepath.Join(home, ".claude"), nil
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
	dir, err := ClaudeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "plugins", "installed_plugins.json")
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
	var inst Install
	var many []Install
	if err := json.Unmarshal(raw, &many); err == nil {
		if len(many) == 0 {
			return nil, nil
		}
		inst = many[0]
	} else if err := json.Unmarshal(raw, &inst); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	enabled, err := claudeEnabled(dir)
	if err != nil {
		return nil, err
	}
	inst.Enabled = enabled
	return &inst, nil
}

// claudeEnabled reads the plugin's key in enabledPlugins of the user's settings, where
// setup installs it. A missing key counts as enabled: only a manifest with
// defaultEnabled: false starts a plugin turned off.
func claudeEnabled(dir string) (bool, error) {
	path := filepath.Join(dir, "settings.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	var settings struct {
		EnabledPlugins map[string]bool `json:"enabledPlugins"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	if enabled, ok := settings.EnabledPlugins[PluginID]; ok {
		return enabled, nil
	}
	return true, nil
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
	return codexInstall(data)
}

// codexInstall reads the plugin's install from the output of codex plugin list --json.
func codexInstall(data []byte) (*Install, error) {
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
	dir, err := ClaudeDir()
	if err != nil {
		return false, err
	}
	file := filepath.Join(dir, "settings.json")
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
