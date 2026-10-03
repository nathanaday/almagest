package host

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
