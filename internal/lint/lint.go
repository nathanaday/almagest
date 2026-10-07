// Package lint is the health check. It reads every typed document and reports what is
// wrong, and which tool or skill fixes it. It never writes.
package lint

import (
	"cmp"
	"fmt"
	"os"
	"path"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/gitx"
	"github.com/nathanaday/almagest/internal/links"
	"github.com/nathanaday/almagest/internal/schema"
	"github.com/nathanaday/almagest/internal/tags"
	"github.com/nathanaday/almagest/internal/vault"
)

// Severities.
const (
	Error   = "error"
	Warning = "warning"
	Info    = "info"
)

// Thresholds of the info checks.
const (
	BehindLimit = 50
	PendingAge  = 7 * 24 * time.Hour
	ProposedAge = 24 * time.Hour
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
	// Tags limit the run to the documents that hold every one.
	Tags []string
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
	ref := r.idx.Ref(d)
	if ref.Type == "" || r.idx.ByID(d.ID()) != d {
		ref = vault.Ref{ID: d.ID(), Type: d.Type(), Title: vault.Title(d), Path: d.Path, Tags: []string{}}
	}
	r.out = append(r.out, Finding{Check: check, Severity: severity, Doc: ref, Message: fmt.Sprintf(format, args...), Fix: fix})
}

// Run checks the vault.
func Run(idx *vault.Index, opts Options) (*Findings, error) {
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	want, err := tags.NormalizeAll(opts.Tags)
	if err != nil {
		return nil, err
	}
	opts.Tags = want
	r := &run{idx: idx, opts: opts}
	docs := r.selected()
	r.titles(docs)
	for _, d := range docs {
		r.schema(d)
		if d.Type() == "repository" {
			r.repository(d)
		}
	}
	r.tagPages(docs)
	if len(want) == 0 {
		r.placement()
	}
	if !opts.Quick {
		incoming := r.incoming()
		for _, d := range docs {
			r.deadLinks(d)
			r.knowledge(d, incoming)
			r.info(d)
		}
		if len(want) == 0 {
			r.nearTags()
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

// selected are the typed documents the run checks: every one, or those that hold every
// tag of the run.
func (r *run) selected() []*doc.Doc {
	if len(r.opts.Tags) == 0 {
		return r.idx.Docs
	}
	var out []*doc.Doc
	for _, d := range r.idx.Documents() {
		if vault.Holds(d, r.opts.Tags...) {
			out = append(out, d)
		}
	}
	return out
}

func (r *run) titles(docs []*doc.Doc) {
	for _, d := range docs {
		if vault.IsMarker(d.Path) {
			continue
		}
		if schema.IsDocument(d.Type()) && vault.ReservedTitle(d.Title()) {
			r.add("duplicate-title", Error, d, fixFor(d), "its title begins as a view's title does (%q)", d.Title())
		}
		others := without(r.idx.TitleHolders(d.Title()), d.Path)
		if len(others) > 0 {
			r.add("duplicate-title", Error, d, fixFor(d)+": rename one of them", "its title %q is also held by %s", d.Title(), strings.Join(others, ", "))
		}
		for _, a := range d.List("aliases") {
			if holders := without(r.idx.TitleHolders(a), d.Path); len(holders) > 0 {
				r.add("duplicate-title", Error, d, fixFor(d)+": drop the alias or rename the other document", "its alias %q is also held by %s", a, strings.Join(holders, ", "))
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

// fixFor names the tool or skill that repairs a document.
func fixFor(d *doc.Doc) string {
	t := schema.Get(d.Type())
	switch {
	case t == nil:
		return "your edit"
	case t.Family == schema.Knowledge:
		return "wiki-edit (a change)"
	case d.Type() == "vault":
		return "edit Almagest.md"
	}
	return "vault sync"
}

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
			typ, err := r.idx.TypeOfLink(v)
			switch {
			case err != nil:
				// Two files hold the title; duplicate-title reports it.
			case typ == "":
				r.add("dead-link", Error, d, fixFor(d), "%s: %s names no document", f.Name, v)
			case len(f.Targets) > 0 && !slices.Contains(f.Targets, typ):
				r.add("schema", Error, d, fixFor(d), "%s: %s is a %s; it must be a %s", f.Name, v, typ, strings.Join(f.Targets, " or "))
			}
		}
	}
}

func (r *run) repository(d *doc.Doc) {
	p := d.Str("path")
	if p == "" || d.Front.Bool("unlinked") {
		return
	}
	abs := vault.Expand(p)
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		r.add("repository-path", Error, d, "repo-link with the new path, or repo-unlink", "its path %s is gone", p)
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

// tagPages checks that one document defines a tag, and that a tag page holds its tag's
// parent.
func (r *run) tagPages(docs []*doc.Doc) {
	in := map[string]bool{}
	for _, d := range docs {
		in[d.ID()] = true
	}
	for t, pages := range r.idx.TagPages() {
		if len(pages) > 1 && in[pages[1].ID()] {
			var names []string
			for _, p := range pages {
				names = append(names, p.Title())
			}
			r.add("tag", Error, pages[1], "wiki-edit: remove defines from all but one", "%d documents define %s: %s", len(pages), t, strings.Join(names, ", "))
		}
		for _, p := range pages {
			if parent := tags.Parent(t); parent != "" && in[p.ID()] && !r.opts.Quick && !tags.Holds(p.List("tags"), parent) {
				r.add("tag", Warning, p, "wiki-edit: add the tag "+parent, "it defines %s but does not hold %s", t, parent)
			}
		}
	}
	for _, d := range docs {
		if d.Type() == "topic" && d.Str("defines") != "" && d.Str("kind") != "overview" {
			r.add("tag", Error, d, "wiki-edit: make it an overview, or drop defines", "a %s defines %s; only an overview or a repository defines a tag", d.Str("kind"), d.Str("defines"))
		}
	}
}

// placement reports typed documents out of place, and files with no type in
// source-core/documents.
func (r *run) placement() {
	for _, d := range r.idx.Misplaced {
		fix := "move it into " + vault.Documents + " (sync does so for a file under " + vault.Core + "/)"
		if d.Type() == "session" || d.Type() == "change" {
			fix = "move it back under " + d.Type() + "s/"
		}
		r.add("misplaced", Error, d, fix, "a %s at %s, where tools do not see it", d.Type(), d.Path)
	}
	for _, d := range r.idx.Notes {
		switch {
		case path.Dir(d.Path) != vault.Documents:
		case slices.Contains(schema.ArchivedTypes, d.Type()):
			r.add("archived", Error, d, "move it into "+vault.Threads+"/", "a %s of Almagest 8.x in "+vault.Documents+"; Almagest keeps thread documents in %s/ and reads none of them", d.Type(), vault.Threads)
		case !r.opts.Quick:
			r.add("untyped", Warning, d, "wiki-ingest: capture it, or move it out of "+vault.Documents, "a note with no type in "+vault.Documents)
		}
	}
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
	for _, l := range links.Find(vault.Readable(d)) {
		out = append(out, l.Target)
	}
	return out
}

func (r *run) deadLinks(d *doc.Doc) {
	// A change that ran its course is a record: a link it holds may name a document a
	// later change removed.
	if d.Type() == "change" && d.Str("status") != "proposed" && d.Str("status") != "running" {
		return
	}
	text := vault.Readable(d)
	if d.Type() == "change" {
		// The Summary links what the change writes, which exists once it applies.
		text = doc.RemoveSection(text, "Summary")
	}
	seen := map[string]bool{}
	for _, l := range links.Find(text) {
		if l.Target == "" || seen[l.Target] {
			continue
		}
		seen[l.Target] = true
		if len(r.idx.LinkPaths(l.Target)) == 0 {
			r.add("dead-link", Error, d, fixFor(d)+": create the document, or change the link", "line %d links [[%s]], which resolves to nothing", l.Line, l.Target)
		}
	}
}

func (r *run) knowledge(d *doc.Doc, incoming map[string]int) {
	if d.Type() != "topic" && d.Type() != "source" {
		return
	}
	if incoming[d.Path] == 0 && !r.idx.Pending(d) && !r.sharesTag(d) {
		r.add("orphan", Warning, d, "wiki-edit: link it from a related topic, tag it, or remove it", "no other document links to it or holds its tags")
	}
	if d.Type() == "topic" && d.Str("kind") != "overview" && len(d.List("sources")) == 0 {
		r.add("uncited", Warning, d, "wiki-edit: cite the documents it rests on", "its sources are empty")
	}
	if d.Type() == "topic" {
		refreshed, ok := schema.ParseTime(cmp.Or(d.Str("refreshed"), d.Str("updated")))
		if !ok {
			return
		}
		for _, s := range d.List("sources") {
			c := r.idx.Linked(s)
			if c == nil {
				continue
			}
			if t, ok := schema.ParseTime(cmp.Or(c.Str("refreshed"), c.Str("updated"))); ok && t.After(refreshed) {
				r.add("stale", Warning, d, "wiki-review, then wiki-edit (a modify, or a confirm)", "it cites %s, which changed after it was refreshed", vault.Title(c))
				break
			}
		}
	}
}

// sharesTag reports whether another document holds one of a document's tags.
func (r *run) sharesTag(d *doc.Doc) bool {
	counts := r.idx.TagCounts()
	for _, t := range vault.DocTags(d) {
		if counts[strings.ToLower(t)] > 1 {
			return true
		}
	}
	return false
}

// nearTags reports two tags that are likely one: names that differ only by a plural or
// a separator, or two tags that hold the same documents.
func (r *run) nearTags() {
	counts := r.idx.TagCounts()
	byKey := map[string][]string{}
	members := map[string][]string{}
	for _, d := range r.idx.Documents() {
		for _, t := range tags.Expand(vault.DocTags(d)) {
			members[t] = append(members[t], d.ID())
		}
	}
	for t := range counts {
		k := strings.NewReplacer("-", "", "/", "").Replace(t)
		k = strings.TrimSuffix(k, "s")
		byKey[k] = append(byKey[k], t)
	}
	reported := map[string]bool{}
	anchor := r.idx.ByPath(r.idx.V.Doc.Path)
	if anchor == nil {
		return
	}
	for _, list := range byKey {
		if len(list) < 2 {
			continue
		}
		sort.Strings(list)
		key := strings.Join(list, ",")
		if !reported[key] {
			reported[key] = true
			r.add("tag-near", Info, anchor, "wiki-edit: a retag merges them", "the tags %s differ only by a plural or a separator", strings.Join(list, ", "))
		}
	}
	var names []string
	for t := range members {
		names = append(names, t)
	}
	sort.Strings(names)
	for i, a := range names {
		for _, b := range names[i+1:] {
			if len(members[a]) < 3 || !slices.Equal(members[a], members[b]) || tags.Under(b, a) || tags.Under(a, b) {
				continue
			}
			r.add("tag-near", Info, anchor, "wiki-edit: a retag merges them, if they mean one thing", "the tags %s and %s hold the same %d documents", a, b, len(members[a]))
		}
	}
}

func (r *run) info(d *doc.Doc) {
	now := r.opts.Now
	switch d.Type() {
	case "repository":
		if n := d.Front.Int("behind"); n > BehindLimit && d.Str("described") != "" {
			r.add("repository-behind", Info, d, "repo-ingest", "its description is %d commits behind the head", n)
		}
	case "change":
		if d.Str("status") == "proposed" {
			if t, ok := schema.ParseTime(d.Str("proposed")); ok && now.Sub(t) > ProposedAge {
				r.add("change-stale", Info, d, "apply or reject it", "proposed %s and still waiting", d.Str("proposed"))
			}
		}
	}
	if r.idx.Pending(d) {
		if t, ok := schema.ParseTime(d.Str("created")); ok && now.Sub(t) > PendingAge {
			r.add("pending", Info, d, "wiki-sync", "the wiki has not absorbed it since %s", d.Str("created"))
		}
	}
}
