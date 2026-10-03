// Package migrate moves a vault of an earlier layout to the layout of 8.0 in one commit,
// in two steps. The first takes a 6.x vault to the flat layout of 7.0: every typed
// document into wiki/documents, the captured originals into wiki/assets, the scope tree
// into tags, the threads into stubs, plans, and events. It reads the 6.x fields (scope,
// parent, thread), which 6.4 and later kept current from the folders, and which earlier
// releases held as the record. The second (v8.go) takes the plans of 7.x to threads and
// chords. A 7.x vault takes the second step only. Plan computes the move and writes
// nothing; Run writes it.
package migrate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/derive"
	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/links"
	"github.com/nathanaday/atlas-obsidian/internal/lint"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/tags"
	"github.com/nathanaday/atlas-obsidian/internal/thread"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
	"github.com/nathanaday/atlas-obsidian/internal/views"
)

// Trailer marks the migration's commit.
const Trailer = "Atlas-Migrate: 8.0"

// TagMap is the tag a scope became.
type TagMap struct {
	Scope string `json:"scope"`
	Tag   string `json:"tag"`
}

// Retitle is a title the migration changes; links follow.
type Retitle struct {
	Old string `json:"old"`
	New string `json:"new"`
}

// Report is what a migration does.
type Report struct {
	Vault string `json:"vault"`
	// From is the layout the vault had: 6.x, or 7.x.
	From string `json:"from"`
	// Threads, Specs, TaskLists, Verifications, Chords, and Topics count what the plans
	// and designs of 7.x became.
	Threads       int `json:"threads"`
	Specs         int `json:"specs"`
	TaskLists     int `json:"task_lists"`
	Verifications int `json:"verifications"`
	Chords        int `json:"chords"`
	Topics        int `json:"topics"`
	Notes         int `json:"notes"`

	Documents  int       `json:"documents"`
	Events     int       `json:"events"`
	Assets     int       `json:"assets"`
	Tags       []TagMap  `json:"tags"`
	Retitles   []Retitle `json:"retitles"`
	Inbox      []string  `json:"inbox"`
	Scratchpad []string  `json:"scratchpad"`
	Removed    []string  `json:"removed"`
	Warnings   []string  `json:"warnings"`
	Commit     string    `json:"commit,omitempty"`
	Plugin     string    `json:"plugin,omitempty"`
	Problems   int       `json:"problems"`
}

// write is one file the migration writes; from, when set, is the file it replaces.
type write struct {
	from    string
	to      string
	content string
}

// move is one file the migration moves as it is.
type move struct{ from, to string }

// plan is the whole migration, in memory.
type plan struct {
	v       *vault.Vault
	now     time.Time
	report  *Report
	writes  []*write
	moves   []move
	removes []string
	titles  *titles
	rename  links.Rename
	// absorbed are the documents whose 6.x version an applied change absorbed: the path of
	// the new document it became.
	absorbed []string
}

// titles holds every title the new vault takes.
type titles struct{ taken map[string]string }

func (t *titles) free(title string) bool  { _, ok := t.taken[strings.ToLower(title)]; return !ok }
func (t *titles) take(title, path string) { t.taken[strings.ToLower(title)] = path }

func newReport(v *vault.Vault) *Report {
	return &Report{Vault: v.Name(), Tags: []TagMap{}, Retitles: []Retitle{}, Inbox: []string{}, Scratchpad: []string{}, Removed: []string{}, Warnings: []string{}}
}

// Plan computes the migration of a vault and writes nothing. For a 6.x vault it lists the
// first step only: the second reads what the first wrote.
func Plan(v *vault.Vault, now time.Time) (*Report, error) {
	if fresh, err := vault.Open(v.Root); err == nil {
		v = fresh
	}
	switch layout := v.LayoutVersion(); {
	case layout >= vault.Layout:
		return nil, errors.New("this vault has the 8.0 layout already; there is nothing to migrate")
	case layout >= vault.LayoutFlat:
		p, err := build8(v, now, newReport(v))
		if err != nil {
			return nil, err
		}
		p.report.From = "7.x"
		return p.report, nil
	}
	p, err := build(v, now)
	if err != nil {
		return nil, err
	}
	p.report.From = "6.x"
	p.report.Warnings = append(p.report.Warnings, "this lists the first step, the move to the flat layout; the same run then turns each plan into a thread (a stub, a spec, tasks, a verification)")
	return p.report, nil
}

// Run migrates a vault in one commit, then syncs the derived parts and writes the views.
func Run(v *vault.Vault, now time.Time) (*Report, error) {
	tx, err := vault.Begin(v, func() error { return vault.Recover(v) })
	if err != nil {
		return nil, err
	}
	defer tx.Close()
	fresh, err := vault.Open(v.Root)
	if err != nil {
		return nil, err
	}
	if fresh.LayoutVersion() >= vault.Layout {
		return nil, errors.New("this vault has the 8.0 layout already; there is nothing to migrate")
	}
	report := newReport(fresh)
	report.From = "7.x"
	if fresh.LayoutVersion() < vault.LayoutFlat {
		p, err := build(fresh, now)
		if err != nil {
			return nil, err
		}
		if err := p.execute(tx); err != nil {
			return nil, err
		}
		report = p.report
		report.From = "6.x"
		if fresh, err = vault.Open(v.Root); err != nil {
			return nil, err
		}
	}
	p, err := build8(fresh, now, report)
	if err != nil {
		return nil, err
	}
	if err := p.execute(tx); err != nil {
		return nil, err
	}
	if fresh, err = vault.Open(v.Root); err != nil {
		return nil, err
	}
	// The files the next steps write are kept too, so a failed commit puts them back.
	machine := []string{vault.AppJSON}
	for _, f := range vault.PluginFiles {
		machine = append(machine, path.Join(vault.PluginDir, f))
	}
	for _, rel := range vault.Bases {
		machine = append(machine, rel)
	}
	if err := tx.Keep(machine...); err != nil {
		return nil, err
	}
	if err := fresh.EnsureFolders(); err != nil {
		return nil, err
	}
	if _, err := vault.ObsidianSettings(fresh); err != nil {
		return nil, err
	}
	// An older plugin cannot read the new layout, so the vault gets the one this binary carries.
	if fresh.InstalledPluginVersion() != "" {
		wrote, err := vault.InstallPlugin(fresh)
		if err != nil {
			return nil, err
		}
		if len(wrote) > 0 {
			p.report.Plugin = vault.PluginVersion()
		}
	}
	if err := syncDerived(fresh, tx, now); err != nil {
		return nil, err
	}
	sha, err := tx.CommitAll("layout: migrate to 8.0\n\n" + Trailer + " from " + report.From)
	if err != nil {
		return nil, err
	}
	p.report.Commit = sha
	idx, err := vault.Load(fresh)
	if err != nil {
		return p.report, err
	}
	views.Write(idx, now)
	fresh.SyncSettings(nil)
	if f, err := lint.Run(idx, lint.Options{Quick: true, Now: now}); err == nil {
		p.report.Problems = f.Counts[lint.Error]
	}
	return p.report, nil
}

// syncDerived writes the statuses, the callouts, and the git facts the new layout
// derives, so the migration's commit holds them.
func syncDerived(v *vault.Vault, tx *vault.Tx, now time.Time) error {
	idx, err := vault.Load(v)
	if err != nil {
		return err
	}
	if _, err := thread.Load(idx).SyncWith(vault.NewGuard(idx, tx)); err != nil {
		return err
	}
	if idx, err = vault.Load(v); err != nil {
		return err
	}
	if _, err := derive.GitFacts(idx, vault.NewGuard(idx, tx).Write, now); err != nil {
		return err
	}
	if idx, err = vault.Load(v); err != nil {
		return err
	}
	_, err = derive.Sync(idx, vault.NewGuard(idx, tx).Write)
	return err
}

// old is the 6.x vault as read from disk.
type old struct {
	docs    []*doc.Doc          // every typed document
	byTitle map[string]*doc.Doc // lower-case title → document
	byID    map[string]*doc.Doc
	files   []string // every other file under wiki/ and threads/
	notes   []*doc.Doc
}

var skipDirs = map[string]bool{".git": true, ".obsidian": true, ".trash": true, ".claude": true, "node_modules": true}

// oldTypes are the types of the 6.x vault.
var oldTypes = []string{"vault", "area", "repository", "concept", "entity", "policy", "source", "stub", "spec", "task", "receipt", "session", "change"}

func read(v *vault.Vault) (*old, error) {
	o := &old{byTitle: map[string]*doc.Doc{}, byID: map[string]*doc.Doc{}}
	err := filepath.WalkDir(v.Root, func(abs string, e fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel := v.Rel(abs)
		if e.IsDir() {
			if abs != v.Root && (skipDirs[e.Name()] || strings.HasPrefix(e.Name(), ".") || rel == vault.Views) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(e.Name(), ".") {
			return nil
		}
		inScoped := strings.HasPrefix(rel, "wiki/") || strings.HasPrefix(rel, "threads/")
		// 6.x kept every captured original here, a note too, and never indexed the folder.
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".md") || strings.HasPrefix(rel, "wiki/sources/files/") {
			if inScoped {
				o.files = append(o.files, rel)
			}
			return nil
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return nil
		}
		d := doc.Parse(rel, data)
		if d.Front != nil && slices.Contains(oldTypes, d.Type()) {
			o.docs = append(o.docs, d)
			o.byTitle[strings.ToLower(vault.Title(d))] = d
			if d.ID() != "" {
				o.byID[d.ID()] = d
			}
		} else if inScoped {
			o.notes = append(o.notes, d)
		}
		return nil
	})
	sort.Slice(o.docs, func(i, j int) bool { return o.docs[i].Path < o.docs[j].Path })
	return o, err
}

func (o *old) linked(value string) *doc.Doc {
	t := doc.LinkTarget(value)
	if t == "" {
		return nil
	}
	if d := o.byID[t]; d != nil {
		return d
	}
	return o.byTitle[strings.ToLower(t)]
}

func (o *old) of(types ...string) []*doc.Doc {
	var out []*doc.Doc
	for _, d := range o.docs {
		if slices.Contains(types, d.Type()) {
			out = append(out, d)
		}
	}
	return out
}

// build reads the 6.x vault and computes every write of the first step.
func build(v *vault.Vault, now time.Time) (*plan, error) {
	if fresh, err := vault.Open(v.Root); err == nil {
		v = fresh
	}
	if v.LayoutVersion() >= vault.LayoutFlat {
		return nil, errors.New("this vault has the flat layout already")
	}
	o, err := read(v)
	if err != nil {
		return nil, err
	}
	p := &plan{v: v, now: now.Truncate(time.Second), report: newReport(v), titles: &titles{taken: map[string]string{}}, rename: links.Rename{}}
	var waiting []string
	for _, c := range o.of("change") {
		if s := c.Str("status"); s == "proposed" || s == "applying" {
			waiting = append(waiting, vault.Title(c))
		}
	}
	if len(waiting) > 0 {
		return nil, fmt.Errorf("apply or reject these changes first, since their writes use the 6.x schemas: %s", strings.Join(waiting, ", "))
	}
	for _, s := range o.of("session") {
		if st := s.Str("status"); st == "running" || st == "waiting" {
			p.report.Warnings = append(p.report.Warnings, fmt.Sprintf("the session %s is %s; its threads and tasks become the work and specs of the new layout", s.Title(), st))
		}
	}
	for _, d := range o.docs {
		p.titles.take(vault.Title(d), d.Path)
	}
	for _, n := range o.notes {
		p.titles.take(n.Title(), n.Path)
	}
	scopeTag := p.tags(o)
	absorbed := absorbedIDs(o)
	p.knowledge(o, scopeTag)
	if err := p.threads(o, scopeTag, absorbed); err != nil {
		return nil, err
	}
	p.records(o)
	p.files(o)
	p.atlas()
	p.linkRewrites(o)
	p.absorbChange(o)
	return p, nil
}

// tags maps each area and repository to its tag: the names on its chain from the top.
func (p *plan) tags(o *old) map[string]string {
	out := map[string]string{}
	var tagOf func(d *doc.Doc, seen map[string]bool) string
	tagOf = func(d *doc.Doc, seen map[string]bool) string {
		if t, ok := out[d.ID()]; ok {
			return t
		}
		if seen[d.ID()] {
			return slug(vault.Title(d), d.ID())
		}
		seen[d.ID()] = true
		t := slug(vault.Title(d), d.ID())
		if parent := o.linked(d.Str("parent")); parent != nil && parent.Type() == "area" {
			t = tagOf(parent, seen) + "/" + t
		}
		out[d.ID()] = t
		return t
	}
	for _, d := range o.of("area", "repository") {
		tagOf(d, map[string]bool{})
	}
	// Two scopes of one name under one parent take their ids.
	seen := map[string]string{}
	for _, d := range o.of("area", "repository") {
		t := out[d.ID()]
		if other, ok := seen[t]; ok && other != d.ID() {
			out[d.ID()] = t + "-" + strings.TrimPrefix(d.ID(), d.ID()[:4])
		}
		seen[out[d.ID()]] = d.ID()
	}
	for _, d := range o.of("area", "repository") {
		p.report.Tags = append(p.report.Tags, TagMap{Scope: vault.Title(d), Tag: out[d.ID()]})
	}
	sort.Slice(p.report.Tags, func(i, j int) bool { return p.report.Tags[i].Tag < p.report.Tags[j].Tag })
	return out
}

var nonTag = regexp.MustCompile(`[^a-z0-9-]+`)

// slug makes a scope's name a tag part.
func slug(name, id string) string {
	if t, err := tags.Normalize(name); err == nil && !strings.Contains(t, "/") {
		return t
	}
	s := nonTag.ReplaceAllString(strings.ToLower(name), "-")
	s = strings.Trim(regexp.MustCompile(`-{2,}`).ReplaceAllString(s, "-"), "-")
	if !tags.Valid(s) {
		s = "scope-" + strings.TrimPrefix(id, id[:min(4, len(id))])
	}
	return s
}

// scopeTags are the tags of the scopes a list of links names.
func scopeTags(o *old, scopeTag map[string]string, values ...string) []string {
	var out []string
	for _, v := range values {
		if s := o.linked(v); s != nil {
			if t := scopeTag[s.ID()]; t != "" && !slices.Contains(out, t) {
				out = append(out, t)
			}
		}
	}
	return out
}

// normalized keeps the valid tags of a 6.x tags list.
func normalized(list []string) []string {
	var out []string
	for _, t := range list {
		if n, err := tags.Normalize(t); err == nil && !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out
}

func union(lists ...[]string) []string {
	out := []string{}
	for _, l := range lists {
		for _, x := range l {
			if x != "" && !slices.Contains(out, x) {
				out = append(out, x)
			}
		}
	}
	return out
}

// put records a document the migration writes at its new path.
func (p *plan) put(d *doc.Doc, title, content string) string {
	to := vault.DocPath(title)
	p.writes = append(p.writes, &write{from: d.Path, to: to, content: content})
	p.report.Documents++
	return to
}

// fields keeps the fields of d that the new schema of typ names or that are the user's,
// and drops the 6.x fields code owned.
var dropped = []string{"scope", "parent", "chain", "stage", "outcome", "active", "tasks", "thread", "thread_id", "superseded", "status", "kind", "blocked", "order", "repository", "depends", "areas"}

func (p *plan) keepUser(d *doc.Doc, typ string, skip ...string) []doc.Field {
	t := schema.Get(typ)
	var out []doc.Field
	for _, k := range d.Front.Keys() {
		if slices.Contains(skip, k) || slices.Contains(dropped, k) || (t != nil && t.Field(k) != nil) {
			continue
		}
		// Every 6.x document had sources; an empty one is not the user's.
		if k == "sources" && len(d.List(k)) == 0 {
			continue
		}
		out = append(out, doc.Field{Key: k, Value: d.Front.Map()[k]})
	}
	return out
}

func stamp(s string, fallback time.Time) string {
	if t, ok := vault.ParseTime(s); ok {
		return vault.Stamp(t)
	}
	return vault.Stamp(fallback)
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

// knowledge converts areas, repositories, concepts, entities, policies, and sources.
func (p *plan) knowledge(o *old, scopeTag map[string]string) {
	for _, d := range o.of("area", "repository", "concept", "entity", "policy", "source") {
		created := stamp(d.Str("created"), p.now)
		updated := stamp(d.Str("updated"), p.now)
		body := strings.TrimLeft(doc.StripLead(d.Body), "\n")
		base := []doc.Field{{Key: "id", Value: d.ID()}}
		var fields []doc.Field
		switch d.Type() {
		case "area":
			parentTags := scopeTags(o, scopeTag, d.Str("parent"))
			fields = append(base,
				doc.Field{Key: "type", Value: "topic"},
				doc.Field{Key: "kind", Value: "overview"},
				doc.Field{Key: "description", Value: orDefault(d.Str("description"), vault.Title(d))},
				doc.Field{Key: "tags", Value: union(parentTags)},
				doc.Field{Key: "aliases", Value: nonNil(d.List("aliases"))},
				doc.Field{Key: "created", Value: created},
				doc.Field{Key: "updated", Value: updated},
				doc.Field{Key: "refreshed", Value: updated},
				doc.Field{Key: "status", Value: "stable"},
				doc.Field{Key: "sources", Value: []string{}},
				doc.Field{Key: "defines", Value: scopeTag[d.ID()]},
			)
			summary := orDefault(d.Str("description"), "")
			body = "## Summary\n\n" + summary + "\n\n## Context\n\n" + strings.TrimSpace(body) + "\n"
		case "repository":
			fields = append(base,
				doc.Field{Key: "type", Value: "repository"},
				doc.Field{Key: "description", Value: orDefault(d.Str("description"), vault.Title(d))},
				doc.Field{Key: "tags", Value: union(scopeTags(o, scopeTag, d.Str("parent")))},
				doc.Field{Key: "aliases", Value: nonNil(d.List("aliases"))},
				doc.Field{Key: "created", Value: created},
				doc.Field{Key: "updated", Value: updated},
				doc.Field{Key: "refreshed", Value: updated},
				doc.Field{Key: "defines", Value: scopeTag[d.ID()]},
				doc.Field{Key: "path", Value: d.Str("path")},
				doc.Field{Key: "remote", Value: d.Str("remote")},
				doc.Field{Key: "branch", Value: d.Str("branch")},
				doc.Field{Key: "described", Value: d.Str("described")},
			)
		case "source":
			file := doc.LinkTarget(d.Str("file"))
			fields = append(base,
				doc.Field{Key: "type", Value: "source"},
				doc.Field{Key: "description", Value: orDefault(d.Str("description"), "Captured, not yet ingested.")},
				doc.Field{Key: "tags", Value: union(scopeTags(o, scopeTag, d.Str("scope")), normalized(d.List("tags")))},
				doc.Field{Key: "aliases", Value: nonNil(d.List("aliases"))},
				doc.Field{Key: "created", Value: created},
				doc.Field{Key: "updated", Value: updated},
				doc.Field{Key: "refreshed", Value: stamp(d.Str("captured"), p.now)},
				doc.Field{Key: "authority", Value: orDefault(d.Str("authority"), "unknown")},
				doc.Field{Key: "status", Value: "pending"},
				doc.Field{Key: "file", Value: doc.Link(file)},
				doc.Field{Key: "media", Value: mediaOf(file)},
				doc.Field{Key: "sha256", Value: d.Str("sha256")},
				doc.Field{Key: "origin", Value: orDefault(d.Str("origin"), "inbox")},
				doc.Field{Key: "locator", Value: d.Str("locator")},
				doc.Field{Key: "measure", Value: d.Str("measure")},
				doc.Field{Key: "captured", Value: stamp(d.Str("captured"), p.now)},
			)
			// The 6.x body opened with the embed; derive puts the original back.
			body = strings.TrimLeft(strings.TrimPrefix(body, "!"+doc.Link(file)), "\n")
		default:
			extra := normalized(d.List("tags"))
			if k := d.Str("kind"); d.Type() == "entity" && k != "" && k != "other" {
				extra = union(extra, normalized([]string{k}))
			}
			fields = append(base,
				doc.Field{Key: "type", Value: "topic"},
				doc.Field{Key: "kind", Value: d.Type()},
				doc.Field{Key: "description", Value: orDefault(d.Str("description"), vault.Title(d))},
				doc.Field{Key: "tags", Value: union(scopeTags(o, scopeTag, d.Str("scope")), extra)},
				doc.Field{Key: "aliases", Value: nonNil(d.List("aliases"))},
				doc.Field{Key: "created", Value: created},
				doc.Field{Key: "updated", Value: updated},
				doc.Field{Key: "refreshed", Value: updated},
				doc.Field{Key: "status", Value: orDefault(d.Str("status"), "stable")},
				doc.Field{Key: "sources", Value: nonNil(d.List("sources"))},
			)
			if s := d.Str("strength"); s != "" {
				fields = append(fields, doc.Field{Key: "strength", Value: s})
			}
		}
		fields = append(fields, p.keepUser(d, fieldType(fields))...)
		p.put(d, vault.Title(d), doc.Render(fields, body))
	}
}

func fieldType(fields []doc.Field) string {
	for _, f := range fields {
		if f.Key == "type" {
			return f.Value.(string)
		}
	}
	return ""
}

func mediaOf(file string) string {
	switch strings.ToLower(path.Ext(file)) {
	case ".pdf":
		return "pdf"
	case ".md", ".markdown":
		return "markdown"
	case ".txt", ".text", ".csv", ".tsv", ".json", ".yaml", ".yml", ".html", ".htm", ".xml", ".log":
		return "text"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp":
		return "image"
	case ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".odt", ".rtf":
		return "office"
	}
	return "other"
}

func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

// absorbedIDs maps each document id an applied change absorbed to the hashes it recorded.
func absorbedIDs(o *old) map[string][]string {
	out := map[string][]string{}
	for _, c := range o.of("change") {
		if c.Str("status") != "applied" {
			continue
		}
		for _, a := range vault.ParseAbsorbed(c.Body) {
			out[a.ID] = append(out[a.ID], a.Hash)
		}
	}
	return out
}

// wasAbsorbed reports whether an applied change absorbed a 6.x document as it is now.
func wasAbsorbed(absorbed map[string][]string, d *doc.Doc) bool {
	h := doc.ContentHash(d.Body)
	for _, have := range absorbed[d.ID()] {
		if doc.SameHash(have, h) {
			return true
		}
	}
	return false
}
