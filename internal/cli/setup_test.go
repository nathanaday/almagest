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
