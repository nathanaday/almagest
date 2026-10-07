package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nathanaday/almagest/internal/core"
	"github.com/nathanaday/almagest/internal/gitx"
	"github.com/nathanaday/almagest/internal/host"
	"github.com/nathanaday/almagest/internal/lint"
	"github.com/nathanaday/almagest/internal/mcpserver"
	"github.com/nathanaday/almagest/internal/obsidian"
	"github.com/nathanaday/almagest/internal/vault"
)

// setupCmd installs the binary under ~/.almagest/bin, adds the agent plugin to the host, and
// makes a first vault when asked.
func (c *CLI) setupCmd(argv []string) error {
	a := parse(argv, "no-plugin", "allow-vault", "yes")
	if err := a.removed("yes", "setup asks nothing; run it without --yes"); err != nil {
		return err
	}
	agent := a.get("agent")
	if agent == "" {
		agent = "claude"
	}
	if agent != "claude" && agent != "codex" {
		return fmt.Errorf("--agent takes claude or codex, not %q", agent)
	}
	h := c.home()
	if !gitx.Available() {
		return errors.New("git is not on PATH; almagest needs it for every vault's history")
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
	enabled := false
	if a.has("no-plugin") {
		fmt.Fprintln(c.Out, "plugin   skipped (--no-plugin)")
	} else if inst, err := host.Installed(agent); err != nil {
		fmt.Fprintf(c.Out, "plugin   could not read the install: %v\n", err)
	} else if inst != nil && !inst.Enabled {
		fmt.Fprintf(c.Out, "plugin   %s %s in %s is disabled; %s\n", host.PluginID, inst.Version, agent, host.EnableHint(agent))
	} else if inst != nil {
		enabled = true
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
			enabled = true
			fmt.Fprintf(c.Out, "plugin   %s installed in %s\n", host.PluginID, agent)
		}
	}
	// Codex runs no plugin hook the user has not trusted, and a changed hooks.json needs
	// trust again, so every run says where the trust stands.
	if enabled && agent == "codex" {
		_, detail := codexHooks(c.Dir)
		fmt.Fprintf(c.Out, "hooks    %s\n", detail)
	}
	// A first vault.
	made := ""
	if p := a.get("vault"); p != "" {
		st, err := core.Init(vault.InitOptions{Path: p, Name: a.get("name"), Description: a.get("description"), Tagging: a.get("tagging")}, h, c.Now())
		if err != nil {
			return err
		}
		fmt.Fprintf(c.Out, "vault    %s at %s\n", st.Vault.Name, st.Vault.Path)
		made = st.Vault.Path
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
	if made != "" {
		fmt.Fprintf(c.Out, "  almagest open --register --vault %s\n", shellArg(made))
	} else {
		fmt.Fprintf(c.Out, "  start %s in an empty folder and say \"set up almagest\": the almagest-onboard skill makes the vault\n", agent)
		fmt.Fprintln(c.Out, "  or: almagest vault init --path ~/notes/work --name Work --tagging open --description \"...\"")
		fmt.Fprintln(c.Out, "  then: almagest open --register --vault ~/notes/work")
	}
	fmt.Fprintf(c.Out, "  optional: the Almagest plugin for Obsidian, from its community plugins: %s\n", vault.PluginLink)
	fmt.Fprintln(c.Out, "  almagest doctor checks every part")
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
			note(agent+" plugin", "not installed: almagest setup --agent "+agent)
		case !inst.Enabled:
			line(false, agent+" plugin", inst.Version+" is installed but disabled; "+host.EnableHint(agent))
		case inst.Version != Version && Version != "dev":
			line(false, agent+" plugin", fmt.Sprintf("%s, but the binary is %s; update one so they match", inst.Version, Version))
		default:
			line(true, agent+" plugin", inst.Version)
		}
		if inst != nil && inst.Enabled {
			ok, detail := c.serverCheck(agent)
			line(ok, agent+" server", detail)
			if agent == "codex" {
				switch state, detail := codexHooks(c.Dir); state {
				case checkUnknown:
					note("codex hooks", detail)
				default:
					line(state == checkOK, "codex hooks", detail)
				}
			}
		}
	}
	cfg, err := h.Load()
	if err != nil {
		line(false, "config", err.Error())
		return 1
	}
	line(true, "config", fmt.Sprintf("%s lists %d vaults", h.ConfigPath(), len(cfg.Vaults)))
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
		if err := v.CheckLayout(); err != nil {
			at := shellArg(vault.Shorten(v.Root))
			line(false, "vault "+v.Name(), fmt.Sprintf("%s: %v. From another folder: almagest vault migrate --dry-run --vault %s, then almagest vault migrate --vault %s", vault.Shorten(v.Root), err, at, at))
			continue
		}
		f, _ := lint.Run(idx, lint.Options{Quick: true, Now: c.Now()})
		detail := fmt.Sprintf("%s · %d documents", vault.Shorten(v.Root), len(idx.Docs))
		ok := true
		if f != nil && f.Counts[lint.Error] > 0 {
			detail += fmt.Sprintf(" · %d errors (almagest lint)", f.Counts[lint.Error])
			ok = false
		}
		if installed := v.InstalledPluginVersion(); installed != "" {
			detail += " · Obsidian plugin " + installed
		}
		line(ok, "vault "+v.Name(), detail)
	}
	if failed {
		return 1
	}
	return 0
}

// serverCheck starts the plugin's MCP server as the host runs it, and compares its tools
// with the tools this binary serves.
func (c *CLI) serverCheck(agent string) (bool, string) {
	s, err := host.Entry(agent)
	if err != nil {
		return false, fmt.Sprintf("%v; %s", err, host.UpdateHint(agent))
	}
	// A shell script in the args may hold its own ${…}; only the command must be a path.
	if strings.Contains(s.Command, "${") {
		return false, fmt.Sprintf("%s runs %q, whose placeholder it does not expand; %s", agent, s.Command, host.UpdateHint(agent))
	}
	names, err := host.Probe(context.Background(), s, c.Dir)
	if err != nil {
		step := "run it in a shell to see what it does: " + s.ShellLine()
		if agent == "codex" {
			step = "codex mcp list --json shows the command Codex runs; run it in a shell to see what it does"
		}
		return false, fmt.Sprintf("the server did not start: %v; %s", err, step)
	}
	want := mcpserver.ToolNames()
	if !sameSet(names, want) {
		return false, fmt.Sprintf("the server lists %s, and this binary serves %s; %s, or install the binary that matches it", strings.Join(names, ", "), strings.Join(want, ", "), host.UpdateHint(agent))
	}
	return true, fmt.Sprintf("%s: %d tools", host.ServerName, len(names))
}

// The states of a check that may not know its answer.
const (
	checkOK = iota
	checkFail
	checkUnknown
)

// codexHooks reports whether Codex runs the plugin's hooks. setup and doctor print it
// in the same words.
func codexHooks(dir string) (int, string) {
	trust, err := host.CodexHookTrust(dir)
	if err != nil {
		return checkUnknown, fmt.Sprintf("Codex did not report its hooks (%v); open /hooks in Codex to see whether it trusts the %s hooks", err, host.Plugin)
	}
	if trust.Total() == 0 {
		return checkUnknown, fmt.Sprintf("Codex lists no %s hook; start a new Codex session, then run almagest doctor again", host.Plugin)
	}
	if off := trust.Off(); off > 0 {
		return checkFail, fmt.Sprintf("Codex runs %d of %d %s hooks (%d untrusted, %d modified); open /hooks in Codex, trust the %s hooks, and start a new session",
			trust.Total()-off, trust.Total(), host.Plugin, trust["untrusted"], trust["modified"], host.Plugin)
	}
	return checkOK, fmt.Sprintf("%d hooks trusted", trust.Total())
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]bool{}
	for _, x := range a {
		seen[x] = true
	}
	for _, x := range b {
		if !seen[x] {
			return false
		}
	}
	return true
}

// openCmd opens the vault, or one of its documents, in Obsidian. A vault Obsidian does not
// know is added to its registry with --register, which restarts Obsidian on macOS.
func (c *CLI) openCmd(argv []string) error {
	a := parse(argv, "register", "update-plugin")
	if err := a.removed("update-plugin", "Obsidian installs and updates the Almagest plugin from its community plugins: "+vault.PluginLink); err != nil {
		return err
	}
	v, err := c.open(a)
	if err != nil {
		return err
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
			fmt.Fprintf(c.Out, "Obsidian does not know %s yet. Run almagest open --register --vault %s (it restarts Obsidian on macOS), or use Open folder as vault.\n", v.Root, shellArg(v.Root))
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

// shellArg quotes a path for a command line the user may paste; a leading ~ stays bare
// so the shell expands it.
func shellArg(p string) string {
	if p != "" && strings.Trim(p, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_./~+,@%:") == "" {
		return p
	}
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		return "~/'" + strings.ReplaceAll(rest, "'", `'\''`) + "'"
	}
	return "'" + strings.ReplaceAll(p, "'", `'\''`) + "'"
}
