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
	"github.com/nathanaday/atlas-obsidian/internal/vault"
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
		"\"plugin marketplace\") cat \"$FAKE_MARKETPLACES\" 2>/dev/null ;;\n" +
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
			`FAIL codex server     codex runs "${CLAUDE_PLUGIN_ROOT}/scripts/atlas-obsidian", whose placeholder it does not expand; run: codex plugin marketplace upgrade ` + host.Marketplace + ` (a Git marketplace only)`},
		{"fewer tools", `{"type": "stdio", "command": "` + testvault.MCPServer(t, "vault", "search") + `", "args": [], "env": null, "env_vars": [], "cwd": null}`,
			"FAIL codex server     the server lists vault, search, and this binary serves " + strings.Join(all, ", ")},
		{"a server that does not start", `{"type": "stdio", "command": "/bin/sh", "args": ["-c", "echo atlas: the atlas-obsidian binary is not installed. >&2; exit 1"], "env": null, "env_vars": [], "cwd": null}`,
			"FAIL codex server     the server did not start: atlas: the atlas-obsidian binary is not installed."},
		{"a server that exits with nothing on stderr", `{"type": "stdio", "command": "/usr/bin/true", "args": [], "env": null, "env_vars": [], "cwd": null}`,
			"FAIL codex server     the server did not start: the server ended (exit status 0) before it answered, and wrote nothing to stderr; codex mcp list --json shows the command Codex runs; run it in a shell to see what it does"},
		{"a working entry", `{"type": "stdio", "command": "` + working + `", "args": [], "env": null, "env_vars": ["ATLAS_HOME"], "cwd": null}`,
			"ok   codex server     atlas: " + fmt.Sprint(len(all)) + " tools"},
		{"a working entry whose script expands variables", `{"type": "stdio", "command": "/bin/sh", "args": ["-c", "w=${ATLAS_BIN:-` + working + `}; exec \\"$w\\""], "env": null, "env_vars": [], "cwd": null}`,
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

// appServerAnswer is a fake codex app-server that answers hooks/list with the plugin's
// hooks in the given trust states, and one hook of another plugin.
func appServerAnswer(states ...string) string {
	hooks := []string{`{"key": "other@m:hooks.json:stop:0:0", "pluginId": "other@m", "trustStatus": "untrusted"}`}
	for i, s := range states {
		hooks = append(hooks, fmt.Sprintf(`{"key": "%s:hooks/hooks.json:%d", "pluginId": "%s", "trustStatus": "%s"}`, host.PluginID, i, host.PluginID, s))
	}
	answer := `{"id": 2, "result": {"data": [{"cwd": "/x", "hooks": [` + strings.Join(hooks, ", ") + `], "errors": [], "warnings": []}]}}`
	return `while IFS= read -r l; do case "$l" in *hooks/list*) echo '{"method": "remoteControl/status/changed", "params": {}}'; echo '` + answer + `'; exit 0 ;; esac; done`
}

func TestDoctorReadsTheCodexHookTrust(t *testing.T) {
	working := testvault.MCPServer(t, mcpserver.ToolNames()...)
	entry := `{"type": "stdio", "command": "` + working + `", "args": [], "env": null, "env_vars": [], "cwd": null}`
	trusted := []string{"trusted", "trusted", "trusted", "trusted", "trusted", "trusted", "trusted", "managed"}
	oneModified := append([]string{"modified"}, trusted[1:]...)
	untrusted := []string{"untrusted", "untrusted", "untrusted", "untrusted", "untrusted", "untrusted", "untrusted", "untrusted"}
	cases := []struct {
		name, appServer, want string
		fails                 bool
	}{
		{"all trusted", appServerAnswer(trusted...), fmt.Sprintf("ok   %-16s 8 hooks trusted", "codex hooks"), false},
		{"one modified", appServerAnswer(oneModified...),
			fmt.Sprintf("FAIL %-16s Codex runs 7 of 8 %s hooks (0 untrusted, 1 modified); open /hooks in Codex, trust the %s hooks, and start a new session", "codex hooks", host.Plugin, host.Plugin), true},
		{"a fresh install", appServerAnswer(untrusted...),
			fmt.Sprintf("FAIL %-16s Codex runs 0 of 8 %s hooks (8 untrusted, 0 modified)", "codex hooks", host.Plugin), true},
		{"no answer", "", fmt.Sprintf("--   %-16s Codex did not report its hooks", "codex hooks"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tv := testvault.New(t)
			r := run{t: t, tv: tv}
			fakeHosts(t, entry, working, c.appServer)
			code, out, _ := r.atlas("", "doctor")
			if !strings.Contains(out, c.want) {
				t.Fatalf("doctor lacks %q:\n%s", c.want, out)
			}
			if c.fails != (code != 0) {
				t.Fatalf("doctor exits %d:\n%s", code, out)
			}
		})
	}
}

func TestDoctorFitsTheCodexUpdateToTheMarketplace(t *testing.T) {
	entry := `{"type": "stdio", "command": "${CLAUDE_PLUGIN_ROOT}/scripts/atlas-obsidian", "args": ["mcp"], "env": null, "env_vars": [], "cwd": null}`
	working := testvault.MCPServer(t, mcpserver.ToolNames()...)
	for _, c := range []struct{ kind, want string }{
		{"git", "run: codex plugin marketplace upgrade " + host.Marketplace + " && codex plugin remove " + host.PluginID + " && codex plugin add " + host.PluginID},
		{"local", "bring the marketplace folder /src/atlas up to date, then run: codex plugin remove " + host.PluginID + " && codex plugin add " + host.PluginID},
	} {
		t.Run(c.kind, func(t *testing.T) {
			tv := testvault.New(t)
			r := run{t: t, tv: tv}
			fakeHosts(t, entry, working, "")
			list := filepath.Join(t.TempDir(), "list.json")
			body := `{"marketplaces": [{"name": "` + host.Marketplace + `", "root": "/x", "marketplaceSource": {"sourceType": "` + c.kind + `", "source": "/src/atlas"}}]}`
			if err := os.WriteFile(list, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("FAKE_MARKETPLACES", list)
			_, out, _ := r.atlas("", "doctor")
			if !strings.Contains(out, "whose placeholder it does not expand; "+c.want) {
				t.Fatalf("doctor lacks %q:\n%s", c.want, out)
			}
		})
	}
}

func TestDoctorNamesTheMigrationOfAnOldLayout(t *testing.T) {
	tv := testvault.New(t)
	r := run{t: t, tv: tv}
	t.Setenv("PATH", t.TempDir()+":/usr/bin:/bin")
	atlas := tv.Read("Atlas.md")
	if !strings.Contains(atlas, "\nlayout: 4\n") {
		t.Fatalf("Atlas.md holds no layout 4:\n%s", atlas)
	}
	tv.Write("Atlas.md", strings.Replace(atlas, "\nlayout: 4\n", "\nlayout: 3\n", 1))
	_, out, _ := r.atlas("", "doctor")
	if !strings.Contains(out, vault.ErrLegacy.Error()) || strings.Contains(out, "6.x layout") || !strings.Contains(out, "vault migrate --dry-run --vault ") {
		t.Fatalf("doctor on a layout-3 vault:\n%s", out)
	}
}
