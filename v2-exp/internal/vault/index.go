package vault

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/doc"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/links"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/schema"
)

// skipDirs are folders the index never enters.
var skipDirs = map[string]bool{".git": true, ".obsidian": true, ".trash": true, ".claude": true, "node_modules": true}

type cacheEntry struct {
	mod  time.Time
	size int64
	d    *doc.Doc
}

var (
	cacheMu sync.Mutex
	cache   = map[string]cacheEntry{}
)

// cachedDoc parses a vault file, or returns the copy parsed before when the file has not
// changed since.
func cachedDoc(v *Vault, rel string) (*doc.Doc, error) {
	abs := v.Abs(rel)
	st, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	cacheMu.Lock()
	e, ok := cache[abs]
	cacheMu.Unlock()
	if ok && e.mod.Equal(st.ModTime()) && e.size == st.Size() {
		return e.d, nil
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	d := doc.Parse(rel, data)
	cacheMu.Lock()
	cache[abs] = cacheEntry{mod: st.ModTime(), size: st.Size(), d: d}
	cacheMu.Unlock()
	return d, nil
}

// Index is every document of the vault, read at one moment.
type Index struct {
	V *Vault
	// Docs are the typed documents: those whose type is a document type.
	Docs []*doc.Doc
	// Notes are the other markdown files outside the scratchpad: the user's notes.
	Notes []*doc.Doc
	// Files are every other file a link may name, including the scratchpad's notes.
	Files []string

	byID    map[string]*doc.Doc
	byTitle map[string][]string // title key → paths of every markdown file
	byAlias map[string][]*doc.Doc
	byPath  map[string]*doc.Doc
	fileKey map[string][]string // file name key → paths of non-markdown files and scratchpad notes

	pendingOnce sync.Once
	absorbed    map[string]map[string]bool // document id → hashes applied changes absorbed
}

// Load reads every document of the vault.
func Load(v *Vault) (*Index, error) {
	idx := &Index{V: v, byID: map[string]*doc.Doc{}, byTitle: map[string][]string{}, byAlias: map[string][]*doc.Doc{}, byPath: map[string]*doc.Doc{}, fileKey: map[string][]string{}}
	err := filepath.WalkDir(v.Root, func(abs string, e fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		r, _ := filepath.Rel(v.Root, abs)
		rel := filepath.ToSlash(r)
		if e.IsDir() {
			if abs != v.Root && (skipDirs[e.Name()] || strings.HasPrefix(e.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(e.Name(), ".") {
			return nil
		}
		md := strings.HasSuffix(strings.ToLower(e.Name()), ".md")
		if !md || rel == Scratchpad || strings.HasPrefix(rel, Scratchpad+"/") {
			idx.Files = append(idx.Files, rel)
			key := links.Key(e.Name())
			if md {
				key = links.BaseKey(rel)
				idx.byTitle[key] = append(idx.byTitle[key], rel)
			}
			idx.fileKey[strings.ToLower(e.Name())] = append(idx.fileKey[strings.ToLower(e.Name())], rel)
			return nil
		}
		d, err := cachedDoc(v, rel)
		if err != nil {
			return nil
		}
		idx.add(d)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(idx.Docs, func(i, j int) bool { return idx.Docs[i].Path < idx.Docs[j].Path })
	return idx, nil
}

func (idx *Index) add(d *doc.Doc) {
	idx.byPath[d.Path] = d
	key := links.BaseKey(d.Path)
	idx.byTitle[key] = append(idx.byTitle[key], d.Path)
	if d.Front == nil || !schema.Is(d.Type()) {
		idx.Notes = append(idx.Notes, d)
		return
	}
	idx.Docs = append(idx.Docs, d)
	if id := d.ID(); id != "" {
		idx.byID[id] = d
	}
	for _, a := range d.List("aliases") {
		k := links.Key(a)
		idx.byAlias[k] = append(idx.byAlias[k], d)
	}
}

// Reload reads the vault again after a write.
func (idx *Index) Reload() (*Index, error) { return Load(idx.V) }

// ByID is the typed document with that id, or nil.
func (idx *Index) ByID(id string) *doc.Doc { return idx.byID[id] }

// ByPath is the parsed markdown file at a vault-relative path, or nil.
func (idx *Index) ByPath(rel string) *doc.Doc { return idx.byPath[rel] }

// Title is a document's title: its file name, or the vault's name for Atlas.md.
func Title(d *doc.Doc) string {
	if d.Path == Marker {
		if n := d.Str("name"); n != "" {
			return n
		}
	}
	return d.Title()
}

// Of lists the typed documents of the given types, in path order.
func (idx *Index) Of(types ...string) []*doc.Doc {
	want := map[string]bool{}
	for _, t := range types {
		want[t] = true
	}
	var out []*doc.Doc
	for _, d := range idx.Docs {
		if want[d.Type()] {
			out = append(out, d)
		}
	}
	return out
}

// Resolve finds the one typed document a key names: an id, a title, a [[link]], or an
// alias. None or two is an error that lists what it found.
func (idx *Index) Resolve(key string) (*doc.Doc, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("no document named")
	}
	if d := idx.byID[key]; d != nil {
		return d, nil
	}
	target := doc.LinkTarget(key)
	var found []*doc.Doc
	seen := map[string]bool{}
	for _, p := range idx.byTitle[links.BaseKey(target)] {
		if d := idx.byPath[p]; d != nil && d.Front != nil && schema.Is(d.Type()) && !seen[p] {
			if strings.Contains(target, "/") && !strings.HasSuffix(links.Key(p), links.Key(target)) {
				continue
			}
			found = append(found, d)
			seen[p] = true
		}
	}
	if len(found) == 0 {
		for _, d := range idx.byAlias[links.Key(target)] {
			if !seen[d.Path] {
				found = append(found, d)
				seen[d.Path] = true
			}
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return nil, fmt.Errorf("no document is named %q; search finds one by its words", key)
	}
	var names []string
	for _, d := range found {
		names = append(names, fmt.Sprintf("%s (%s, %s)", Title(d), d.ID(), d.Path))
	}
	return nil, fmt.Errorf("%q names %d documents: %s; give the id", key, len(found), strings.Join(names, "; "))
}

// ResolveType is Resolve, refusing a document of another type.
func (idx *Index) ResolveType(key string, types ...string) (*doc.Doc, error) {
	d, err := idx.Resolve(key)
	if err != nil {
		return nil, err
	}
	for _, t := range types {
		if d.Type() == t {
			return d, nil
		}
	}
	return nil, fmt.Errorf("%s is a %s, not a %s", Title(d), d.Type(), strings.Join(types, " or "))
}

// LinkPaths lists the files a link target names, the way Obsidian resolves it: a path
// from the vault's root, else a markdown file by its title, else any file by its name.
func (idx *Index) LinkPaths(target string) []string {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}
	if strings.Contains(target, "/") {
		want := links.Key(strings.TrimPrefix(target, "/"))
		var out []string
		for _, p := range idx.byTitle[links.BaseKey(target)] {
			if links.Key(p) == want {
				out = append(out, p)
			}
		}
		for _, p := range idx.fileKey[strings.ToLower(path.Base(target))] {
			if strings.EqualFold(p, strings.TrimPrefix(target, "/")) {
				out = append(out, p)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	if paths := idx.byTitle[links.BaseKey(target)]; len(paths) > 0 {
		return dedupe(paths)
	}
	return idx.fileKey[strings.ToLower(path.Base(target))]
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range in {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// TypeOfLink is the type of the typed document a link names; "" when it names none or a
// file with no type.
func (idx *Index) TypeOfLink(target string) (string, error) {
	t := doc.LinkTarget(target)
	if d := idx.byID[t]; d != nil {
		return d.Type(), nil
	}
	paths := idx.LinkPaths(t)
	if len(paths) > 1 {
		return "", fmt.Errorf("%s names %d files: %s", target, len(paths), strings.Join(paths, ", "))
	}
	if len(paths) == 0 {
		return "", nil
	}
	if d := idx.byPath[paths[0]]; d != nil && d.Front != nil && schema.Is(d.Type()) {
		return d.Type(), nil
	}
	return "file", nil
}

// Linked is the typed document a frontmatter link names, or nil.
func (idx *Index) Linked(value string) *doc.Doc {
	t := doc.LinkTarget(value)
	if t == "" {
		return nil
	}
	if d := idx.byID[t]; d != nil {
		return d
	}
	paths := idx.LinkPaths(t)
	if len(paths) != 1 {
		return nil
	}
	if d := idx.byPath[paths[0]]; d != nil && d.Front != nil && schema.Is(d.Type()) {
		return d
	}
	return nil
}

// TitleHolders lists the markdown files whose title, or whose typed alias, is title,
// without case.
func (idx *Index) TitleHolders(title string) []string {
	k := links.Key(title)
	out := append([]string{}, idx.byTitle[k]...)
	for _, d := range idx.byAlias[k] {
		out = append(out, d.Path)
	}
	return dedupe(out)
}

// Ref is how a tool names one document: its id, type, title, path, scope, one line, and
// the state its type derives.
type Ref struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	Path        string         `json:"path"`
	Scope       []string       `json:"scope"`
	Description string         `json:"description,omitempty"`
	State       map[string]any `json:"state,omitempty"`
}

// Ref builds the reference to a document.
func (idx *Index) Ref(d *doc.Doc) Ref {
	r := Ref{ID: d.ID(), Type: d.Type(), Title: Title(d), Path: d.Path, Scope: idx.ScopeIDs(d), Description: d.Str("description")}
	state := map[string]any{}
	switch d.Type() {
	case "stub":
		state["stage"] = d.Str("stage")
		state["outcome"] = d.Str("outcome")
		state["active"] = d.Front.Bool("active")
		state["tasks"] = d.Str("tasks")
		state["priority"] = orDefault(d.Str("priority"), "normal")
		state["blocked"] = d.Str("blocked")
	case "task":
		state["status"] = d.Str("status")
		state["active"] = d.Front.Bool("active")
		state["order"] = d.Front.Int("order")
		state["repository"] = doc.LinkTarget(d.Str("repository"))
	case "session":
		state["status"] = d.Str("status")
		state["threads"] = targets(d.List("threads"))
		state["tasks"] = targets(d.List("tasks"))
	case "change":
		state["status"] = d.Str("status")
		state["counts"] = d.Str("counts")
		if notes, ok := doc.Section(d.Body, "Notes"); ok {
			r.Description = doc.FirstLine(notes)
		}
	case "source":
		state["pending"] = idx.Pending(d)
	case "repository":
		state["path"] = d.Str("path")
	default:
		if s := d.Str("status"); s != "" {
			state["status"] = s
		}
	}
	if idx.Wikified(d.Type()) && d.Type() != "source" {
		state["pending"] = idx.Pending(d)
	}
	if len(state) > 0 {
		r.State = state
	}
	return r
}

// Refs builds the references to documents.
func (idx *Index) Refs(ds []*doc.Doc) []Ref {
	out := make([]Ref, 0, len(ds))
	for _, d := range ds {
		out = append(out, idx.Ref(d))
	}
	return out
}

func targets(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, doc.LinkTarget(v))
	}
	return out
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// ScopeIDs are the ids of the scope pages a document points at: a knowledge page's
// scope, a stub's scopes, a scope page's parent, a task's repository.
func (idx *Index) ScopeIDs(d *doc.Doc) []string {
	var values []string
	switch d.Type() {
	case "area", "repository":
		values = []string{d.Str("parent")}
	case "stub":
		values = d.List("scope")
	case "task":
		values = []string{d.Str("repository")}
	case "concept", "entity", "policy", "source":
		values = []string{d.Str("scope")}
	case "spec", "receipt":
		if stub := idx.Linked(d.Str("thread")); stub != nil {
			values = stub.List("scope")
		}
	}
	out := []string{}
	for _, v := range values {
		if s := idx.Linked(v); s != nil && (s.Type() == "area" || s.Type() == "repository") {
			out = append(out, s.ID())
		}
	}
	return out
}

// Parent is a scope page's parent area, or nil for the vault.
func (idx *Index) Parent(d *doc.Doc) *doc.Doc {
	p := idx.Linked(d.Str("parent"))
	if p != nil && p.Type() == "area" {
		return p
	}
	return nil
}

// Ancestors are the areas above a scope page, nearest first, stopping at a loop.
func (idx *Index) Ancestors(d *doc.Doc) []*doc.Doc {
	var out []*doc.Doc
	seen := map[string]bool{d.ID(): true}
	for p := idx.Parent(d); p != nil && !seen[p.ID()]; p = idx.Parent(p) {
		seen[p.ID()] = true
		out = append(out, p)
	}
	return out
}

// Under reports whether a scope page is scope, or lies below it. The empty scope is the
// vault, which holds everything.
func (idx *Index) Under(d *doc.Doc, scope string) bool {
	if scope == "" || d.ID() == scope {
		return true
	}
	for _, a := range idx.Ancestors(d) {
		if a.ID() == scope {
			return true
		}
	}
	return false
}

// InScope reports whether a document belongs to scope or a scope below it. A document
// with no scope belongs only to the vault.
func (idx *Index) InScope(d *doc.Doc, scope string) bool {
	if scope == "" {
		return true
	}
	for _, id := range idx.ScopeIDs(d) {
		if s := idx.byID[id]; s != nil && idx.Under(s, scope) {
			return true
		}
	}
	return false
}

// Wikified reports whether documents of a type are pending until a change absorbs them.
func (idx *Index) Wikified(t string) bool {
	for _, w := range idx.V.Wikify() {
		if w == t {
			return true
		}
	}
	return false
}

// Hash is a document's content hash for pending: a source's file hash, else the hash of
// its body below the lead callout.
func Hash(d *doc.Doc) string {
	if d.Type() == "source" {
		return d.Str("sha256")
	}
	return doc.ContentHash(d.Body)
}

// Pending reports whether the wiki has not absorbed a document's current content.
func (idx *Index) Pending(d *doc.Doc) bool {
	if !idx.Wikified(d.Type()) {
		return false
	}
	if d.Type() == "task" && d.Str("status") != "done" {
		return false
	}
	if d.Type() == "receipt" && d.Front.Bool("superseded") {
		return false
	}
	idx.pendingOnce.Do(idx.readAbsorbed)
	h := Hash(d)
	if h == "" {
		return true
	}
	for have := range idx.absorbed[d.ID()] {
		if doc.SameHash(have, h) {
			return false
		}
	}
	return true
}

// PendingDocs lists every pending document, oldest first.
func (idx *Index) PendingDocs() []*doc.Doc {
	var out []*doc.Doc
	for _, d := range idx.Docs {
		if idx.Pending(d) {
			out = append(out, d)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Str("created") < out[j].Str("created") })
	return out
}

// Absorbed is one row of a change's Absorbed table.
type Absorbed struct {
	Title string
	ID    string
	Hash  string
}

// ParseAbsorbed reads the rows of a change document's Absorbed table.
func ParseAbsorbed(body string) []Absorbed {
	text, ok := doc.Section(body, "Absorbed")
	if !ok {
		return nil
	}
	var out []Absorbed
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) < 3 {
			continue
		}
		id, hash := strings.TrimSpace(cells[1]), strings.TrimSpace(cells[2])
		if !doc.IDPattern.MatchString(id) {
			continue
		}
		out = append(out, Absorbed{Title: doc.LinkTarget(strings.TrimSpace(cells[0])), ID: id, Hash: hash})
	}
	return out
}

func (idx *Index) readAbsorbed() {
	idx.absorbed = map[string]map[string]bool{}
	for _, c := range idx.Of("change") {
		if c.Str("status") != "applied" {
			continue
		}
		for _, a := range ParseAbsorbed(c.Body) {
			if idx.absorbed[a.ID] == nil {
				idx.absorbed[a.ID] = map[string]bool{}
			}
			idx.absorbed[a.ID][a.Hash] = true
		}
	}
}
