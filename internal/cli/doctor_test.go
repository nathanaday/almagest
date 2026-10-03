package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/host"
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
