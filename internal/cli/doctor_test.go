package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/host"
	"github.com/nathanaday/atlas-obsidian/internal/mcpserver"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
)

func TestDoctorReportsADisabledPlugin(t *testing.T) {
	tv := testvault.New(t)
	r := run{t: t, tv: tv}
	bin := t.TempDir()
	codexList := `{"installed": [{"pluginId": "` + host.PluginID + `", "version": "8.1.1", "enabled": false}]}`
	scripts := map[string]string{
		"claude": "#!/bin/sh\nexit 0\n",
		"codex":  "#!/bin/sh\necho '" + codexList + "'\n",
	}
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+":/usr/bin:/bin")

	claude := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claude)
	if err := os.MkdirAll(filepath.Join(claude, "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	registry := `{"version": 2, "plugins": {"` + host.PluginID + `": [{"scope": "user", "version": "8.1.1"}]}}`
	if err := os.WriteFile(filepath.Join(claude, "plugins", "installed_plugins.json"), []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claude, "settings.json"), []byte(`{"enabledPlugins": {"`+host.PluginID+`": false}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out, _ := r.atlas("", "doctor")
	if code == 0 {
		t.Fatalf("doctor passes with both plugins disabled:\n%s", out)
	}
	for _, want := range []string{
		"FAIL claude plugin    8.1.1 is installed but disabled; run: claude plugin enable " + host.PluginID,
		"FAIL codex plugin     8.1.1 is installed but disabled; " + host.EnableHint("codex"),
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor lacks %q:\n%s", want, out)
		}
	}
}

// fakeHosts puts fake claude and codex commands on PATH, with the plugin installed and
// enabled in both. codexServer is the transport codex mcp list prints for atlas;
// claudeServer is a script that the installed plugin's .mcp.json runs as
// ${CLAUDE_PLUGIN_ROOT}/server. appServer is the
// body of a shell case for codex app-server ("" answers nothing).
func fakeHosts(t *testing.T, codexServer, claudeServer, appServer string) {
	t.Helper()
	bin := t.TempDir()
	codexList := `{"installed": [{"pluginId": "` + host.PluginID + `", "version": "8.1.1", "enabled": true}]}`
	mcpList := `[{"name": "atlas", "enabled": true, "transport": ` + codexServer + `}]`
	if appServer == "" {
		appServer = "exit 1"
	}
	codex := "#!/bin/sh\ncase \"$1 $2\" in\n" +
		"\"plugin list\") echo '" + codexList + "' ;;\n" +
		"\"mcp list\") echo '" + mcpList + "' ;;\n" +
		"\"app-server \") " + appServer + " ;;\n" +
		"esac\n"
	for name, body := range map[string]string{"claude": "#!/bin/sh\nexit 0\n", "codex": codex} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+":/usr/bin:/bin")

	claude := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claude)
	plugin := filepath.Join(claude, "plugins", "cache", "m", "atlas-obsidian", "8.1.1")
	if err := os.MkdirAll(plugin, 0o755); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(claudeServer)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plugin, "server"), script, 0o755); err != nil {
		t.Fatal(err)
	}
	mcp := `{"mcpServers": {"atlas": {"command": "${CLAUDE_PLUGIN_ROOT}/server", "args": []}}}`
	if err := os.WriteFile(filepath.Join(plugin, ".mcp.json"), []byte(mcp), 0o644); err != nil {
		t.Fatal(err)
	}
	registry := `{"version": 2, "plugins": {"` + host.PluginID + `": [{"scope": "user", "version": "8.1.1", "installPath": "` + plugin + `"}]}}`
	if err := os.WriteFile(filepath.Join(claude, "plugins", "installed_plugins.json"), []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorStartsEachHostsServer(t *testing.T) {
	all := mcpserver.ToolNames()
	working := testvault.MCPServer(t, all...)
	cases := []struct {
		name, codex, want string
	}{
		{"the 8.1.1 entry", `{"type": "stdio", "command": "${CLAUDE_PLUGIN_ROOT}/scripts/atlas-obsidian", "args": ["mcp"], "env": null, "env_vars": [], "cwd": null}`,
			`FAIL codex server     codex runs "${CLAUDE_PLUGIN_ROOT}/scripts/atlas-obsidian", whose placeholder it does not expand; ` + host.UpdateHint("codex")},
		{"fewer tools", `{"type": "stdio", "command": "` + testvault.MCPServer(t, "vault", "search") + `", "args": [], "env": null, "env_vars": [], "cwd": null}`,
			"FAIL codex server     the server lists vault, search, and this binary serves " + strings.Join(all, ", ")},
		{"a server that does not start", `{"type": "stdio", "command": "/bin/sh", "args": ["-c", "echo atlas: the atlas-obsidian binary is not installed. >&2; exit 1"], "env": null, "env_vars": [], "cwd": null}`,
			"FAIL codex server     the server did not start: atlas: the atlas-obsidian binary is not installed."},
		{"a working entry", `{"type": "stdio", "command": "` + working + `", "args": [], "env": null, "env_vars": ["ATLAS_HOME"], "cwd": null}`,
			"ok   codex server     atlas: " + fmt.Sprint(len(all)) + " tools"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tv := testvault.New(t)
			r := run{t: t, tv: tv}
			fakeHosts(t, c.codex, working, "")
			code, out, _ := r.atlas("", "doctor")
			if !strings.Contains(out, c.want) {
				t.Fatalf("doctor lacks %q:\n%s", c.want, out)
			}
			if !strings.Contains(out, "ok   claude server    atlas: "+fmt.Sprint(len(all))+" tools") {
				t.Fatalf("the Claude Code server, with ${CLAUDE_PLUGIN_ROOT} expanded, is not ok:\n%s", out)
			}
			if strings.HasPrefix(c.want, "FAIL") && code == 0 {
				t.Fatalf("doctor exits 0 with a failed server:\n%s", out)
			}
		})
	}
}
