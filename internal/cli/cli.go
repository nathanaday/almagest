// Package cli is the atlas-obsidian command: one subcommand per tool action, the hooks, the MCP
// server, and the commands no tool needs (setup, doctor, version, open, vault migrate).
// Every command reaches the same function its tool does.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/brief"
	"github.com/nathanaday/atlas-obsidian/internal/change"
	"github.com/nathanaday/atlas-obsidian/internal/core"
	"github.com/nathanaday/atlas-obsidian/internal/hooks"
	"github.com/nathanaday/atlas-obsidian/internal/lint"
	"github.com/nathanaday/atlas-obsidian/internal/match"
	"github.com/nathanaday/atlas-obsidian/internal/mcpserver"
	"github.com/nathanaday/atlas-obsidian/internal/migrate"
	"github.com/nathanaday/atlas-obsidian/internal/search"
	"github.com/nathanaday/atlas-obsidian/internal/source"
	"github.com/nathanaday/atlas-obsidian/internal/thread"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Version is the binary's version; the build stamps it.
var Version = "dev"

// CLI is one run of the command.
type CLI struct {
	// moved are the notes the views step of this command moved out of views/.
	moved  []vault.Moved
	In     io.Reader
	Out    io.Writer
	Err    io.Writer
	Getenv func(string) string
	Now    func() time.Time
	Dir    string
}

// views writes the views after a write, and tells the user, on stderr so JSON output
// stays clean, where each note found in views/ went.
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

const usage = `atlas-obsidian: one vault for what you know, the threads you work on, and what happened.

Usage:
  atlas-obsidian vault [status|init|sync [--views]|mention|migrate [--dry-run]]
  atlas-obsidian search TEXT [--type T]... [--kind K]... [--tag T]... [--status S]... [--repository R] [--limit N]
  atlas-obsidian context [REPOSITORY] [--tag T]... [--path P]
  atlas-obsidian match --items FILE.json | --docs ID... [--tag T]... [--across]
  atlas-obsidian source capture [--inbox NAME]... | [--text FILE --title T] | [--repository R] [--tag T]... [--resolves STUB]
  atlas-obsidian source chunks DOC
  atlas-obsidian source read DOC CHUNK
  atlas-obsidian change propose FILE.json | show ID | apply ID | reject ID --reason R | undo ID
  atlas-obsidian thread [list] | load THREAD | stub TEXT... [--title T] [--chord C] [--after T]...
                        | spec THREAD FILE.md | tasks THREAD FILE.json [--repository R] | start THREAD [--take]
                        | check THREAD TASK [--state S] [--commit C]... [--note N] [--reason R]
                        | verify THREAD FILE.json | finding THREAD FINDING --outcome O [--reason R] [--link L] [--task FILE.json]
                        | drop THREAD --reason R | reopen THREAD | block THREAD --reason R | unblock THREAD
                        | resolve STUB --became DOC... | note DOC --text T
                        | set DOC [--title T] [--priority P] [--tag T]... [--chord C] [--after T]...
  atlas-obsidian chord [list] | load CHORD | create FILE.json | add CHORD THREAD [--after T]... | remove CHORD THREAD
                       | order CHORD FILE.json | drop CHORD --reason R | reopen CHORD
                       | canvas CHORD [--save | --write | --tidy]
  atlas-obsidian lint [--tag T]...
  atlas-obsidian hook EVENT                        a hook; reads the event JSON on stdin
  atlas-obsidian mcp                               the MCP server, over stdio
  atlas-obsidian config [show] | set KEY VALUE [--global] | unset KEY [--global]
                                                   agent preferences: the vault's file wins over ~/.atlas/config.json
  atlas-obsidian setup | doctor | version | open [DOC]

Every command takes --vault (a path, or a name from ~/.atlas/config.json) and --json.
`

// Run runs the command and returns its exit code.
func (c *CLI) Run(argv []string) int {
	if len(argv) == 0 {
		fmt.Fprint(c.Out, usage)
		return 0
	}
	cmd, rest := argv[0], argv[1:]
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
	case "thread":
		err = c.threadCmd(rest)
	case "chord":
		err = c.chordCmd(rest)
	case "lint":
		err = c.lintCmd(rest)
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
		fmt.Fprint(c.Out, usage)
	default:
		err = fmt.Errorf("no command %q; atlas-obsidian help lists them", cmd)
	}
	if err != nil {
		fmt.Fprintln(c.Err, "atlas: "+err.Error())
		return 1
	}
	return 0
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

func (a args) ptr(name string) *string {
	if !a.has(name) {
		return nil
	}
	v := a.get(name)
	return &v
}

func (a args) listPtr(name string) *[]string {
	if !a.has(name) {
		return nil
	}
	l := a.list(name)
	return &l
}

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

// withMoved adds moved_from_views to a JSON result, whatever its type, so a caller that
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
	out["moved_from_views"] = moved
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
func (c *CLI) user() thread.Opts { return thread.Opts{Now: c.Now(), By: thread.ByUser} }

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
			fmt.Fprintf(w, "Vault %s is ready at %s.\nOpen it in Obsidian (atlas-obsidian open) and turn on the Atlas plugin under Community plugins.\n", st.Vault.Name, st.Vault.Path)
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
			fmt.Fprintf(w, "Synced: %s, %s, %d moved back, %s, %s, %s", count(len(s.Threads), "thread document", "thread documents"), count(len(s.Knowledge), "knowledge document", "knowledge documents"), len(s.Moved), count(len(s.Lost), "lost session", "lost sessions"), count(len(s.Sessions), "session callout", "session callouts"), count(s.Views, "view", "views"))
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
	case "mention":
		v, err := c.open(a)
		if err != nil {
			return err
		}
		line, _ := strconv.Atoi(a.get("line"))
		m, err := core.CloseMention(v, a.get("note"), line, a.get("link"))
		if err != nil {
			return err
		}
		c.views(v, now)
		return c.emit(a, map[string]any{"mention": m}, func(w io.Writer) { fmt.Fprintf(w, "Closed: %s\n", m.Text) })
	case "migrate":
		v, err := c.open(a)
		if err != nil {
			return err
		}
		if a.has("dry-run") {
			r, err := migrate.Plan(v, now)
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
	return fmt.Errorf("vault takes status, init, sync, mention, or migrate, not %q", a.arg(0))
}

func printMigration(w io.Writer, r *migrate.Report, done bool) {
	if done {
		fmt.Fprintf(w, "Migrated %s from the %s layout to 8.0 in one commit, %s.\n", r.Vault, r.From, short(r.Commit))
	} else {
		fmt.Fprintf(w, "The migration of %s from the %s layout to 8.0 would make these moves; run it without --dry-run to write them.\n", r.Vault, r.From)
	}
	if r.Documents+r.Events+r.Assets > 0 {
		fmt.Fprintf(w, "Documents: %d to wiki/documents · %d events written · %d originals to wiki/assets\n", r.Documents, r.Events, r.Assets)
	}
	if !(r.From == "6.x" && !done) {
		fmt.Fprintf(w, "Plans: %s · %s · %s · %s · %s · %s kept as notes\n", count(r.Threads, "thread", "threads"), count(r.Specs, "spec", "specs"), count(r.TaskLists, "task list", "task lists"), count(r.Verifications, "verification", "verifications"), count(r.Chords, "chord", "chords"), count(r.Notes, "set of sections", "sets of sections"))
		if r.Topics > 0 {
			fmt.Fprintf(w, "Designs: %s\n", count(r.Topics, "draft topic", "draft topics"))
		}
	}
	for _, t := range r.Tags {
		fmt.Fprintf(w, "  tag  %s ← %s\n", t.Tag, t.Scope)
	}
	for _, rt := range r.Retitles {
		fmt.Fprintf(w, "  title  %s → %s\n", rt.Old, rt.New)
	}
	for _, p := range r.Inbox {
		fmt.Fprintf(w, "  to the inbox  %s\n", p)
	}
	for _, p := range r.Scratchpad {
		fmt.Fprintf(w, "  to the scratchpad  %s\n", p)
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
	for _, t := range []string{"topic", "source", "repository", "stub", "chord", "event"} {
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
	t := st.Threads
	fmt.Fprintf(w, "Threads: %d started · %d verified · %d ready · %d blocked · %d waiting · %s · %s · %d active\n", t.Started, t.Verified, t.Ready, t.Blocked, t.Waiting, count(t.Stubs, "stub", "stubs"), count(t.Chords, "open chord", "open chords"), len(t.Active))
	fmt.Fprintf(w, "Sessions: %d running · %d waiting · %d idle\n", len(st.Sessions.Running), len(st.Sessions.Waiting), len(st.Sessions.Idle))
	for _, s := range st.Sessions.Waiting {
		fmt.Fprintf(w, "  waits for you: %s %s\n", s.Title, s.Description)
	}
	fmt.Fprintf(w, "Inbox: %d · Pending: %d · Proposed changes: %d · Mentions: %d · Problems: %d\n", len(st.Inbox), len(st.Pending), len(st.Changes.Proposed), len(st.Mentions), st.Problems)
	for _, c := range st.Changes.Proposed {
		fmt.Fprintf(w, "  proposed: %s (%s)\n", c.Title, c.ID)
	}
	for _, m := range st.Mentions {
		fmt.Fprintf(w, "  mention: %s:%d %s\n", m.Doc.Path, m.Line, m.Text)
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
		for _, wk := range b.Work {
			fmt.Fprintf(w, "Thread: %s · %s\n", wk.Title, wk.Status)
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
		v, err := c.open(a)
		if err != nil {
			return err
		}
		req := source.Request{Inbox: a.list("inbox"), Title: a.get("title"), Repository: a.get("repository"), Tags: a.list("tag"), Locator: a.get("locator"), Resolves: a.get("resolves"), NewTags: a.has("new-tags")}
		if a.has("text") {
			text, err := c.readText(a.get("text"))
			if err != nil {
				return err
			}
			req.Text = text
		}
		res, err := source.Capture(v, req, c.user())
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
		pv, err = change.Propose(v, plan, now)
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
		return fmt.Errorf("change takes propose, show, apply, reject, or undo, not %q", a.arg(0))
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

func (c *CLI) threadCmd(argv []string) error {
	a := parse(argv, "take", "new-tags")
	v, err := c.open(a)
	if err != nil {
		return err
	}
	o := c.user()
	show := func(r *thread.Result, err error) error {
		if err != nil {
			return err
		}
		c.views(v, c.Now())
		return c.emit(a, r, func(w io.Writer) { printResult(w, r) })
	}
	switch a.arg(0) {
	case "", "list":
		idx, err := vault.Load(v)
		if err != nil {
			return err
		}
		b := thread.Load(idx).BoardView(thread.Filter{Tags: a.list("tag"), Repository: a.get("repository"), Chord: a.get("chord")})
		return c.emit(a, map[string]any{"board": b}, func(w io.Writer) { printBoard(w, b) })
	case "load":
		idx, err := vault.Load(v)
		if err != nil {
			return err
		}
		loaded, err := thread.LoadThread(idx, a.arg(1))
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"thread": loaded}, func(w io.Writer) { printLoaded(w, loaded) })
	case "stub":
		text := strings.Join(a.pos[1:], " ")
		if text == "" || text == "-" {
			if text, err = c.readText("-"); err != nil {
				return err
			}
		}
		return show(thread.Stub(v, thread.StubIn{Text: text, Title: a.get("title"), Description: a.get("description"), Tags: a.list("tag"), Priority: a.get("priority"), Chord: a.get("chord"), After: a.list("after"), Inbox: a.get("inbox"), NewTags: a.has("new-tags")}, o))
	case "spec":
		text, err := c.readText(a.arg(2))
		if err != nil {
			return err
		}
		return show(thread.Spec(v, thread.SpecIn{Thread: a.arg(1), Text: text, Description: a.get("description")}, o))
	case "tasks":
		var tasks []thread.TaskIn
		if err := c.readJSON(a.arg(2), &tasks); err != nil {
			return fmt.Errorf("the tasks: %w", err)
		}
		return show(thread.TasksWrite(v, thread.TasksIn{Thread: a.arg(1), Repository: a.get("repository"), Tasks: tasks}, o))
	case "start":
		return show(thread.Start(v, a.arg(1), a.has("take"), o))
	case "check":
		return show(thread.Check(v, thread.CheckIn{Thread: a.arg(1), Task: a.arg(2), State: a.get("state"), Commits: a.list("commit"), Note: a.get("note"), Reason: a.get("reason")}, o))
	case "verify":
		var in thread.VerifyIn
		if err := c.readJSON(a.arg(2), &in); err != nil {
			return fmt.Errorf("the verification: %w", err)
		}
		in.Thread = a.arg(1)
		return show(thread.Verify(v, in, o))
	case "finding":
		in := thread.FindingIn{Thread: a.arg(1), Finding: a.arg(2), Outcome: a.get("outcome"), Repository: a.get("repository"), Link: a.get("link"), Text: a.get("text"), Reason: a.get("reason")}
		if a.has("task") {
			in.Task = &thread.TaskIn{}
			if err := c.readJSON(a.get("task"), in.Task); err != nil {
				return fmt.Errorf("the task: %w", err)
			}
		}
		return show(thread.FindingOutcome(v, in, o))
	case "drop":
		return show(thread.Drop(v, a.arg(1), a.get("reason"), o))
	case "reopen":
		return show(thread.Reopen(v, a.arg(1), a.get("reason"), o))
	case "block":
		return show(thread.Block(v, a.arg(1), a.get("reason"), o))
	case "unblock":
		return show(thread.Unblock(v, a.arg(1), o))
	case "resolve":
		return show(thread.Resolve(v, a.arg(1), a.list("became"), o))
	case "note":
		return show(thread.Note(v, a.arg(1), a.get("text"), o))
	case "set":
		return show(thread.Set(v, thread.SetIn{Doc: a.arg(1), Title: a.ptr("title"), Description: a.ptr("description"), Priority: a.ptr("priority"), Chord: a.ptr("chord"), Tags: a.listPtr("tag"), Aliases: a.listPtr("alias"), After: a.listPtr("after"), NewTags: a.has("new-tags")}, o))
	}
	return fmt.Errorf("thread takes list, load, stub, spec, tasks, start, check, verify, finding, drop, reopen, block, unblock, resolve, note, or set, not %q", a.arg(0))
}

func (c *CLI) chordCmd(argv []string) error {
	a := parse(argv, "save", "write", "tidy", "new-tags")
	if err := a.removed("new-tags", `chord takes new_tags only in the JSON of chord create, as "new_tags": true`); err != nil {
		return err
	}
	v, err := c.open(a)
	if err != nil {
		return err
	}
	o := c.user()
	show := func(r *thread.Result, err error) error {
		if err != nil {
			return err
		}
		c.views(v, c.Now())
		return c.emit(a, r, func(w io.Writer) { printResult(w, r) })
	}
	switch a.arg(0) {
	case "", "list":
		idx, err := vault.Load(v)
		if err != nil {
			return err
		}
		chords := thread.Load(idx).BoardView(thread.Filter{Tags: a.list("tag")}).Chords
		return c.emit(a, map[string]any{"chords": chords}, func(w io.Writer) {
			for _, cv := range chords {
				printChord(w, cv)
			}
		})
	case "load":
		idx, err := vault.Load(v)
		if err != nil {
			return err
		}
		loaded, err := thread.LoadChord(idx, a.arg(1))
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"chord": loaded}, func(w io.Writer) {
			printChord(w, loaded.ChordView)
			if loaded.Canvas != nil && loaded.Canvas.Differs {
				fmt.Fprintf(w, "  the canvas differs from the stubs: %s\n", strings.Join(loaded.Canvas.Changes, "; "))
			}
		})
	case "create":
		var in thread.ChordIn
		if err := c.readJSON(a.arg(1), &in); err != nil {
			return fmt.Errorf("the chord: %w", err)
		}
		return show(thread.ChordCreate(v, in, o))
	case "add":
		return show(thread.ChordAdd(v, a.arg(1), a.arg(2), a.list("after"), o))
	case "remove":
		return show(thread.ChordRemove(v, a.arg(1), a.arg(2), o))
	case "order":
		var order []thread.OrderIn
		if err := c.readJSON(a.arg(2), &order); err != nil {
			return fmt.Errorf("the order: %w", err)
		}
		return show(thread.ChordOrder(v, a.arg(1), order, o))
	case "drop":
		return show(thread.Drop(v, a.arg(1), a.get("reason"), o))
	case "reopen":
		return show(thread.Reopen(v, a.arg(1), a.get("reason"), o))
	case "canvas":
		switch {
		case a.has("save"):
			return show(thread.CanvasSave(v, a.arg(1), o))
		case a.has("write"), a.has("tidy"):
			return show(thread.CanvasWrite(v, a.arg(1), a.has("tidy"), o))
		}
		idx, err := vault.Load(v)
		if err != nil {
			return err
		}
		state, err := thread.CanvasStatus(idx, a.arg(1))
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"canvas": state}, func(w io.Writer) {
			switch {
			case !state.Exists:
				fmt.Fprintf(w, "%s has no canvas yet.\n", state.Chord.Title)
			case !state.Differs:
				fmt.Fprintf(w, "%s shows the order the stubs hold.\n", state.Path)
			default:
				fmt.Fprintf(w, "%s differs from the stubs. --save would:\n", state.Path)
				for _, ch := range state.Changes {
					fmt.Fprintf(w, "  %s\n", ch)
				}
			}
		})
	}
	return fmt.Errorf("chord takes list, load, create, add, remove, order, drop, reopen, or canvas, not %q", a.arg(0))
}

func threadLine(r vault.Ref) string {
	line := "  " + r.Title + " (" + r.ID + ") · " + r.Status
	if n, _ := r.State["tasks"].(string); n != "" {
		line += " · " + n + " tasks"
	}
	if p, _ := r.State["priority"].(string); p != "normal" && p != "" {
		line += " · " + p
	}
	if c, _ := r.State["chord"].(string); c != "" {
		line += " · chord " + c
	}
	if bl, _ := r.State["blocked"].(string); bl != "" {
		line += " · blocked: " + bl
	}
	return line
}

func printBoard(w io.Writer, b *thread.BoardView) {
	group := func(label string, refs []vault.Ref) {
		if len(refs) == 0 {
			return
		}
		fmt.Fprintf(w, "%s (%d)\n", label, len(refs))
		for _, r := range refs {
			fmt.Fprintln(w, threadLine(r))
		}
	}
	group("active", b.Active)
	group("started", b.Started)
	group("verified, waiting for the wiki change", b.Verified)
	group("ready", b.Ready)
	group("blocked", b.Blocked)
	group("waiting on other threads", b.Waiting)
	group("stubs", b.Stubs)
	group("ended lately", b.Ended)
	for _, cv := range b.Chords {
		printChord(w, cv)
	}
}

func printChord(w io.Writer, cv thread.ChordView) {
	fmt.Fprintf(w, "chord %s (%s) · %s · %v threads closed · next: %s\n", cv.Chord.Title, cv.Chord.ID, cv.Chord.Status, cv.Chord.State["threads"], cv.Next.Reason)
	for _, r := range cv.Threads {
		line := threadLine(r)
		if after, ok := r.State["after"].([]string); ok && len(after) > 0 {
			line += " · after " + strings.Join(after, ", ")
		}
		if ready, _ := r.State["ready"].(bool); ready && (r.Status == thread.StatusStub || r.Status == thread.Specified || r.Status == thread.Planned) {
			line += " · ready"
		}
		fmt.Fprintln(w, line)
	}
}

func printState(w io.Writer, st *thread.State) {
	if st == nil {
		return
	}
	fmt.Fprintln(w, strings.TrimPrefix(threadLine(st.Thread), "  "))
	if len(st.Missing) > 0 {
		fmt.Fprintf(w, "  missing: %s\n", strings.Join(st.Missing, " · "))
	}
	next := st.Next.Reason
	if st.Next.Skill != "" {
		next += " (" + st.Next.Skill + ")"
	}
	fmt.Fprintf(w, "  next: %s\n", next)
}

func printResult(w io.Writer, r *thread.Result) {
	printState(w, r.State)
	if r.Chord != nil {
		printChord(w, *r.Chord)
	}
	for _, d := range r.Wrote {
		fmt.Fprintf(w, "  wrote    %s (%s)\n", d.Title, d.Type)
	}
	for _, e := range r.Events {
		fmt.Fprintf(w, "  event    %s\n", e.Title)
	}
}

func printLoaded(w io.Writer, l *thread.Loaded) {
	printState(w, &thread.State{Thread: l.Thread, Missing: l.Missing, Next: l.Next})
	if l.Blocked != "" {
		fmt.Fprintf(w, "  blocked: %s\n", l.Blocked)
	}
	if l.Spec != nil {
		fmt.Fprintf(w, "  spec     %s · %d requirements\n", l.Spec.Ref.Title, len(l.Requirements))
	}
	for _, list := range l.Lists {
		fmt.Fprintf(w, "  tasks    %s · %s\n", list.Ref.Title, list.Ref.Status)
		for _, t := range list.Open {
			fmt.Fprintf(w, "    [ ] %s: %s\n", t.ID, t.Text)
		}
	}
	if v := l.Verification; v != nil {
		fmt.Fprintf(w, "  verified %s · %s\n", v.Ref.Title, v.Verdict)
		for _, f := range v.Findings {
			fmt.Fprintf(w, "    [ ] %s: %s\n", f.ID, f.Text)
		}
	}
	for _, s := range l.Sessions {
		fmt.Fprintf(w, "  session  %s · %s\n", s.Ref.Title, s.Ref.Status)
	}
	fmt.Fprintf(w, "  hand off %s\n", l.Handoff)
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
