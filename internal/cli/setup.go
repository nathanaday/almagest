package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/core"
	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/host"
	"github.com/nathanaday/atlas-obsidian/internal/lint"
	"github.com/nathanaday/atlas-obsidian/internal/obsidian"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// setupCmd installs the binary under ~/.atlas/bin, adds the agent plugin to the host, and
// makes a first vault when asked.
func (c *CLI) setupCmd(argv []string) error {
	a := parse(argv, "no-plugin", "allow-vault", "yes")
	agent := a.get("agent")
	if agent == "" {
		agent = "claude"
	}
	if agent != "claude" && agent != "codex" {
		return fmt.Errorf("--agent takes claude or codex, not %q", agent)
	}
	h := c.home()
	if !gitx.Available() {
		return errors.New("git is not on PATH; atlas needs it for every vault's history")
	}
	// The binary.
	self, err := os.Executable()
	if err != nil {
		return err
	}
	self, _ = filepath.EvalSymlinks(self)
	target := h.BinPath()
	if real, _ := filepath.EvalSymlinks(target); real != self {
		if err := copyFile(self, target); err != nil {
			return fmt.Errorf("install the binary at %s: %w", target, err)
		}
		fmt.Fprintf(c.Out, "binary   installed at %s\n", target)
	} else {
		fmt.Fprintf(c.Out, "binary   %s\n", target)
	}
	// The agent plugin.
	if a.has("no-plugin") {
		fmt.Fprintln(c.Out, "plugin   skipped (--no-plugin)")
	} else if inst, _ := host.Installed(agent); inst != nil {
		fmt.Fprintf(c.Out, "plugin   %s %s in %s\n", host.PluginID, inst.Version, agent)
	} else {
		ran, err := host.InstallPlugin(agent, a.get("plugin-source"))
		for _, r := range ran {
			fmt.Fprintf(c.Out, "ran      %s\n", r)
		}
		if err != nil {
			fmt.Fprintf(c.Out, "plugin   not installed: %v\n", err)
			for _, cmd := range host.Commands(agent, a.get("plugin-source")) {
				fmt.Fprintf(c.Out, "         %s\n", strings.Join(cmd, " "))
			}
		} else {
			fmt.Fprintf(c.Out, "plugin   %s installed in %s\n", host.PluginID, agent)
		}
		if agent == "codex" {
			fmt.Fprintln(c.Out, "         Codex asks you to trust the plugin's hooks: open /hooks, trust them, and start a new session.")
		}
	}
	// A first vault.
	if p := a.get("vault"); p != "" {
		st, err := core.Init(vault.InitOptions{Path: p, Name: a.get("name"), Description: a.get("description"), Areas: a.get("areas")}, h, c.Now())
		if err != nil {
			return err
		}
		fmt.Fprintf(c.Out, "vault    %s at %s\n", st.Vault.Name, st.Vault.Path)
		if a.has("allow-vault") && agent == "claude" {
			if wrote, err := host.AllowVault(vault.Expand(st.Vault.Path)); err != nil {
				fmt.Fprintf(c.Out, "         %v\n", err)
			} else if wrote {
				fmt.Fprintln(c.Out, "         added to permissions.additionalDirectories in ~/.claude/settings.json")
			}
		}
	}
	fmt.Fprintln(c.Out, "")
	fmt.Fprintln(c.Out, "Next:")
	fmt.Fprintf(c.Out, "  start %s in an empty folder and say \"set up atlas\": the atlas-onboard skill makes the vault\n", agent)
	fmt.Fprintln(c.Out, "  or: atlas vault init --path ~/notes/work --name Work --areas few --description \"...\"")
	fmt.Fprintln(c.Out, "  then: atlas open, and turn on the Atlas plugin in Obsidian")
	fmt.Fprintln(c.Out, "  atlas doctor checks every part")
	return nil
}

func copyFile(from, to string) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	tmp := to + ".tmp"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return err
	}
	return os.Rename(tmp, to)
}

// doctorCmd checks the binary, the plugins, git, the machine file, and every vault.
func (c *CLI) doctorCmd(argv []string) int {
	h := c.home()
	failed := false
	line := func(ok bool, what, detail string) {
		mark := "ok  "
		if !ok {
			mark = "FAIL"
			failed = true
		}
		fmt.Fprintf(c.Out, "%s %-16s %s\n", mark, what, detail)
	}
	note := func(what, detail string) { fmt.Fprintf(c.Out, "--   %-16s %s\n", what, detail) }
	self, _ := os.Executable()
	line(true, "binary", fmt.Sprintf("%s at %s", Version, self))
	if gitx.Available() {
		out, _ := exec.Command("git", "--version").Output()
		line(true, "git", strings.TrimSpace(string(out)))
	} else {
		line(false, "git", "not on PATH")
	}
	for _, agent := range host.Hosts {
		if host.CLI(agent) == "" {
			note(agent, "not on PATH")
			continue
		}
		inst, err := host.Installed(agent)
		switch {
		case err != nil:
			line(false, agent+" plugin", err.Error())
		case inst == nil:
			note(agent+" plugin", "not installed: atlas setup --agent "+agent)
		case inst.Version != Version && Version != "dev":
			line(false, agent+" plugin", fmt.Sprintf("%s, but the binary is %s; update one so they match", inst.Version, Version))
		default:
			line(true, agent+" plugin", inst.Version)
		}
	}
	cfg, err := h.Load()
	if err != nil {
		line(false, "config", err.Error())
		return 1
	}
	line(true, "config", fmt.Sprintf("%s lists %d vaults", h.ConfigPath(), len(cfg.Vaults)))
	bundled := vault.PluginVersion()
	for _, root := range cfg.Paths() {
		v, err := vault.Open(root)
		if err != nil {
			line(false, "vault", err.Error())
			continue
		}
		idx, err := vault.Load(v)
		if err != nil {
			line(false, "vault "+v.Name(), err.Error())
			continue
		}
		f, _ := lint.Run(idx, lint.Options{Quick: true, Now: c.Now()})
		detail := fmt.Sprintf("%s · %d documents", vault.Shorten(v.Root), len(idx.Docs))
		ok := true
		if f != nil && f.Counts[lint.Error] > 0 {
			detail += fmt.Sprintf(" · %d errors (atlas lint)", f.Counts[lint.Error])
			ok = false
		}
		switch installed := v.InstalledPluginVersion(); {
		case installed == "":
			detail += " · no Obsidian plugin"
		case installed != bundled:
			detail += fmt.Sprintf(" · Obsidian plugin %s, the binary carries %s (atlas open --update-plugin)", installed, bundled)
			ok = false
		}
		line(ok, "vault "+v.Name(), detail)
	}
	if failed {
		return 1
	}
	return 0
}

// openCmd opens the vault, or one of its documents, in Obsidian. A vault Obsidian does not
// know is added to its registry with --register, which restarts Obsidian on macOS.
func (c *CLI) openCmd(argv []string) error {
	a := parse(argv, "register", "update-plugin")
	v, err := c.open(a)
	if err != nil {
		return err
	}
	if a.has("update-plugin") {
		wrote, err := vault.InstallPlugin(v)
		if err != nil {
			return err
		}
		fmt.Fprintf(c.Out, "Obsidian plugin: %d files updated; reload Obsidian to use them.\n", len(wrote))
	}
	target := v.Root
	if key := a.arg(0); key != "" {
		idx, err := vault.Load(v)
		if err != nil {
			return err
		}
		d, err := idx.Resolve(key)
		if err != nil {
			return err
		}
		target = v.Abs(d.Path)
	}
	registered, _, err := obsidian.Status(v.Root)
	if err != nil {
		return err
	}
	if !registered {
		if !a.has("register") {
			fmt.Fprintf(c.Out, "Obsidian does not know %s yet. Run atlas open --register (it restarts Obsidian on macOS), or use Open folder as vault.\n", v.Root)
			return nil
		}
		if err := obsidian.RegisterAndOpen(v.Root); err != nil {
			return err
		}
	}
	if err := obsidian.OpenPath(target); err != nil {
		return err
	}
	return nil
}
