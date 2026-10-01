package vault

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
)

// VaultConfigSchema is the schema of a vault's own config file.
const VaultConfigSchema = "atlas.vault-config.v1"

// VaultConfigFile is a vault's own config file, relative to its root.
const VaultConfigFile = ".atlas/config.json"

// Agents are the harnesses Atlas starts.
var Agents = []string{"claude", "codex"}

// Terminals are the terminals Atlas opens a command in.
var Terminals = []string{"terminal", "iterm", "wezterm", "ghostty", "custom"}

// Preferences are how the user starts agents. The machine file holds them for every vault;
// a vault's own file overrides them key by key. An empty field is not set.
type Preferences struct {
	Agent           string            `json:"agent,omitempty"`
	AgentCommands   map[string]string `json:"agent_commands,omitempty"`
	Terminal        string            `json:"terminal,omitempty"`
	TerminalCommand string            `json:"terminal_command,omitempty"`
}

// VaultConfig is a vault's own config file.
type VaultConfig struct {
	Schema      string      `json:"schema"`
	Preferences Preferences `json:"preferences"`
}

// PreferenceKeys are the keys config set and unset take.
func PreferenceKeys() []string {
	keys := []string{"agent"}
	for _, a := range Agents {
		keys = append(keys, "agent_commands."+a)
	}
	return append(keys, "terminal", "terminal_command")
}

// Get reads one key; "" is not set.
func (p Preferences) Get(key string) string {
	switch key {
	case "agent":
		return p.Agent
	case "terminal":
		return p.Terminal
	case "terminal_command":
		return p.TerminalCommand
	}
	if a, ok := strings.CutPrefix(key, "agent_commands."); ok {
		return p.AgentCommands[a]
	}
	return ""
}

// Set writes one key after checking its value; "" unsets it.
func (p *Preferences) Set(key, value string) error {
	value = strings.TrimSpace(value)
	switch key {
	case "agent":
		if value != "" && !slices.Contains(Agents, value) {
			return fmt.Errorf("agent %q is not one of %s", value, strings.Join(Agents, ", "))
		}
		p.Agent = value
	case "terminal":
		if value != "" && !slices.Contains(Terminals, value) {
			return fmt.Errorf("terminal %q is not one of %s", value, strings.Join(Terminals, ", "))
		}
		p.Terminal = value
	case "terminal_command":
		if value != "" && !strings.Contains(value, "{command}") {
			return fmt.Errorf("terminal_command needs {command} where the agent's command goes, as in: kitty sh -lic {command}")
		}
		p.TerminalCommand = value
	default:
		a, ok := strings.CutPrefix(key, "agent_commands.")
		if !ok || !slices.Contains(Agents, a) {
			return fmt.Errorf("no preference %q; the keys are %s", key, strings.Join(PreferenceKeys(), ", "))
		}
		if value == "" {
			delete(p.AgentCommands, a)
			if len(p.AgentCommands) == 0 {
				p.AgentCommands = nil
			}
			return nil
		}
		if p.AgentCommands == nil {
			p.AgentCommands = map[string]string{}
		}
		p.AgentCommands[a] = value
	}
	return nil
}

// check checks every value, as Set does.
func (p Preferences) check() error {
	var q Preferences
	for _, k := range PreferenceKeys() {
		if err := q.Set(k, p.Get(k)); err != nil {
			return err
		}
	}
	for a := range p.AgentCommands {
		if !slices.Contains(Agents, a) {
			return fmt.Errorf("agent_commands names %q, which is not one of %s", a, strings.Join(Agents, ", "))
		}
	}
	return nil
}

// Source is where an effective preference comes from.
type Source string

const (
	FromDefault Source = "default"
	FromGlobal  Source = "global"
	FromVault   Source = "vault"
)

// Effective are the preferences Atlas acts on: the vault's, else the machine's, else the default.
type Effective struct {
	Agent           string            `json:"agent"`
	AgentCommand    string            `json:"agent_command"`
	AgentCommands   map[string]string `json:"agent_commands"`
	Terminal        string            `json:"terminal"`
	TerminalCommand string            `json:"terminal_command"`
	Sources         map[string]Source `json:"sources"`
}

// Get reads one key of the effective preferences.
func (e Effective) Get(key string) string {
	return Preferences{Agent: e.Agent, AgentCommands: e.AgentCommands, Terminal: e.Terminal, TerminalCommand: e.TerminalCommand}.Get(key)
}

// Merge layers the vault's preferences over the machine's. The vault wins on every key both set.
func Merge(global, local Preferences) Effective {
	e := Effective{AgentCommands: map[string]string{}, Sources: map[string]Source{}}
	pick := func(key, def string) string {
		if v := local.Get(key); v != "" {
			e.Sources[key] = FromVault
			return v
		}
		if v := global.Get(key); v != "" {
			e.Sources[key] = FromGlobal
			return v
		}
		e.Sources[key] = FromDefault
		return def
	}
	e.Agent = pick("agent", "claude")
	for _, a := range Agents {
		e.AgentCommands[a] = pick("agent_commands."+a, a)
	}
	e.AgentCommand = e.AgentCommands[e.Agent]
	e.Terminal = pick("terminal", "terminal")
	e.TerminalCommand = pick("terminal_command", "")
	return e
}

// ConfigPath is the vault's own config file.
func (v *Vault) ConfigPath() string { return v.Abs(VaultConfigFile) }

// LoadConfig reads the vault's own config file. A missing file is an empty config.
func (v *Vault) LoadConfig() (*VaultConfig, error) {
	c := &VaultConfig{Schema: VaultConfigSchema}
	data, err := os.ReadFile(v.ConfigPath())
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	if err := decodeStrict(data, c); err != nil {
		return nil, fmt.Errorf("%s: %w", v.ConfigPath(), err)
	}
	if c.Schema != VaultConfigSchema {
		return nil, fmt.Errorf("%s: schema %q is not %s", v.ConfigPath(), c.Schema, VaultConfigSchema)
	}
	if err := c.Preferences.check(); err != nil {
		return nil, fmt.Errorf("%s: %w", v.ConfigPath(), err)
	}
	return c, nil
}

// SaveConfig writes the vault's own config file.
func (v *Vault) SaveConfig(c *VaultConfig) error {
	c.Schema = VaultConfigSchema
	data, err := marshalConfig(c)
	if err != nil {
		return err
	}
	return writeAtomic(v.ConfigPath(), data)
}

// marshalConfig writes a config file as people read it: indented, with < > & as they are.
func marshalConfig(c any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	err := enc.Encode(c)
	return buf.Bytes(), err
}

// decodeStrict decodes JSON and refuses a key it does not know, so a typo is an error.
func decodeStrict(data []byte, into any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(into)
}
