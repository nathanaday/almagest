package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/host"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
)

func TestSetupDoesNotReinstallWhenTheInstallCannotBeRead(t *testing.T) {
	tv := testvault.New(t)
	r := run{t: t, tv: tv}
	bin := t.TempDir()
	calls := filepath.Join(bin, "calls")
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\necho \"$@\" >> '"+calls+"'\n"), 0o755); err != nil {
		t.Fatal(err)
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
	settings := filepath.Join(claude, "settings.json")
	if err := os.WriteFile(settings, []byte(`{"enabledPlugins": `), 0o644); err != nil {
		t.Fatal(err)
	}

	out := r.ok("", "setup")
	if !strings.Contains(out, "plugin   could not read the install: "+settings) {
		t.Fatalf("setup:\n%s", out)
	}
	if data, _ := os.ReadFile(calls); len(data) != 0 {
		t.Fatalf("setup ran claude: %s", data)
	}

	if err := os.WriteFile(settings, []byte(`{"enabledPlugins": {"`+host.PluginID+`": false}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out = r.ok("", "setup")
	if !strings.Contains(out, "is disabled; "+host.EnableHint("claude")) {
		t.Fatalf("setup with a disabled plugin:\n%s", out)
	}
	if data, _ := os.ReadFile(calls); len(data) != 0 {
		t.Fatalf("setup ran claude: %s", data)
	}
}

func TestSetupReportsTheCodexHookTrustOnEveryRun(t *testing.T) {
	tv := testvault.New(t)
	r := run{t: t, tv: tv}
	working := testvault.MCPServer(t, "vault")
	entry := `{"type": "stdio", "command": "` + working + `", "args": [], "env": null, "env_vars": [], "cwd": null}`
	fakeHosts(t, entry, working, appServerAnswer("trusted", "modified"))
	want := "hooks    Codex runs 1 of 2 " + host.Plugin + " hooks (0 untrusted, 1 modified); open /hooks in Codex"
	for i := 0; i < 2; i++ {
		out := r.ok("", "setup", "--agent", "codex")
		if !strings.Contains(out, "plugin   "+host.PluginID+" 8.1.1 in codex") || !strings.Contains(out, want) {
			t.Fatalf("run %d of setup on an installed plugin lacks %q:\n%s", i+1, want, out)
		}
	}
}

func TestSetupReportsTheCodexHookTrustAfterAFreshInstall(t *testing.T) {
	tv := testvault.New(t)
	r := run{t: t, tv: tv}
	bin := t.TempDir()
	codex := "#!/bin/sh\ncase \"$1 $2\" in\n" +
		"\"plugin list\") echo '{\"installed\": []}' ;;\n" +
		"\"app-server \") " + appServerAnswer("untrusted", "untrusted") + " ;;\n" +
		"esac\nexit 0\n"
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte(codex), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	out := r.ok("", "setup", "--agent", "codex")
	want := "hooks    Codex runs 0 of 2 " + host.Plugin + " hooks (2 untrusted, 0 modified); open /hooks in Codex, trust the " + host.Plugin + " hooks, and start a new session"
	if !strings.Contains(out, "plugin   "+host.PluginID+" installed in codex") || !strings.Contains(out, want) {
		t.Fatalf("setup after a fresh install lacks %q:\n%s", want, out)
	}
}
