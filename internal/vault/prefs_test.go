package vault

import (
	"os"
	"strings"
	"testing"
)

func TestMergeVaultWins(t *testing.T) {
	global := Preferences{Agent: "claude", Terminal: "wezterm", AgentCommands: map[string]string{"codex": "codex --full-auto"}}
	local := Preferences{AgentCommands: map[string]string{"claude": "claude-work"}, Terminal: "terminal"}
	e := Merge(global, local)
	if e.Agent != "claude" || e.Sources["agent"] != FromGlobal {
		t.Fatalf("agent: %+v", e)
	}
	if e.AgentCommand != "claude-work" || e.Sources["agent_commands.claude"] != FromVault {
		t.Fatalf("the vault's command wins: %+v", e)
	}
	if e.AgentCommands["codex"] != "codex --full-auto" || e.Sources["agent_commands.codex"] != FromGlobal {
		t.Fatalf("a key only the global file sets stays: %+v", e)
	}
	if e.Terminal != "terminal" || e.Sources["terminal"] != FromVault {
		t.Fatalf("terminal: %+v", e)
	}
	d := Merge(Preferences{}, Preferences{})
	if d.Agent != "claude" || d.AgentCommand != "claude" || d.AgentCommands["codex"] != "codex" || d.Terminal != "terminal" || d.Sources["terminal"] != FromDefault {
		t.Fatalf("defaults: %+v", d)
	}
	if c := Merge(Preferences{}, Preferences{Agent: "codex"}); c.AgentCommand != "codex" {
		t.Fatalf("the command follows the agent: %+v", c)
	}
}

func TestPreferencesSetChecks(t *testing.T) {
	var p Preferences
	for _, bad := range [][2]string{{"agent", "gemini"}, {"terminal", "kitty"}, {"terminal_command", "kitty"}, {"agent_commands.gemini", "x"}, {"agnet", "claude"}} {
		if err := p.Set(bad[0], bad[1]); err == nil {
			t.Fatalf("Set(%q, %q) took a bad value", bad[0], bad[1])
		}
	}
	if err := p.Set("agent_commands.claude", " claude-work "); err != nil || p.AgentCommands["claude"] != "claude-work" {
		t.Fatalf("set: %v %+v", err, p)
	}
	if err := p.Set("agent_commands.claude", ""); err != nil || p.AgentCommands != nil {
		t.Fatalf("unset leaves no empty map: %v %+v", err, p)
	}
}

func TestVaultConfigFile(t *testing.T) {
	v := &Vault{Root: t.TempDir()}
	c, err := v.LoadConfig()
	if err != nil || c.Preferences.Agent != "" {
		t.Fatalf("a missing file is empty: %v %+v", err, c)
	}
	c.Preferences.Agent = "codex"
	if err := v.SaveConfig(c); err != nil {
		t.Fatal(err)
	}
	if c, err = v.LoadConfig(); err != nil || c.Preferences.Agent != "codex" {
		t.Fatalf("round trip: %v %+v", err, c)
	}
	os.WriteFile(v.ConfigPath(), []byte(`{"schema": "atlas.vault-config.v1", "preferences": {"agent_command": "claude-work"}}`), 0o644)
	if _, err := v.LoadConfig(); err == nil || !strings.Contains(err.Error(), "agent_command") {
		t.Fatalf("an unknown key is an error: %v", err)
	}
	os.WriteFile(v.ConfigPath(), []byte(`{"schema": "atlas.vault-config.v1", "preferences": {"terminal": "kitty"}}`), 0o644)
	if _, err := v.LoadConfig(); err == nil || !strings.Contains(err.Error(), "kitty") {
		t.Fatalf("a bad value is an error: %v", err)
	}
}

func TestMachineFileKeepsPreferences(t *testing.T) {
	h := Home{Root: t.TempDir()}
	os.WriteFile(h.ConfigPath(), []byte(`{"schema": "atlas.config.v1", "vaults": ["~/a"]}`), 0o644)
	c, err := h.Load()
	if err != nil {
		t.Fatalf("a file without preferences loads: %v", err)
	}
	c.Preferences.Terminal = "wezterm"
	if err := h.Save(c); err != nil {
		t.Fatal(err)
	}
	if c, err = h.Load(); err != nil || c.Preferences.Terminal != "wezterm" || len(c.Vaults) != 1 {
		t.Fatalf("round trip: %v %+v", err, c)
	}
}
