// Package cli is the atlas command: one subcommand per tool action, the hooks, the MCP
// server, and the commands no tool needs (setup, doctor, version, open). Every command
// reaches the same function its tool does.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/change"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/core"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/hooks"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/lint"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/match"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/mcpserver"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/scope"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/search"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/source"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/threads"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/vault"
)

// Version is the binary's version; the build stamps it.
var Version = "dev"

// CLI is one run of the command.
type CLI struct {
	In     io.Reader
	Out    io.Writer
	Err    io.Writer
	Getenv func(string) string
	Now    func() time.Time
	Dir    string
}

// New is the command as a process runs it.
func New() *CLI {
	dir, _ := os.Getwd()
	return &CLI{In: os.Stdin, Out: os.Stdout, Err: os.Stderr, Getenv: os.Getenv, Now: time.Now, Dir: dir}
}

const usage = `atlas: one vault for the wiki, the threads, the sessions, and the changes of your work.

Usage:
  atlas vault [status|init|sync|mention]      the state of the vault; make, heal, or answer
  atlas search TEXT [--type T]... [--scope S] [--state K=V]... [--limit N]
  atlas context [SCOPE] [--path P]            walk the context graph to a scope
  atlas match --items FILE.json | --pages ID... [--within S] [--siblings]
  atlas source capture [--inbox NAME]... | [--text FILE --title T] | [--repository R] [--scope S]
  atlas source chunks DOC
  atlas source read DOC CHUNK
  atlas change propose FILE.json | show ID | apply ID | reject ID --reason R | undo ID
  atlas thread [list] | show T | open TEXT... | attach T | file T PART | tasks T FILE.json
               | task ID DO | set T ... | reopen T
  atlas lint [SCOPE]
  atlas hook EVENT                            a hook; reads the event JSON on stdin
  atlas mcp                                   the MCP server, over stdio
  atlas setup | doctor | version | open [DOC]

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
	case "lint":
		err = c.lintCmd(rest)
	case "hook":
		return c.hookCmd(rest)
	case "mcp":
		err = c.mcpCmd()
	case "setup":
		err = c.setupCmd(rest)
	case "doctor":
		return c.doctorCmd(rest)
	case "version", "--version", "-v":
		fmt.Fprintln(c.Out, "atlas "+Version)
	case "open":
		err = c.openCmd(rest)
	case "help", "--help", "-h":
		fmt.Fprint(c.Out, usage)
	default:
		err = fmt.Errorf("no command %q; atlas help lists them", cmd)
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

func (a args) arg(i int) string {
	if i < len(a.pos) {
		return a.pos[i]
	}
	return ""
}

func (c *CLI) home() vault.Home { return vault.HomeFrom(c.Getenv) }

func (c *CLI) open(a args) (*vault.Vault, error) {
	return vault.Resolve(a.get("vault"), c.Dir, c.home())
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
		enc := json.NewEncoder(c.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	human(c.Out)
	return nil
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

func (c *CLI) vaultCmd(argv []string) error {
	a := parse(argv)
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
		st, err := core.Init(vault.InitOptions{Path: p, Name: a.get("name"), Description: a.get("description"), Areas: a.get("areas")}, c.home(), now)
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"status": st}, func(w io.Writer) {
			fmt.Fprintf(w, "Vault %s is ready at %s.\nOpen it in Obsidian (atlas open) and turn on the Atlas plugin under Community plugins.\n", st.Vault.Name, st.Vault.Path)
		})
	case "sync":
		v, err := c.open(a)
		if err != nil {
			return err
		}
		s, err := core.Sync(v, now)
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"synced": s}, func(w io.Writer) {
			n := len(s.Threads) + len(s.Lost) + len(s.Sessions) + len(s.Scopes)
			if n == 0 && !s.Settings {
				fmt.Fprintln(w, "Nothing to heal.")
				return
			}
			fmt.Fprintf(w, "Synced: %d thread documents, %d lost sessions, %d session callouts, %d wiki pages and the map", len(s.Threads), len(s.Lost), len(s.Sessions), len(s.Scopes))
			if s.Settings {
				fmt.Fprint(w, ", the harness settings")
			}
			fmt.Fprintln(w, ".")
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
		return c.emit(a, map[string]any{"mention": m}, func(w io.Writer) { fmt.Fprintf(w, "Closed: %s\n", m.Text) })
	}
	return fmt.Errorf("vault takes status, init, sync, or mention, not %q", a.arg(0))
}

// count is a number and its noun, singular for one.
func count(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func printStatus(w io.Writer, st *core.Status) {
	fmt.Fprintf(w, "%s · %s · %s\n", st.Vault.Name, st.Vault.Path, st.Vault.ID)
	fmt.Fprintf(w, "Scopes: %s, %s\n", count(st.Scopes.Areas, "area", "areas"), count(st.Scopes.Repositories, "repository", "repositories"))
	var pages []string
	for _, t := range []string{"concept", "entity", "policy", "source"} {
		pages = append(pages, fmt.Sprintf("%d %s", st.Wiki.Pages[t], t))
	}
	fmt.Fprintf(w, "Wiki: %s · %d draft · %d contested\n", strings.Join(pages, ", "), st.Wiki.Draft, st.Wiki.Contested)
	o := st.Threads.Open
	fmt.Fprintf(w, "Threads: %d open (tasks %d, spec %d, stub %d) · %d active\n", o["tasks"]+o["spec"]+o["stub"], o["tasks"], o["spec"], o["stub"], len(st.Threads.Active))
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
	q := search.Query{Text: strings.Join(a.pos, " "), Types: a.list("type"), Scope: a.get("scope"), State: map[string][]string{}}
	for _, kv := range a.list("state") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return fmt.Errorf("--state takes key=value, not %q", kv)
		}
		q.State[k] = append(q.State[k], strings.Split(v, ",")...)
	}
	q.Limit, _ = strconv.Atoi(a.get("limit"))
	hits, err := search.Search(idx, q)
	if err != nil {
		return err
	}
	return c.emit(a, hits, func(w io.Writer) {
		for _, h := range hits.Hits {
			fmt.Fprintf(w, "%6.2f  %-10s %s  (%s)\n", h.Score, h.Ref.Type, h.Ref.Title, h.Ref.ID)
			if h.Snippet != "" {
				fmt.Fprintf(w, "        %s\n", h.Snippet)
			}
		}
		fmt.Fprintf(w, "%d of %d\n", len(hits.Hits), hits.Total)
	})
}

func (c *CLI) contextCmd(argv []string) error {
	a := parse(argv)
	idx, err := c.index(a)
	if err != nil {
		return err
	}
	ch, err := scope.Context(idx, a.arg(0), a.get("path"))
	if err != nil {
		return err
	}
	return c.emit(a, ch, func(w io.Writer) {
		var chain []string
		for _, n := range ch.Chain {
			chain = append(chain, n.Ref.Title)
		}
		fmt.Fprintln(w, strings.Join(chain, " → "))
		list := func(label string, refs []vault.Ref) {
			if len(refs) == 0 {
				return
			}
			var names []string
			for _, r := range refs {
				names = append(names, r.Title)
			}
			fmt.Fprintf(w, "%s: %s\n", label, strings.Join(names, ", "))
		}
		list("Children", ch.Children)
		list("Repositories", ch.Repositories)
		for _, p := range ch.Policies {
			fmt.Fprintf(w, "Policy (%s): %s\n", orDash(p.Strength), p.Ref.Title)
		}
		list("Open threads", ch.Threads)
		for _, in := range ch.Instructions {
			fmt.Fprintf(w, "Instructions: %s\n", in.Path)
		}
		if r := ch.Repository; r != nil {
			fmt.Fprintf(w, "Repository: %s · %s · head %s · %d dirty", r.Path, r.Branch, r.Head, r.Dirty)
			if r.Behind >= 0 {
				fmt.Fprintf(w, " · %d commits past its page", r.Behind)
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
	a := parse(argv, "siblings")
	idx, err := c.index(a)
	if err != nil {
		return err
	}
	in := match.Input{Pages: append(a.list("pages"), a.pos...), Siblings: a.has("siblings"), Within: a.get("within")}
	if file := a.get("items"); file != "" {
		if err := c.readJSON(file, &in.Items); err != nil {
			return fmt.Errorf("--items: %w", err)
		}
		in.Pages = nil
	}
	m, err := match.Run(idx, in)
	if err != nil {
		return err
	}
	return c.emit(a, m, nil)
}

func (c *CLI) sourceCmd(argv []string) error {
	a := parse(argv)
	switch a.arg(0) {
	case "capture":
		v, err := c.open(a)
		if err != nil {
			return err
		}
		req := source.Request{Inbox: a.list("inbox"), Title: a.get("title"), Repository: a.get("repository"), Scope: a.get("scope"), Locator: a.get("locator")}
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
		pv, err = change.Apply(v, a.arg(1), now)
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
	return c.emit(a, pv, func(w io.Writer) { printPreview(w, pv) })
}

func printPreview(w io.Writer, pv *change.Preview) {
	fmt.Fprintf(w, "%s · %s · %s\n", pv.Ref.Title, pv.Status, pv.Counts.String())
	fmt.Fprintf(w, "  %s\n", pv.Ref.Path)
	for _, wl := range pv.Writes {
		line := fmt.Sprintf("  %-7s %s", wl.Op, wl.Title)
		if wl.Lines != "" {
			line += "  " + wl.Lines
		}
		if wl.Note != "" {
			line += "  (" + wl.Note + ")"
		}
		fmt.Fprintln(w, line)
	}
	for _, r := range pv.LinkRewrites {
		fmt.Fprintf(w, "  links   %s\n", r.Title)
	}
	for _, warn := range pv.Warnings {
		fmt.Fprintf(w, "  warning: %s\n", warn)
	}
	if pv.Reason != "" {
		fmt.Fprintf(w, "  reason: %s\n", pv.Reason)
	}
	if pv.Commit != "" {
		fmt.Fprintf(w, "  commit %s\n", pv.Commit[:min(10, len(pv.Commit))])
	}
}

func (c *CLI) threadCmd(argv []string) error {
	a := parse(argv, "take")
	v, err := c.open(a)
	if err != nil {
		return err
	}
	now := c.Now()
	show := func(r *threads.Result, err error) error {
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"view": r.View, "commit": r.Commit}, func(w io.Writer) { printView(w, r.View) })
	}
	switch a.arg(0) {
	case "", "list":
		idx, err := vault.Load(v)
		if err != nil {
			return err
		}
		b := threads.List(idx)
		return c.emit(a, map[string]any{"board": b}, func(w io.Writer) { printBoard(w, b) })
	case "show", "attach":
		idx, err := vault.Load(v)
		if err != nil {
			return err
		}
		view, err := threads.Show(idx, a.arg(1))
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"view": view}, func(w io.Writer) { printView(w, view) })
	case "open":
		text := strings.Join(a.pos[1:], " ")
		if text == "" || text == "-" {
			if text, err = c.readText("-"); err != nil {
				return err
			}
		}
		return show(threads.Open(v, threads.OpenIn{Text: text, Title: a.get("title"), Scope: a.list("scope"), Priority: a.get("priority"), Inbox: a.get("inbox")}, now))
	case "file":
		text, err := c.readText(a.get("text"))
		if err != nil {
			return err
		}
		return show(threads.File(v, a.arg(1), a.arg(2), text, a.get("outcome"), now))
	case "tasks":
		var tasks []threads.TaskIn
		if err := c.readJSON(a.arg(2), &tasks); err != nil {
			return fmt.Errorf("the tasks: %w", err)
		}
		return show(threads.Tasks(v, a.arg(1), tasks, now))
	case "task":
		do := threads.TaskDo{Do: a.arg(2), Result: a.get("result"), Take: a.has("take"), Title: a.ptr("title"), Repository: a.ptr("repository"), Blocked: a.ptr("blocked")}
		if a.has("depends") {
			deps := a.list("depends")
			do.Depends = &deps
		}
		if a.has("order") {
			n, err := strconv.Atoi(a.get("order"))
			if err != nil {
				return errors.New("--order takes a number")
			}
			do.Order = &n
		}
		return show(threads.Task(v, a.arg(1), a.get("thread"), do, now))
	case "set":
		set := threads.SetIn{Title: a.ptr("title"), Priority: a.ptr("priority"), Blocked: a.ptr("blocked")}
		if a.has("scope") {
			s := a.list("scope")
			set.Scope = &s
		}
		return show(threads.Set(v, a.arg(1), set, now))
	case "reopen":
		return show(threads.Reopen(v, a.arg(1), now))
	}
	return fmt.Errorf("thread takes list, show, open, attach, file, tasks, task, set, or reopen, not %q", a.arg(0))
}

func printBoard(w io.Writer, b *threads.BoardView) {
	for _, col := range b.Board {
		if len(col.Threads) == 0 {
			continue
		}
		fmt.Fprintf(w, "%s (%d)\n", col.Stage, len(col.Threads))
		for _, t := range col.Threads {
			line := "  " + t.Title
			if p, _ := t.State["priority"].(string); p != "normal" && p != "" {
				line += " · " + p
			}
			if n, _ := t.State["tasks"].(string); col.Stage == "tasks" {
				line += " · " + n
			}
			if a, _ := t.State["active"].(bool); a {
				line += " · active"
			}
			if bl, _ := t.State["blocked"].(string); bl != "" {
				line += " · blocked: " + bl
			}
			fmt.Fprintln(w, line)
		}
	}
	if len(b.Closed) > 0 {
		fmt.Fprintf(w, "closed (last %d)\n", len(b.Closed))
		for _, t := range b.Closed {
			fmt.Fprintf(w, "  %s · %v\n", t.Title, t.State["outcome"])
		}
	}
}

func printView(w io.Writer, v *threads.View) {
	if v == nil {
		return
	}
	fmt.Fprintf(w, "%s (%s) · %v · next: %s\n", v.Stub.Title, v.Stub.ID, v.Stub.State["stage"], v.Next)
	if v.Spec != nil {
		fmt.Fprintf(w, "  spec     %s\n", v.Spec.Title)
	}
	for _, t := range v.Tasks {
		ready := ""
		if t.Ready {
			ready = " · ready"
		}
		fmt.Fprintf(w, "  task     %s · %v%s\n", t.Ref.Title, t.Ref.State["status"], ready)
	}
	if v.Receipt != nil {
		fmt.Fprintf(w, "  receipt  %s\n", v.Receipt.Title)
	}
	for _, s := range v.Sessions {
		fmt.Fprintf(w, "  session  %s · %v\n", s.Title, s.State["status"])
	}
}

func (c *CLI) lintCmd(argv []string) error {
	a := parse(argv)
	idx, err := c.index(a)
	if err != nil {
		return err
	}
	f, err := lint.Run(idx, lint.Options{Scope: a.arg(0), Now: c.Now()})
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
		fmt.Fprintln(c.Err, "atlas hook takes one of: "+strings.Join(names, ", "))
		return 1
	}
	env := hooks.Env{Getenv: c.Getenv, Now: c.Now}
	if err := hooks.Run(argv[0], c.In, c.Out, env); err != nil {
		fmt.Fprintln(c.Err, "atlas hook "+argv[0]+": "+err.Error())
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
