package host

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/testvault"
)

func TestClaudeInstalledReadsEnabledPlugins(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	if err := os.MkdirAll(filepath.Join(dir, "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	registry := `{"version": 2, "plugins": {"` + PluginID + `": [{"scope": "user", "version": "8.1.1"}]}}`
	if err := os.WriteFile(filepath.Join(dir, "plugins", "installed_plugins.json"), []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, settings string
		enabled        bool
	}{
		{"no settings file", "", true},
		{"no key", `{"enabledPlugins": {"other@market": false}}`, true},
		{"enabled", `{"enabledPlugins": {"` + PluginID + `": true}}`, true},
		{"disabled", `{"enabledPlugins": {"` + PluginID + `": false}}`, false},
	} {
		settings := filepath.Join(dir, "settings.json")
		os.Remove(settings)
		if c.settings != "" {
			if err := os.WriteFile(settings, []byte(c.settings), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		inst, err := Installed("claude")
		if err != nil || inst == nil || inst.Version != "8.1.1" || inst.Enabled != c.enabled {
			t.Fatalf("%s: %+v %v", c.name, inst, err)
		}
	}
}

func TestNoClaudeDirWithoutAHome(t *testing.T) {
	work := t.TempDir()
	t.Chdir(work)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("HOME", "")
	if _, err := os.UserHomeDir(); err == nil {
		t.Skip("this platform finds a home folder without $HOME")
	}
	if _, err := AllowVault("/tmp/vault"); err == nil || !strings.Contains(err.Error(), "CLAUDE_CONFIG_DIR") {
		t.Fatalf("AllowVault: %v", err)
	}
	if _, err := Installed("claude"); err == nil || !strings.Contains(err.Error(), "CLAUDE_CONFIG_DIR") {
		t.Fatalf("Installed: %v", err)
	}
	if entries, _ := os.ReadDir(work); len(entries) != 0 {
		t.Fatalf("wrote into the working folder: %v", entries)
	}
}

func TestCodexInstallReadsEnabled(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		flag := "false"
		if enabled {
			flag = "true"
		}
		data := `{"installed": [{"pluginId": "other@market", "version": "1.0.0", "enabled": true}, {"pluginId": "` + PluginID + `", "version": "8.1.1", "enabled": ` + flag + `}], "available": []}`
		inst, err := codexInstall([]byte(data))
		if err != nil || inst == nil || inst.Version != "8.1.1" || inst.Enabled != enabled {
			t.Fatalf("enabled %v: %+v %v", enabled, inst, err)
		}
	}
	if inst, err := codexInstall([]byte(`{"installed": []}`)); inst != nil || err != nil {
		t.Fatalf("not installed: %+v %v", inst, err)
	}
}

func TestCodexEntryReadsTheAtlasTransport(t *testing.T) {
	data := `[{"name": "other", "transport": {"type": "stdio", "command": "x"}},
	 {"name": "atlas", "enabled": true, "transport": {"type": "stdio", "command": "${CLAUDE_PLUGIN_ROOT}/scripts/atlas-obsidian", "args": ["mcp"], "env": null, "env_vars": ["ATLAS_HOME"], "cwd": null}}]`
	s, err := codexEntry([]byte(data))
	if err != nil || s.Command != "${CLAUDE_PLUGIN_ROOT}/scripts/atlas-obsidian" || len(s.Args) != 1 || s.EnvVars[0] != "ATLAS_HOME" || s.Cwd != "" {
		t.Fatalf("%+v %v", s, err)
	}
	if _, err := codexEntry([]byte(`[{"name": "other", "transport": {"type": "stdio", "command": "x"}}]`)); err == nil || !strings.Contains(err.Error(), "no atlas server") {
		t.Fatalf("no entry: %v", err)
	}
	if _, err := codexEntry([]byte(`[{"name": "atlas", "transport": {"type": "streamable_http", "url": "http://x"}}]`)); err == nil || !strings.Contains(err.Error(), "not stdio") {
		t.Fatalf("http entry: %v", err)
	}
}

func TestClaudeEntrySubstitutesThePluginRoot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	plugin := filepath.Join(dir, "plugins", "cache", "m", "p", "8.1.1")
	if err := os.MkdirAll(plugin, 0o755); err != nil {
		t.Fatal(err)
	}
	mcp := `{"mcpServers": {"atlas": {"command": "${CLAUDE_PLUGIN_ROOT}/scripts/atlas-obsidian", "args": ["mcp", "${CLAUDE_PLUGIN_ROOT}"]}}}`
	if err := os.WriteFile(filepath.Join(plugin, ".mcp.json"), []byte(mcp), 0o644); err != nil {
		t.Fatal(err)
	}
	registry := `{"version": 2, "plugins": {"` + PluginID + `": [{"scope": "user", "version": "8.1.1", "installPath": "` + plugin + `"}]}}`
	if err := os.WriteFile(filepath.Join(dir, "plugins", "installed_plugins.json"), []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Entry("claude")
	if err != nil {
		t.Fatal(err)
	}
	if s.Command != plugin+"/scripts/atlas-obsidian" || s.Args[1] != plugin || s.Env["CLAUDE_PLUGIN_ROOT"] != plugin {
		t.Fatalf("%+v", s)
	}
	if _, err := claudeEntry(""); err == nil || !strings.Contains(err.Error(), "installPath") {
		t.Fatalf("no install path: %v", err)
	}
}

func TestProbeListsTheToolsWithAReducedEnvironment(t *testing.T) {
	t.Setenv("ATLAS_HOME", "/atlas-home")
	t.Setenv("SECRET", "leaked")
	s := &Server{Command: testvault.MCPServer(t, "vault", "${ATLAS_HOME:-none}", "${SECRET:-none}", "${FROM_ENTRY:-none}"),
		EnvVars: []string{"ATLAS_HOME"}, Env: map[string]string{"FROM_ENTRY": "entry"}}
	names, err := Probe(context.Background(), s, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(names, " "); got != "vault /atlas-home none entry" {
		t.Fatalf("tools %q; want ATLAS_HOME and the entry's env passed, SECRET not", got)
	}
}

func TestProbeSaysWhyAServerDidNotStart(t *testing.T) {
	script := filepath.Join(t.TempDir(), "server")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho 'atlas: the atlas-obsidian binary is not installed.' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Probe(context.Background(), &Server{Command: script}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("%v", err)
	}
	_, err = Probe(context.Background(), &Server{Command: "${CLAUDE_PLUGIN_ROOT}/scripts/atlas-obsidian"}, t.TempDir())
	if err == nil {
		t.Fatal("a placeholder command started")
	}
}
