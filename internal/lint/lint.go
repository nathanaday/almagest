// Package lint is the health check. It reads every typed document and reports what is
// wrong, and which tool or skill fixes it. It never writes.
package lint

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/links"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Severities.
const (
	Error   = "error"
	Warning = "warning"
	Info    = "info"
)

// Thresholds of the info checks.
const (
	BehindLimit  = 50
	PendingAge   = 7 * 24 * time.Hour
	ProposedAge  = 24 * time.Hour
	writeSection = "Writes"
)

// Finding is one thing wrong, and its fix.
type Finding struct {
	Check    string    `json:"check"`
	Severity string    `json:"severity"`
	Doc      vault.Ref `json:"doc"`
	Message  string    `json:"message"`
	Fix      string    `json:"fix"`
}

// Findings is the output of lint.
type Findings struct {
	Findings []Finding      `json:"findings"`
	Counts   map[string]int `json:"counts"`
	Checked  int            `json:"checked"`
}

// Options select a run.
type Options struct {
	// Scope limits the run to one scope and the scopes below it; empty is the vault.
	Scope string
	// Quick runs only the error checks that read frontmatter.
	Quick bool
	Now   time.Time
}

type run struct {
	idx  *vault.Index
	opts Options
	out  []Finding
}

func (r *run) add(check, severity string, d *doc.Doc, fix, format string, args ...any) {
	r.out = append(r.out, Finding{Check: check, Severity: severity, Doc: r.idx.Ref(d), Message: fmt.Sprintf(format, args...), Fix: fix})
}

// Run checks the vault.
func Run(idx *vault.Index, opts Options) (*Findings, error) {
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	if opts.Scope != "" {
		s, err := idx.ResolveType(opts.Scope, "area", "repository")
		if err != nil {
			return nil, err
		}
		opts.Scope = s.ID()
	}
	r := &run{idx: idx, opts: opts}
	docs := r.selected()
	r.titles(docs)
	for _, d := range docs {
		r.schema(d)
		r.scope(d)
		if d.Type() == "repository" {
			r.repository(d)
		}
	}
	r.threads(docs)
	if !opts.Quick {
		incoming := r.incoming()
		for _, d := range docs {
			r.deadLinks(d)
			r.knowledge(d, incoming)
			r.info(d)
		}
	}
	sort.SliceStable(r.out, func(i, j int) bool {
		if a, b := rank(r.out[i].Severity), rank(r.out[j].Severity); a != b {
			return a < b
		}
		if r.out[i].Check != r.out[j].Check {
			return r.out[i].Check < r.out[j].Check
		}
		return r.out[i].Doc.Path < r.out[j].Doc.Path
	})
	counts := map[string]int{Error: 0, Warning: 0, Info: 0}
	for _, f := range r.out {
		counts[f.Severity]++
	}
	if r.out == nil {
		r.out = []Finding{}
	}
	return &Findings{Findings: r.out, Counts: counts, Checked: len(docs)}, nil
}

func rank(s string) int {
	switch s {
	case Error:
		return 0
	case Warning:
		return 1
	}
	return 2
}

// selected are the typed documents the run checks.
func (r *run) selected() []*doc.Doc {
	if r.opts.Scope == "" {
		return r.idx.Docs
	}
	var out []*doc.Doc
	for _, d := range r.idx.Docs {
		switch d.Type() {
		case "area", "repository":
			if r.idx.Under(d, r.opts.Scope) {
				out = append(out, d)
			}
		default:
			if r.idx.InScope(d, r.opts.Scope) {
				out = append(out, d)
			}
		}
	}
	return out
}

func (r *run) titles(docs []*doc.Doc) {
	in := map[string]bool{}
	for _, d := range docs {
		in[d.Path] = true
	}
	for _, d := range docs {
		if d.Path == vault.Marker {
			continue
		}
		others := without(r.idx.TitleHolders(d.Title()), d.Path)
		if len(others) > 0 {
			r.add("duplicate-title", Error, d, "wiki-edit: rename one of them", "its title %q is also held by %s", d.Title(), strings.Join(others, ", "))
		}
		for _, a := range d.List("aliases") {
			if holders := without(r.idx.TitleHolders(a), d.Path); len(holders) > 0 {
				r.add("duplicate-title", Error, d, "wiki-edit: drop the alias or rename the other document", "its alias %q is also held by %s", a, strings.Join(holders, ", "))
			}
		}
	}
}

func without(paths []string, p string) []string {
	var out []string
	for _, x := range paths {
		if x != p {
			out = append(out, x)
		}
	}
	return out
}

// fixFor names the tool or skill that repairs a document's schema.
func fixFor(d *doc.Doc) string {
	t := schema.Get(d.Type())
	switch {
	case t == nil:
		return "your edit"
	case t.Wiki():
		return "wiki-edit: a change that sets the field"
	case t.Family == schema.Thread:
		return "the thread tool: set, task set, or file"
	case d.Type() == "vault":
		return "edit Atlas.md"
	}
	return "vault sync"
}

// linkFields are the fields whose links lint checks one by one, with the check a broken
// one belongs to.
var scopeFields = map[string]bool{"scope": true, "parent": true, "repository": true}

func (r *run) schema(d *doc.Doc) {
	if d.FrontErr != nil {
		r.add("schema", Error, d, "your edit", "its frontmatter does not parse: %v", d.FrontErr)
		return
	}
	t := schema.Get(d.Type())
	for _, p := range t.Check(schema.Values(d.Front.Map()), nil) {
		r.add("schema", Error, d, fixFor(d), "%s", p)
	}
	for _, f := range t.Fields {
		if f.Kind != schema.Link && f.Kind != schema.Links {
			continue
		}
		for _, v := range d.List(f.Name) {
			check := "schema"
			if scopeFields[f.Name] {
				check = "scope"
			}
			typ, err := r.idx.TypeOfLink(v)
			switch {
			case err != nil:
				// Two files hold the title; duplicate-title reports it.
			case typ == "":
				if check == "schema" {
					check = "dead-link"
				}
				r.add(check, Error, d, fixFor(d), "%s: %s names no document", f.Name, v)
			case len(f.Targets) > 0 && !contains(f.Targets, typ):
				r.add(check, Error, d, fixFor(d), "%s: %s is a %s; it must be a %s", f.Name, v, typ, strings.Join(f.Targets, " or "))
			}
		}
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// scope checks where a page of the wiki lies: a scope page in a folder of its own, under
// an area's folder or the wiki's; a knowledge page in its type's folder of its scope.
func (r *run) scope(d *doc.Doc) {
	if !strings.HasPrefix(d.Path, vault.Wiki+"/") {
		return
	}
	title := vault.Title(d)
	switch d.Type() {
	case "area", "repository":
		if r.idx.Legacy() {
			r.add("layout", Error, d, "vault sync", "it lies in the old layout; sync moves every page into the folder of its scope")
			return
		}
		if vault.ReservedTitle(title) {
			r.add("layout", Error, d, "wiki-edit: rename it", "its title %q is the name of a type folder", title)
		}
		if r.idx.Folder(d) == "" {
			r.add("layout", Error, d, "move it into a folder of the same name, in Obsidian or the shell", "it lies at %s, outside a folder of its own (…/%s/%s.md), so no page belongs to it", d.Path, title, title)
			return
		}
		if p := r.idx.Parent(d); p != nil && p.Type() != "area" {
			r.add("layout", Error, d, "move its folder out of the repository's folder", "its folder lies in the folder of the repository %s; a repository holds no area or repository", vault.Title(p))
		}
	case "concept", "entity", "policy", "source":
		if r.idx.Legacy() || r.opts.Quick {
			return
		}
		want := r.idx.Folder(r.idx.Container(d.Path)) + "/" + vault.TypeFolders[d.Type()] + "/"
		if !strings.HasPrefix(d.Path, want) {
			r.add("layout", Warning, d, "move it into "+want, "it lies outside the %s folder of its scope", vault.TypeFolders[d.Type()])
		}
	}
}

func (r *run) repository(d *doc.Doc) {
	p := d.Str("path")
	if p == "" {
		return
	}
	abs := vault.Expand(p)
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		r.add("repository-path", Error, d, "repo-unlink, or wiki-edit: a change that sets the path", "its path %s is gone", p)
		return
	}
	if !gitx.IsRoot(abs) {
		r.add("repository-path", Error, d, "wiki-edit: a change that sets the path to the repository's root", "its path %s is not the root of a git work tree", p)
		return
	}
	if vault.Within(abs, r.idx.V.Root) {
		r.add("repository-path", Error, d, "repo-unlink", "its path %s is inside the vault", p)
	}
}

// threads checks each thread's documents against the rules the thread tool enforces.
func (r *run) threads(docs []*doc.Doc) {
	stubs := map[string]*doc.Doc{}
	for _, s := range r.idx.Of("stub") {
		stubs[s.ID()] = s
	}
	specs, receipts := map[string][]*doc.Doc{}, map[string][]*doc.Doc{}
	for _, d := range r.idx.Of("spec", "task", "receipt") {
		id := d.Str("thread_id")
		if stubs[id] == nil {
			if stub := r.idx.Linked(d.Str("thread")); stub != nil && stub.Type() == "stub" {
				id = stub.ID()
			}
		}
		if stubs[id] == nil {
			r.add("thread", Error, d, "the thread tool: file it on a thread, or delete it", "its thread %s is not a stub", orNone(d.Str("thread_id")))
			continue
		}
		switch d.Type() {
		case "spec":
			specs[id] = append(specs[id], d)
		case "receipt":
			if o := d.Str("outcome"); o != "completed" && o != "killed" {
				r.add("thread", Error, d, "the thread tool: reopen the thread and file the receipt again", "its outcome is %q; it must be completed or killed", o)
			}
			if !d.Front.Bool("superseded") {
				receipts[id] = append(receipts[id], d)
			}
		}
	}
	for id, list := range specs {
		if len(list) > 1 {
			r.add("thread", Error, list[1], "merge the specs by hand and delete one", "thread %s has %d specs; it may have one", vault.Title(stubs[id]), len(list))
		}
	}
	for id, list := range receipts {
		if len(list) > 1 {
			r.add("thread", Error, list[1], "delete the extra receipt, or reopen the thread", "thread %s has %d receipts; it may have one", vault.Title(stubs[id]), len(list))
		}
	}
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

// incoming counts, for each vault path, the documents that link to it.
func (r *run) incoming() map[string]int {
	counts := map[string]int{}
	all := append(append([]*doc.Doc{}, r.idx.Docs...), r.idx.Notes...)
	for _, d := range all {
		seen := map[string]bool{}
		for _, target := range docLinks(d) {
			for _, p := range r.idx.LinkPaths(target) {
				if p != d.Path && !seen[p] {
					counts[p]++
					seen[p] = true
				}
			}
		}
	}
	return counts
}

// docLinks are the link targets of a document: its frontmatter's and its body's, without
// the copies a change document holds under Writes.
func docLinks(d *doc.Doc) []string {
	var out []string
	if d.Front != nil {
		for _, k := range d.Front.Keys() {
			for _, v := range d.Front.List(k) {
				if doc.IsLink(v) {
					out = append(out, doc.LinkTarget(v))
				}
			}
		}
	}
	for _, l := range links.Find(Checked(d)) {
		out = append(out, l.Target)
	}
	return out
}

// Checked is the part of a document's body whose links count: all of it, but a change
// document's Writes, which hold copies of pages.
func Checked(d *doc.Doc) string {
	if d.Type() != "change" {
		return d.Body
	}
	return WithoutWrites(d.Body)
}

// WithoutWrites cuts a change document's Writes section out of its body.
func WithoutWrites(body string) string {
	for _, h := range doc.Headings(body) {
		if h.Level == 2 && strings.EqualFold(h.Title, writeSection) {
			lines := strings.Split(body, "\n")
			return strings.Join(lines[:h.Line], "\n")
		}
	}
	return body
}

func (r *run) deadLinks(d *doc.Doc) {
	seen := map[string]bool{}
	for _, l := range links.Find(Checked(d)) {
		if l.Target == "" || seen[l.Target] {
			continue
		}
		seen[l.Target] = true
		if len(r.idx.LinkPaths(l.Target)) == 0 {
			r.add("dead-link", Error, d, "wiki-edit: create the page, or change the link", "line %d links [[%s]], which resolves to nothing", l.Line, l.Target)
		}
	}
}

func (r *run) knowledge(d *doc.Doc, incoming map[string]int) {
	t := schema.Get(d.Type())
	if t == nil || t.Family != schema.Knowledge {
		return
	}
	if incoming[d.Path] == 0 && !r.idx.Pending(d) {
		r.add("orphan", Warning, d, "wiki-edit: link it from a related page, or remove it", "no other document links to it")
	}
	if d.Type() != "source" && len(d.List("sources")) == 0 {
		r.add("uncited", Warning, d, "wiki-edit: cite the documents it rests on", "its sources are empty")
	}
}

func (r *run) info(d *doc.Doc) {
	now := r.opts.Now
	switch d.Type() {
	case "repository":
		if described := d.Str("described"); described != "" {
			g := gitx.Repo{Dir: vault.Expand(d.Str("path"))}
			if n, err := g.Behind(described); err == nil && n > BehindLimit {
				r.add("repository-behind", Info, d, "repo-ingest", "its page describes %s, %d commits behind the head", described[:min(7, len(described))], n)
			}
		}
	case "change":
		if d.Str("status") == "proposed" {
			if t, ok := vault.ParseTime(d.Str("proposed")); ok && now.Sub(t) > ProposedAge {
				r.add("change-stale", Info, d, "apply or reject it", "proposed %s and still waiting", d.Str("proposed"))
			}
		}
	case "session":
		if d.Str("status") == "lost" {
			for _, task := range d.List("tasks") {
				if t := r.idx.Linked(task); t != nil && t.Str("status") == "open" {
					r.add("session-lost", Info, d, "thread-run: pick the task up again", "it was lost while it held the open task %s", vault.Title(t))
					break
				}
			}
		}
	}
	if r.idx.Pending(d) {
		if t, ok := vault.ParseTime(d.Str("created")); ok && now.Sub(t) > PendingAge {
			r.add("pending", Info, d, "wiki-sync", "the wiki has not absorbed it since %s", d.Str("created"))
		}
	}
}
