package host

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/almagest/internal/testvault"
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

func TestCodexEntryReadsTheAlmagestTransport(t *testing.T) {
	data := `[{"name": "other", "transport": {"type": "stdio", "command": "x"}},
	 {"name": "almagest", "enabled": true, "transport": {"type": "stdio", "command": "${CLAUDE_PLUGIN_ROOT}/scripts/almagest", "args": ["mcp"], "env": null, "env_vars": ["ALMAGEST_HOME"], "cwd": null}}]`
	s, err := codexEntry([]byte(data))
	if err != nil || s.Command != "${CLAUDE_PLUGIN_ROOT}/scripts/almagest" || len(s.Args) != 1 || s.EnvVars[0] != "ALMAGEST_HOME" || s.Cwd != "" {
		t.Fatalf("%+v %v", s, err)
	}
	if _, err := codexEntry([]byte(`[{"name": "other", "transport": {"type": "stdio", "command": "x"}}]`)); err == nil || !strings.Contains(err.Error(), "no almagest server") {
		t.Fatalf("no entry: %v", err)
	}
	if _, err := codexEntry([]byte(`[{"name": "almagest", "transport": {"type": "streamable_http", "url": "http://x"}}]`)); err == nil || !strings.Contains(err.Error(), "not stdio") {
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
	mcp := `{"mcpServers": {"almagest": {"command": "${CLAUDE_PLUGIN_ROOT}/scripts/almagest", "args": ["mcp", "${CLAUDE_PLUGIN_ROOT}"]}}}`
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
	if s.Command != plugin+"/scripts/almagest" || s.Args[1] != plugin || s.Env["CLAUDE_PLUGIN_ROOT"] != plugin {
		t.Fatalf("%+v", s)
	}
	if _, err := claudeEntry(""); err == nil || !strings.Contains(err.Error(), "installPath") {
		t.Fatalf("no install path: %v", err)
	}
}

func TestProbeListsTheToolsWithAReducedEnvironment(t *testing.T) {
	t.Setenv("ALMAGEST_HOME", "/almagest-home")
	t.Setenv("SECRET", "leaked")
	s := &Server{Command: testvault.MCPServer(t, "vault", "${ALMAGEST_HOME:-none}", "${SECRET:-none}", "${FROM_ENTRY:-none}"),
		EnvVars: []string{"ALMAGEST_HOME"}, Env: map[string]string{"FROM_ENTRY": "entry"}}
	names, err := Probe(context.Background(), s, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(names, " "); got != "vault /almagest-home none entry" {
		t.Fatalf("tools %q; want ALMAGEST_HOME and the entry's env passed, SECRET not", got)
	}
}

func TestProbeSaysWhyAServerDidNotStart(t *testing.T) {
	script := filepath.Join(t.TempDir(), "server")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho 'almagest: the almagest binary is not installed.' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Probe(context.Background(), &Server{Command: script}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("%v", err)
	}
	_, err = Probe(context.Background(), &Server{Command: "${CLAUDE_PLUGIN_ROOT}/scripts/almagest"}, t.TempDir())
	if err == nil {
		t.Fatal("a placeholder command started")
	}
}

func TestProbeSaysWhatHappenedWhenTheServerIsSilent(t *testing.T) {
	old := ProbeTimeout
	ProbeTimeout = 500 * time.Millisecond
	t.Cleanup(func() { ProbeTimeout = old })
	for _, c := range []struct {
		name string
		s    *Server
		want string
	}{
		{"a hang", &Server{Command: "/bin/sleep", Args: []string{"30"}}, "the server did not answer within 500ms"},
		{"a silent exit", &Server{Command: "/usr/bin/true"}, "the server ended (exit status 0) before it answered, and wrote nothing to stderr"},
		{"no such command", &Server{Command: "/no/such/server"}, "the command does not exist"},
	} {
		start := time.Now()
		_, err := Probe(context.Background(), c.s, t.TempDir())
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: %v, want %q", c.name, err, c.want)
		}
		if time.Since(start) > 5*time.Second {
			t.Fatalf("%s took %s", c.name, time.Since(start))
		}
	}
}

func TestShellLineQuotesWhatAShellWouldSplit(t *testing.T) {
	s := &Server{Command: "/bin/sh", Args: []string{"-c", "echo 'hi' $HOME", "plain/x"}}
	if got, want := s.ShellLine(), `/bin/sh -c 'echo '\''hi'\'' $HOME' plain/x`; got != want {
		t.Fatalf("%s, want %s", got, want)
	}
}

func TestCodexMarketplaceSourceReadsTheListing(t *testing.T) {
	data := `{"marketplaces": [{"name": "openai-api-curated", "root": "/x/.tmp/plugins"},
	 {"name": "` + Marketplace + `", "root": "/x/.tmp/marketplaces/m", "marketplaceSource": {"sourceType": "git", "source": "https://github.com/nathanaday/almagest.git"}}]}`
	if kind, source := codexMarketplaceSource([]byte(data)); kind != "git" || source != "https://github.com/nathanaday/almagest.git" {
		t.Fatalf("%q %q", kind, source)
	}
	if kind, _ := codexMarketplaceSource([]byte(`{"marketplaces": []}`)); kind != "" {
		t.Fatalf("no marketplace: %q", kind)
	}
	if !strings.Contains(codexUpdateHint("", ""), "(a Git marketplace only); codex plugin remove") {
		t.Fatal(codexUpdateHint("", ""))
	}
}
