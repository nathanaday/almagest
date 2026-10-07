// Package cli is the atlas-obsidian command: one subcommand per tool action, the hooks, the MCP
// server, and the commands no tool needs (setup, doctor, version, open, vault migrate).
// Every command reaches the same function its tool does.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/brief"
	"github.com/nathanaday/atlas-obsidian/internal/change"
	"github.com/nathanaday/atlas-obsidian/internal/checkout"
	"github.com/nathanaday/atlas-obsidian/internal/core"
	"github.com/nathanaday/atlas-obsidian/internal/hooks"
	"github.com/nathanaday/atlas-obsidian/internal/journal"
	"github.com/nathanaday/atlas-obsidian/internal/lint"
	"github.com/nathanaday/atlas-obsidian/internal/match"
	"github.com/nathanaday/atlas-obsidian/internal/mcpserver"
	"github.com/nathanaday/atlas-obsidian/internal/migrate"
	"github.com/nathanaday/atlas-obsidian/internal/search"
	"github.com/nathanaday/atlas-obsidian/internal/source"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Version is the binary's version; the build stamps it.
var Version = "dev"

// CLI is one run of the command.
type CLI struct {
	// moved are the notes the views step of this command moved out of wiki-view/.
	moved  []vault.Moved
	In     io.Reader
	Out    io.Writer
	Err    io.Writer
	Getenv func(string) string
	Now    func() time.Time
	Dir    string
}

// views writes the views after a write, and tells the user, on stderr so JSON output
// stays clean, where each note found in wiki-view/ went.
func (c *CLI) views(v *vault.Vault, now time.Time) {
	moved, _ := core.Views(v, now)
	c.moved = append(c.moved, moved...)
	for _, m := range moved {
		fmt.Fprintln(c.Err, core.StrayLine(m))
	}
}

// New is the command as a process runs it.
func New() *CLI {
	dir, _ := os.Getwd()
	return &CLI{In: os.Stdin, Out: os.Stdout, Err: os.Stderr, Getenv: os.Getenv, Now: time.Now, Dir: dir}
}

const usageHead = `atlas-obsidian: one vault for what you know, and what your agents did.

Usage:
`

const usageTail = `
A command that acts on a vault takes --vault (a path, or a name from ~/.atlas/config.json),
else $ATLAS_VAULT, else the vault above the working folder. setup's --vault is the folder
of a new vault, vault init takes --path, and doctor, version, help, hook, and mcp take no
--vault.
--json prints JSON from: vault, search, context, source, change, checkout, journal, lint, config.
match always prints JSON.
No --json: setup, open, doctor, version, help, hook, mcp.
`

// commands are the usage of each command, in the order help lists them.
var commands = []struct{ name, usage string }{
	{"vault", `  atlas-obsidian vault [status] | sync [--views] | snapshot | trash PATH | migrate [--dry-run]
                       | init [--path FOLDER | FOLDER] --name N [--description D] [--tagging open|known]
`},
	{"search", `  atlas-obsidian search TEXT [--type T]... [--kind K]... [--tag T]... [--status S]... [--repository R] [--limit N]
`},
	{"context", `  atlas-obsidian context [REPOSITORY] [--tag T]... [--path P]
`},
	{"match", `  atlas-obsidian match --items FILE.json | --docs ID... [--tag T]... [--across]
`},
	{"source", `  atlas-obsidian source capture [--ingest NAME]... | [--text FILE --title T [--locator URL]] | [--repository R]
                                [--tag T]... [--new-tags]
  atlas-obsidian source chunks DOC
  atlas-obsidian source read DOC CHUNK
`},
	{"change", `  atlas-obsidian change propose FILE.json [--id ID] | show ID | apply ID | reject ID --reason R | undo ID
                        | start --kind ingest|repair [--title T] [--file NAME]... | progress ID TEXT...
`},
	{"checkout", `  atlas-obsidian checkout [list] | candidates TEXT... [--tag T]... [--type T]... [--limit N]
                          | make FILE.json | return FOLDER
`},
	{"journal", `  atlas-obsidian journal [list] | publish VOLUME
                                                   a journal volume is a folder directly under journals/
`},
	{"lint", `  atlas-obsidian lint [--tag T]...
`},
	{"hook", `  atlas-obsidian hook EVENT                        a hook; reads the event JSON on stdin
`},
	{"mcp", `  atlas-obsidian mcp                               the MCP server, over stdio
`},
	{"config", `  atlas-obsidian config [show] | set KEY VALUE [--global] | unset KEY [--global]
                                                   agent preferences: the vault's file wins over ~/.atlas/config.json
`},
	{"setup", `  atlas-obsidian setup [--agent claude|codex] [--no-plugin] [--plugin-source SOURCE]
                       [--vault FOLDER --name N [--description D] [--tagging open|known] [--allow-vault]]
                                                   installs the binary and the agent plugin, and makes a first vault
`},
	{"doctor", `  atlas-obsidian doctor                            checks the binary, each agent's plugin and server, and every vault
`},
	{"version", `  atlas-obsidian version
`},
	{"open", `  atlas-obsidian open [DOC] [--register] [--update-plugin]
`},
	{"help", `  atlas-obsidian help | COMMAND --help
`},
}

// usage is the whole usage, which help prints.
func usage() string {
	var b strings.Builder
	b.WriteString(usageHead)
	for _, c := range commands {
		b.WriteString(c.usage)
	}
	b.WriteString(usageTail)
	return b.String()
}

// commandUsage is one command's usage, or "" for a name help does not list.
func commandUsage(name string) string {
	for _, c := range commands {
		if c.name == name {
			return "Usage:\n" + c.usage + usageTail
		}
	}
	return ""
}

// wantsHelp reports whether an argument before -- asks for help.
func wantsHelp(argv []string) bool {
	for _, a := range argv {
		if a == "--" {
			return false
		}
		if a == "--help" || a == "-h" {
			return true
		}
	}
	return false
}

// Run runs the command and returns its exit code.
func (c *CLI) Run(argv []string) int {
	if len(argv) == 0 {
		fmt.Fprint(c.Out, usage())
		return 0
	}
	cmd, rest := argv[0], argv[1:]
	// Help runs nothing: a command reads --help as an option, and setup, vault init,
	// hook, and mcp act at once.
	if wantsHelp(rest) {
		if u := commandUsage(cmd); u != "" {
			fmt.Fprint(c.Out, u)
			return 0
		}
	}
	var err error
	switch cmd {
	case "vault":
		err = c.vaultCmd(rest)
	case "search":
		err = c.searchCmd(rest)
	case "context":
		err = c.contextCmd(rest)
	case "match":
		err = c.matchCmd(rest)
	case "source":
		err = c.sourceCmd(rest)
	case "change":
		err = c.changeCmd(rest)
	case "lint":
		err = c.lintCmd(rest)
	case "journal":
		err = c.journalCmd(rest)
	case "checkout":
		err = c.checkoutCmd(rest)
	case "hook":
		return c.hookCmd(rest)
	case "mcp":
		err = c.mcpCmd()
	case "config":
		err = c.configCmd(rest)
	case "setup":
		err = c.setupCmd(rest)
	case "doctor":
		return c.doctorCmd(rest)
	case "version", "--version", "-v":
		fmt.Fprintln(c.Out, "atlas-obsidian "+Version)
	case "open":
		err = c.openCmd(rest)
	case "help", "--help", "-h":
		if u := commandUsage(argOr(rest, 0)); u != "" {
			fmt.Fprint(c.Out, u)
			break
		}
		fmt.Fprint(c.Out, usage())
	default:
		err = fmt.Errorf("no command %q; atlas-obsidian help lists them", cmd)
	}
	if err != nil {
		fmt.Fprintln(c.Err, "atlas: "+err.Error())
		var coded exitError
		if errors.As(err, &coded) {
			return coded.code
		}
		return 1
	}
	return 0
}

func argOr(argv []string, i int) string {
	if i < len(argv) {
		return argv[i]
	}
	return ""
}

// args are a command's positional arguments and its --flags.
type args struct {
	pos   []string
	flags map[string][]string
}

// removed refuses an option a command no longer takes. The command lists it among
// parse's booleans, so it never takes the next word as its value.
func (a args) removed(name, why string) error {
	if a.has(name) {
		return fmt.Errorf("--%s is gone: %s", name, why)
	}
	return nil
}

// parse reads --name value, --name=value, and the boolean flags named in bools.
func parse(argv []string, bools ...string) args {
	a := args{flags: map[string][]string{}}
	isBool := map[string]bool{"json": true}
	for _, b := range bools {
		isBool[b] = true
	}
	for i := 0; i < len(argv); i++ {
		s := argv[i]
		if s == "--" {
			a.pos = append(a.pos, argv[i+1:]...)
			break
		}
		name, ok := strings.CutPrefix(s, "--")
		if !ok || name == "" {
			a.pos = append(a.pos, s)
			continue
		}
		if k, v, eq := strings.Cut(name, "="); eq {
			a.flags[k] = append(a.flags[k], v)
			continue
		}
		if isBool[name] || i+1 >= len(argv) {
			a.flags[name] = append(a.flags[name], "true")
			continue
		}
		a.flags[name] = append(a.flags[name], argv[i+1])
		i++
	}
	return a
}

func (a args) get(name string) string {
	if v := a.flags[name]; len(v) > 0 {
		return v[len(v)-1]
	}
	return ""
}

func (a args) has(name string) bool { return len(a.flags[name]) > 0 }

func (a args) list(name string) []string { return a.flags[name] }

func (a args) arg(i int) string {
	if i < len(a.pos) {
		return a.pos[i]
	}
	return ""
}

func (c *CLI) home() vault.Home { return vault.HomeFrom(c.Getenv) }

func (c *CLI) open(a args) (*vault.Vault, error) {
	return vault.Select(a.get("vault"), c.Dir, c.home(), c.Getenv(vault.EnvVault))
}

func (c *CLI) index(a args) (*vault.Index, error) {
	v, err := c.open(a)
	if err != nil {
		return nil, err
	}
	return vault.Load(v)
}

// emit prints an entity as JSON with --json, else through the human printer.
func (c *CLI) emit(a args, v any, human func(w io.Writer)) error {
	if a.has("json") || human == nil {
		if len(c.moved) > 0 {
			v = withMoved(v, c.moved)
		}
		enc := json.NewEncoder(c.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	human(c.Out)
	return nil
}

// withMoved adds moved_from_wiki_view to a JSON result, whatever its type, so a caller that
// reads only stdout (the Obsidian plugin) learns where each note went.
func withMoved(v any, moved []vault.Moved) any {
	data, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out map[string]any
	if json.Unmarshal(data, &out) != nil || out == nil {
		return v
	}
	out["moved_from_wiki_view"] = moved
	return out
}

func (c *CLI) readJSON(file string, into any) error {
	var data []byte
	var err error
	if file == "-" || file == "" {
		data, err = io.ReadAll(c.In)
	} else {
		data, err = os.ReadFile(file)
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, into)
}

func (c *CLI) readText(file string) (string, error) {
	if file == "" || file == "-" {
		data, err := io.ReadAll(c.In)
		return string(data), err
	}
	data, err := os.ReadFile(file)
	return string(data), err
}

// user is how the CLI acts: as the user, now.
func (c *CLI) vaultCmd(argv []string) error {
	a := parse(argv, "views", "dry-run")
	now := c.Now()
	switch a.arg(0) {
	case "", "status":
		idx, err := c.index(a)
		if err != nil {
			return err
		}
		st := core.StatusOf(idx, now)
		st.Versions = map[string]string{"binary": Version, "obsidian_plugin": idx.V.InstalledPluginVersion()}
		return c.emit(a, map[string]any{"status": st}, func(w io.Writer) { printStatus(w, st) })
	case "init":
		p := a.get("path")
		if p == "" {
			p = a.arg(1)
		}
		if p == "" {
			p = c.Dir
		}
		st, err := core.Init(vault.InitOptions{Path: p, Name: a.get("name"), Description: a.get("description"), Tagging: a.get("tagging")}, c.home(), now)
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"status": st}, func(w io.Writer) {
			fmt.Fprintf(w, "Vault %s is ready at %s.\nOpen it in Obsidian (atlas-obsidian open --register --vault %s) and turn on the Atlas plugin under Community plugins.\n", st.Vault.Name, st.Vault.Path, shellArg(st.Vault.Path))
		})
	case "sync":
		v, err := c.open(a)
		if err != nil {
			return err
		}
		s, err := core.Sync(v, now, core.SyncOptions{Views: a.has("views")})
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"synced": s}, func(w io.Writer) {
			fmt.Fprintf(w, "Synced: %s, %d moved back, %s, %s, %s", count(len(s.Knowledge), "knowledge document", "knowledge documents"), len(s.Moved), count(len(s.Lost), "lost session", "lost sessions"), count(len(s.Sessions), "session callout", "session callouts"), count(s.Views, "view", "views"))
			if s.Settings {
				fmt.Fprint(w, ", the harness settings")
			}
			fmt.Fprintln(w, ".")
			for _, m := range s.Strays {
				fmt.Fprintln(w, core.StrayLine(m))
			}
			for _, p := range s.Skipped {
				fmt.Fprintf(w, "Left %s as saved during the sync; the next sync derives it.\n", p)
			}
		})
	case "snapshot":
		v, err := c.open(a)
		if err != nil {
			return err
		}
		sha, n, err := core.Snapshot(v)
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"snapshot": map[string]any{"commit": sha, "files": n}}, func(w io.Writer) {
			if sha == "" {
				fmt.Fprintln(w, "Nothing to commit: the vault holds no hand edits.")
				return
			}
			fmt.Fprintf(w, "Committed %s as a snapshot, %s.\n", count(n, "hand edit", "hand edits"), short(sha))
		})
	case "trash":
		v, err := c.open(a)
		if err != nil {
			return err
		}
		res, err := core.Trash(v, a.arg(1), now)
		if err != nil {
			return err
		}
		if res.Moved != "" {
			c.views(v, now)
		}
		if err := c.emit(a, map[string]any{"trash": res}, func(w io.Writer) { printTrash(w, res) }); err != nil {
			return err
		}
		if len(res.Backlinks) > 0 {
			return exitError{code: 2, err: fmt.Errorf("%s stays: %s; point them elsewhere first", res.Path, count(len(res.Backlinks), "document links it", "documents link it"))}
		}
		return nil
	case "migrate":
		v, err := c.open(a)
		if err != nil {
			return err
		}
		if a.has("dry-run") {
			r, err := migrate.Plan(v)
			if err != nil {
				return err
			}
			return c.emit(a, r, func(w io.Writer) { printMigration(w, r, false) })
		}
		r, err := migrate.Run(v, now)
		if err != nil {
			return err
		}
		return c.emit(a, r, func(w io.Writer) { printMigration(w, r, true) })
	}
	return fmt.Errorf("vault takes status, init, sync, snapshot, trash, or migrate, not %q", a.arg(0))
}

func printTrash(w io.Writer, r *core.Trashed) {
	if len(r.Backlinks) > 0 {
		fmt.Fprintf(w, "%s stays: these link it.\n", r.Path)
		for _, b := range r.Backlinks {
			fmt.Fprintf(w, "  %s\n", b.Path)
		}
		return
	}
	fmt.Fprintf(w, "Moved %s to %s.\n", r.Path, r.Moved)
	if r.Change != nil {
		fmt.Fprintf(w, "  through the change %s (%s)\n", r.Change.Title, r.Change.ID)
	}
}

// exitError is an error with its own exit code.
type exitError struct {
	code int
	err  error
}

func (e exitError) Error() string { return e.err.Error() }

func printMigration(w io.Writer, r *migrate.Report, done bool) {
	if done {
		fmt.Fprintf(w, "Migrated %s from the %s layout to 10.0 in one commit, %s.\n", r.Vault, r.From, short(r.Commit))
	} else {
		fmt.Fprintf(w, "The migration of %s from the %s layout to 10.0 would make these moves; run it without --dry-run to write them, or add --json to list every file.\n", r.Vault, r.From)
	}
	// The moves, counted by the folders they leave and reach.
	type pair struct{ from, to string }
	counts := map[pair]int{}
	var order []pair
	for _, m := range r.Moved {
		k := pair{path.Dir(m.From), path.Dir(m.To)}
		if counts[k] == 0 {
			order = append(order, k)
		}
		counts[k]++
	}
	for _, k := range order {
		fmt.Fprintf(w, "  move  %s/ → %s/: %s\n", k.from, k.to, count(counts[k], "file", "files"))
	}
	if n := len(r.Edited); n > 0 {
		fmt.Fprintf(w, "  edit  %s: the fields and sections of the threads, or the paths they name\n", count(n, "file", "files"))
	}
	if n := len(r.Removed); n > 0 {
		fmt.Fprintf(w, "  remove  %s that code writes again in %s/\n", count(n, "view", "views"), vault.WikiView)
	}
	for _, x := range r.Warnings {
		fmt.Fprintf(w, "  warning: %s\n", x)
	}
	if r.Plugin != "" {
		fmt.Fprintf(w, "Obsidian plugin: updated to %s; reload Obsidian to use it.\n", r.Plugin)
	}
	for _, m := range r.Strays {
		fmt.Fprintln(w, core.StrayLine(m))
	}
	if done && r.Problems > 0 {
		fmt.Fprintf(w, "Lint finds %d errors after the move: atlas-obsidian lint lists them.\n", r.Problems)
	}
}

func short(sha string) string { return sha[:min(10, len(sha))] }

// count is a number and its noun, singular for one.
func count(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func printStatus(w io.Writer, st *core.Status) {
	fmt.Fprintf(w, "%s · %s · %s · tagging %s\n", st.Vault.Name, st.Vault.Path, st.Vault.ID, st.Vault.Tagging)
	var docs []string
	for _, t := range []string{"topic", "source", "repository"} {
		docs = append(docs, fmt.Sprintf("%d %s", st.Documents[t], t))
	}
	fmt.Fprintf(w, "Documents: %s · %d draft · %d contested\n", strings.Join(docs, ", "), st.Topics.Draft, st.Topics.Contested)
	var tagList []string
	for i, t := range st.Tags {
		if i == 12 {
			tagList = append(tagList, "…")
			break
		}
		tagList = append(tagList, fmt.Sprintf("%s %d", t.Tag, t.Count))
	}
	if len(tagList) > 0 {
		fmt.Fprintf(w, "Tags: %s\n", strings.Join(tagList, " · "))
	}
	fmt.Fprintf(w, "Sessions: %d running · %d waiting · %d idle\n", len(st.Sessions.Running), len(st.Sessions.Waiting), len(st.Sessions.Idle))
	for _, s := range st.Sessions.Waiting {
		fmt.Fprintf(w, "  waits for you: %s %s\n", s.Title, s.Description)
	}
	fmt.Fprintf(w, "Ingest: %d · Pending: %d · Proposed changes: %d · Problems: %d\n", len(st.Ingest), len(st.Pending), len(st.Changes.Proposed), st.Problems)
	for _, c := range st.Changes.Proposed {
		fmt.Fprintf(w, "  proposed: %s (%s)\n", c.Title, c.ID)
	}
}

func (c *CLI) searchCmd(argv []string) error {
	a := parse(argv)
	idx, err := c.index(a)
	if err != nil {
		return err
	}
	q := search.Query{Text: strings.Join(a.pos, " "), Types: a.list("type"), Kinds: a.list("kind"), Tags: a.list("tag"), Status: a.list("status"), Repository: a.get("repository")}
	q.Limit, _ = strconv.Atoi(a.get("limit"))
	hits, err := search.Search(idx, q)
	if err != nil {
		return err
	}
	return c.emit(a, hits, func(w io.Writer) {
		for _, h := range hits.Hits {
			kind := h.Ref.Type
			if h.Ref.Kind != "" {
				kind += " " + h.Ref.Kind
			}
			fmt.Fprintf(w, "%6.2f  %-16s %s  (%s)\n", h.Score, kind, h.Ref.Title, h.Ref.ID)
			if h.Snippet != "" {
				fmt.Fprintf(w, "        %s\n", h.Snippet)
			}
		}
		fmt.Fprintf(w, "%d of %d\n", len(hits.Hits), hits.Total)
		if len(hits.Facets.Tags) > 0 {
			var list []string
			for t, n := range hits.Facets.Tags {
				list = append(list, fmt.Sprintf("%s %d", t, n))
			}
			sort.Strings(list)
			fmt.Fprintf(w, "with: %s\n", strings.Join(list, " · "))
		}
	})
}

func (c *CLI) contextCmd(argv []string) error {
	a := parse(argv)
	idx, err := c.index(a)
	if err != nil {
		return err
	}
	b, err := brief.Of(idx, brief.Input{Repository: a.arg(0), Tags: a.list("tag"), Path: a.get("path")})
	if err != nil {
		return err
	}
	return c.emit(a, b, func(w io.Writer) {
		var tagList []string
		for _, t := range b.Tags {
			tagList = append(tagList, fmt.Sprintf("%s %d", t.Tag, t.Count))
		}
		fmt.Fprintf(w, "Tags: %s\n", strings.Join(tagList, " · "))
		for _, p := range b.Pages {
			fmt.Fprintf(w, "Page of %s: %s\n", p.Tag, p.Ref.Title)
		}
		var repos []string
		for _, r := range b.Repositories {
			repos = append(repos, r.Title)
		}
		if len(repos) > 0 {
			fmt.Fprintf(w, "Repositories: %s\n", strings.Join(repos, ", "))
		}
		for _, p := range b.Policies {
			fmt.Fprintf(w, "Policy (%s): %s · %s\n", orDash(p.Strength), p.Ref.Title, strings.Join(p.Via, ", "))
		}
		for _, in := range b.Instructions {
			fmt.Fprintf(w, "Instructions: %s\n", in.Path)
		}
		if r := b.Repository; r != nil {
			fmt.Fprintf(w, "Repository: %s · %s · head %s · %d dirty · %d ahead, %d behind the remote", r.Path, r.Branch, r.Head, len(r.Dirty), r.Ahead, r.BehindRemote)
			if r.Behind >= 0 {
				fmt.Fprintf(w, " · %d commits past its description", r.Behind)
			}
			fmt.Fprintln(w)
		}
	})
}

func orDash(s string) string {
	if s == "" {
		return "–"
	}
	return s
}

func (c *CLI) matchCmd(argv []string) error {
	a := parse(argv, "across")
	idx, err := c.index(a)
	if err != nil {
		return err
	}
	in := match.Input{Docs: append(a.list("docs"), a.pos...), Tags: a.list("tag"), Across: a.has("across")}
	if file := a.get("items"); file != "" {
		if err := c.readJSON(file, &in.Items); err != nil {
			return fmt.Errorf("--items: %w", err)
		}
		in.Docs = nil
	}
	m, err := match.Run(idx, in)
	if err != nil {
		return err
	}
	return c.emit(a, m, nil)
}

func (c *CLI) sourceCmd(argv []string) error {
	a := parse(argv, "new-tags")
	switch a.arg(0) {
	case "capture":
		if err := a.removed("resolves", "a stub no longer exists in Atlas 9.0; capture the source without --resolves"); err != nil {
			return err
		}
		if err := a.removed("inbox", "the inbox is ingest/ since Atlas 10.0; name the files with --ingest"); err != nil {
			return err
		}
		v, err := c.open(a)
		if err != nil {
			return err
		}
		req := source.Request{Ingest: a.list("ingest"), Title: a.get("title"), Repository: a.get("repository"), Tags: a.list("tag"), Locator: a.get("locator"), NewTags: a.has("new-tags")}
		if a.has("text") {
			text, err := c.readText(a.get("text"))
			if err != nil {
				return err
			}
			req.Text = text
		}
		res, err := source.Capture(v, req, c.Now())
		if err != nil {
			return err
		}
		c.views(v, c.Now())
		return c.emit(a, res, func(w io.Writer) {
			for _, cp := range res.Captured {
				if cp.Duplicate != "" {
					fmt.Fprintf(w, "already captured: %s (%s)\n", cp.Ref.Title, cp.Duplicate)
					continue
				}
				fmt.Fprintf(w, "captured: %s (%s) · %s · %d chunks\n", cp.Ref.Title, cp.Ref.ID, cp.Measure, len(cp.Chunks))
			}
		})
	case "chunks":
		idx, err := c.index(a)
		if err != nil {
			return err
		}
		chunks, err := source.Chunks(idx, a.arg(1))
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"chunks": chunks}, func(w io.Writer) {
			for _, ch := range chunks {
				fmt.Fprintf(w, "%d/%d  %s\n", ch.Index, ch.Count, ch.Locator)
			}
		})
	case "read":
		idx, err := c.index(a)
		if err != nil {
			return err
		}
		n, _ := strconv.Atoi(a.arg(2))
		blob, err := source.Read(idx, a.arg(1), n)
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"blob": blob}, func(w io.Writer) {
			if blob.File != "" {
				fmt.Fprintf(w, "Read %s pages %s\n", blob.File, blob.Pages)
				return
			}
			fmt.Fprintln(w, blob.Content)
		})
	}
	return fmt.Errorf("source takes capture, chunks, or read, not %q", a.arg(0))
}

func (c *CLI) changeCmd(argv []string) error {
	a := parse(argv)
	v, err := c.open(a)
	if err != nil {
		return err
	}
	now := c.Now()
	var pv *change.Preview
	switch a.arg(0) {
	case "propose":
		var plan change.Plan
		if err := c.readJSON(a.arg(1), &plan); err != nil {
			return fmt.Errorf("the plan: %w", err)
		}
		if id := a.get("id"); id != "" {
			plan.ID = id
		}
		pv, err = change.Propose(v, plan, now)
	case "start":
		pv, err = change.Start(v, change.StartIn{Title: a.get("title"), Kind: a.get("kind"), Files: a.list("file")}, now)
	case "progress":
		pv, err = change.Progress(v, a.arg(1), strings.Join(a.pos[min(2, len(a.pos)):], " "), now)
	case "show", "":
		idx, lerr := vault.Load(v)
		if lerr != nil {
			return lerr
		}
		pv, err = change.Show(idx, a.arg(1))
	case "apply":
		pv, err = change.Apply(v, a.arg(1), now, nil)
	case "reject":
		pv, err = change.Reject(v, a.arg(1), a.get("reason"), now)
	case "undo":
		pv, err = change.Undo(v, a.arg(1), now)
	default:
		return fmt.Errorf("change takes propose, start, progress, show, apply, reject, or undo, not %q", a.arg(0))
	}
	if err != nil {
		return err
	}
	if a.arg(0) != "show" && a.arg(0) != "" {
		c.views(v, now)
	}
	return c.emit(a, pv, func(w io.Writer) { printPreview(w, pv) })
}

func printPreview(w io.Writer, pv *change.Preview) {
	fmt.Fprintf(w, "%s · %s · %s\n", pv.Ref.Title, pv.Status, pv.Counts.String())
	fmt.Fprintf(w, "  %s\n", pv.Ref.Path)
	for _, wl := range pv.Writes {
		line := fmt.Sprintf("  %-7s %s", wl.Op, wl.Title)
		if wl.Kind != "" {
			line += " (" + wl.Kind + ")"
		}
		if wl.Lines != "" {
			line += "  " + wl.Lines
		}
		if wl.Note != "" {
			line += "  · " + wl.Note
		}
		fmt.Fprintln(w, line)
	}
	for _, r := range pv.Rewrites {
		fmt.Fprintf(w, "  rewrite %s\n", r.Title)
	}
	if len(pv.NewTags) > 0 {
		fmt.Fprintf(w, "  new tags: %s\n", strings.Join(pv.NewTags, ", "))
	}
	for _, warn := range pv.Warnings {
		fmt.Fprintf(w, "  warning: %s\n", warn)
	}
	if pv.Reason != "" {
		fmt.Fprintf(w, "  reason: %s\n", pv.Reason)
	}
	if pv.Commit != "" {
		fmt.Fprintf(w, "  commit %s\n", short(pv.Commit))
	}
}

func (c *CLI) checkoutCmd(argv []string) error {
	a := parse(argv)
	v, err := c.open(a)
	if err != nil {
		return err
	}
	now := c.Now()
	switch a.arg(0) {
	case "", "list":
		list := checkout.List(v)
		return c.emit(a, map[string]any{"checkouts": list}, func(w io.Writer) {
			if len(list) == 0 {
				fmt.Fprintln(w, "No checkout yet.")
			}
			for _, e := range list {
				fmt.Fprintf(w, "%s · %s · %s · %d edited · returned %s\n", e.Folder, e.Request, count(e.Documents, "document", "documents"), e.Edited, orDash(e.Returned))
			}
		})
	case "candidates":
		idx, err := vault.Load(v)
		if err != nil {
			return err
		}
		limit, _ := strconv.Atoi(a.get("limit"))
		cands, err := checkout.Candidates(idx, strings.Join(a.pos[1:], " "), a.list("tag"), a.list("type"), limit)
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"candidates": cands}, func(w io.Writer) {
			for _, cd := range cands {
				via := ""
				if cd.Via != "" {
					via = " · via " + cd.Via
				}
				fmt.Fprintf(w, "%6.2f  %d  %s (%s)%s\n", cd.Score, cd.Distance, cd.Ref.Title, cd.Ref.ID, via)
			}
		})
	case "make":
		var o checkout.Order
		if err := c.readJSON(a.arg(1), &o); err != nil {
			return fmt.Errorf("the order: %w", err)
		}
		m, err := checkout.Make(v, o, now)
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"made": m}, func(w io.Writer) {
			fmt.Fprintf(w, "Checked out %s into %s; the reading list is %s.\n", count(len(m.Copies), "document", "documents"), m.Folder, m.ReadingList)
		})
	case "return":
		r, err := checkout.Return(v, a.arg(1), now)
		if err != nil {
			return err
		}
		c.views(v, now)
		return c.emit(a, map[string]any{"returned": r}, func(w io.Writer) {
			printPreview(w, r.Change)
			for _, s := range r.Skipped {
				fmt.Fprintf(w, "  left out: %s\n", s)
			}
		})
	}
	return fmt.Errorf("checkout takes list, candidates, make, or return, not %q", a.arg(0))
}

func (c *CLI) journalCmd(argv []string) error {
	a := parse(argv)
	v, err := c.open(a)
	if err != nil {
		return err
	}
	switch a.arg(0) {
	case "", "list":
		idx, err := vault.Load(v)
		if err != nil {
			return err
		}
		vols := journal.Volumes(idx)
		return c.emit(a, map[string]any{"journals": vols}, func(w io.Writer) {
			if len(vols) == 0 {
				fmt.Fprintf(w, "No journal yet: a volume is a folder directly under %s/.\n", vault.Journals)
			}
			for _, vol := range vols {
				line := fmt.Sprintf("%s (%s) · %s", vol.Name, vol.Volume, count(vol.Notes, "note", "notes"))
				if vol.Edition != "" {
					line += " · latest: " + vol.Edition
				}
				if vol.Changed {
					line += " · changed since"
				}
				fmt.Fprintln(w, line)
			}
		})
	case "publish":
		now := c.Now()
		p, err := journal.Publish(v, a.arg(1), now)
		if err != nil {
			return err
		}
		c.views(v, now)
		return c.emit(a, map[string]any{"published": p}, func(w io.Writer) {
			fmt.Fprintf(w, "Published %s (%s), %s. It waits for the wiki as a pending source.\n", p.Source.Title, p.Source.ID, short(p.Commit))
		})
	}
	return fmt.Errorf("journal takes list or publish, not %q", a.arg(0))
}

func (c *CLI) lintCmd(argv []string) error {
	a := parse(argv)
	idx, err := c.index(a)
	if err != nil {
		return err
	}
	f, err := lint.Run(idx, lint.Options{Tags: append(a.list("tag"), a.pos...), Now: c.Now()})
	if err != nil {
		return err
	}
	return c.emit(a, f, func(w io.Writer) {
		if len(f.Findings) == 0 {
			fmt.Fprintf(w, "%d documents, no findings.\n", f.Checked)
			return
		}
		groups := map[string][]lint.Finding{}
		var order []string
		for _, x := range f.Findings {
			k := x.Severity + " " + x.Check
			if _, ok := groups[k]; !ok {
				order = append(order, k)
			}
			groups[k] = append(groups[k], x)
		}
		for _, k := range order {
			list := groups[k]
			fmt.Fprintf(w, "%s (%d) · fix: %s\n", k, len(list), list[0].Fix)
			for _, x := range list {
				fmt.Fprintf(w, "  %s: %s\n", x.Doc.Path, x.Message)
			}
		}
		fmt.Fprintf(w, "%d documents · %d errors · %d warnings · %d info\n", f.Checked, f.Counts[lint.Error], f.Counts[lint.Warning], f.Counts[lint.Info])
	})
}

func (c *CLI) hookCmd(argv []string) int {
	if len(argv) == 0 {
		var names []string
		for n := range hooks.Events {
			names = append(names, n)
		}
		sort.Strings(names)
		fmt.Fprintln(c.Err, "atlas-obsidian hook takes one of: "+strings.Join(names, ", "))
		return 1
	}
	env := hooks.Env{Getenv: c.Getenv, Now: c.Now}
	if err := hooks.Run(argv[0], c.In, c.Out, env); err != nil {
		fmt.Fprintln(c.Err, "atlas-obsidian hook "+argv[0]+": "+err.Error())
		return 1
	}
	return 0
}

func (c *CLI) mcpCmd() error {
	dir := c.Getenv("CLAUDE_PROJECT_DIR")
	if dir == "" {
		dir = c.Dir
	}
	return mcpserver.New(mcpserver.Options{Version: Version, Dir: dir, Getenv: c.Getenv, Now: c.Now}).Serve(context.Background())
}
