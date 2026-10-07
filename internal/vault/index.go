package vault

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/links"
	"github.com/nathanaday/almagest/internal/schema"
	"github.com/nathanaday/almagest/internal/tags"
)

// skipDirs are folders the index never enters.
var skipDirs = map[string]bool{".git": true, ".obsidian": true, ".trash": true, ".claude": true, "node_modules": true}

// skipFolders are the vault's own folders the index never enters: the views, which code
// derives, and the trash, which holds what the user deleted.
var skipFolders = []string{WikiView, Trash}

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
	// Docs are the typed documents in their place: the three types directly in
	// tool/source-core/documents, sessions under tool/sessions/, changes under changes/, and Almagest.md.
	Docs []*doc.Doc
	// Misplaced are typed documents anywhere else. Tools do not see them; lint reports
	// them, and sync moves one under wiki/ back into the documents.
	Misplaced []*doc.Doc
	// Notes are the other markdown files outside the scratchpad, the thread archive, and the
	// views: the user's.
	Notes []*doc.Doc
	// Files are every other file a link may name, including the notes of the scratchpad and
	// the thread archive.
	Files []string

	byID    map[string]*doc.Doc
	byTitle map[string][]string // title key → paths of every markdown file
	byAlias map[string][]*doc.Doc
	byPath  map[string]*doc.Doc
	fileKey map[string][]string // file name key → paths of non-markdown files and scratchpad notes

	tagOnce  sync.Once
	counts   map[string]int
	pages    map[string][]*doc.Doc
	children map[string][]string

	pendingOnce sync.Once
	absorbed    map[string]map[string]bool // document id → hashes applied changes absorbed
	absorbers   map[string][]absorber      // document id → the applied changes that absorbed it
}

type absorber struct {
	hash   string
	change *doc.Doc
}

// Load reads every document of the vault. It skips wiki-view/ and tool/trash/.
func Load(v *Vault) (*Index, error) {
	idx := &Index{V: v, byID: map[string]*doc.Doc{}, byTitle: map[string][]string{}, byAlias: map[string][]*doc.Doc{}, byPath: map[string]*doc.Doc{}, fileKey: map[string][]string{}}
	err := filepath.WalkDir(v.Root, func(abs string, e fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		r, _ := filepath.Rel(v.Root, abs)
		rel := filepath.ToSlash(r)
		if e.IsDir() {
			if abs != v.Root && (skipDirs[e.Name()] || strings.HasPrefix(e.Name(), ".") || slices.Contains(skipFolders, rel)) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(e.Name(), ".") {
			return nil
		}
		md := strings.HasSuffix(strings.ToLower(e.Name()), ".md")
		if !md || Unread(rel) {
			idx.Files = append(idx.Files, rel)
			if md {
				key := links.BaseKey(rel)
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

// Unread reports whether a path lies in a folder whose markdown files the index keeps as
// link targets only: the scratchpad, the journals, the checkouts, and the returned ones.
func Unread(rel string) bool {
	for _, dir := range []string{Scratchpad, Journals, Checkout, Returned} {
		if rel == dir || strings.HasPrefix(rel, dir+"/") {
			return true
		}
	}
	return false
}

// InPlace reports whether a typed document lies where its type lives.
func InPlace(d *doc.Doc) bool {
	switch t := d.Type(); {
	case t == "vault":
		return IsMarker(d.Path)
	case t == "session":
		return strings.HasPrefix(d.Path, Sessions+"/")
	case t == "change":
		return strings.HasPrefix(d.Path, Changes+"/")
	case schema.IsDocument(t):
		return path.Dir(d.Path) == Documents
	}
	return false
}

func (idx *Index) add(d *doc.Doc) {
	idx.byPath[d.Path] = d
	key := links.BaseKey(d.Path)
	idx.byTitle[key] = append(idx.byTitle[key], d.Path)
	if d.Front == nil || !schema.Is(d.Type()) {
		idx.Notes = append(idx.Notes, d)
		return
	}
	if !InPlace(d) {
		idx.Misplaced = append(idx.Misplaced, d)
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

// ByID is the typed document with that id, or nil.
func (idx *Index) ByID(id string) *doc.Doc { return idx.byID[id] }

// ByPath is the parsed markdown file at a vault-relative path, or nil.
func (idx *Index) ByPath(rel string) *doc.Doc { return idx.byPath[rel] }

// Title is a document's title: its file name, or the vault's name for Almagest.md.
func Title(d *doc.Doc) string {
	if IsMarker(d.Path) {
		if n := d.Str("name"); n != "" {
			return n
		}
	}
	return d.Title()
}

// Of lists the typed documents of the given types, in path order.
func (idx *Index) Of(types ...string) []*doc.Doc {
	var out []*doc.Doc
	for _, d := range idx.Docs {
		if slices.Contains(types, d.Type()) {
			out = append(out, d)
		}
	}
	return out
}

// Documents lists the documents of the documents.
func (idx *Index) Documents() []*doc.Doc { return idx.Of(schema.DocumentTypes...) }

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
		if d := idx.byPath[p]; d != nil && idx.byID[d.ID()] == d && !seen[p] {
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
	if slices.Contains(types, d.Type()) {
		return d, nil
	}
	return nil, fmt.Errorf("%s is a %s, not a %s", Title(d), d.Type(), strings.Join(types, " or "))
}

// Backlinks lists the files that link to rel: every typed document and note, and the
// markdown kept as link targets only (the scratchpad, the thread archive, the journals,
// the checkouts), whose links Obsidian follows as well. The records do not count: a
// change or a session names what it touched, and keeps the name after a delete.
func (idx *Index) Backlinks(rel string) []string {
	var out []string
	check := func(d *doc.Doc) {
		if d.Path == rel || d.Type() == "change" || d.Type() == "session" {
			return
		}
		var targets []string
		if d.Front != nil {
			for _, k := range d.Front.Keys() {
				for _, v := range d.Front.List(k) {
					if doc.IsLink(v) {
						targets = append(targets, doc.LinkTarget(v))
					}
				}
			}
		}
		for _, l := range links.Find(Readable(d)) {
			targets = append(targets, l.Target)
		}
		for _, t := range targets {
			if slices.Contains(idx.LinkPaths(t), rel) {
				out = append(out, d.Path)
				return
			}
		}
	}
	for _, d := range append(append(append([]*doc.Doc{}, idx.Docs...), idx.Notes...), idx.Misplaced...) {
		check(d)
	}
	for _, f := range idx.Files {
		if strings.HasSuffix(strings.ToLower(f), ".md") && Unread(f) {
			if data, err := idx.V.Read(f); err == nil {
				check(doc.Parse(f, data))
			}
		}
	}
	sort.Strings(out)
	return out
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

// TypeOfLink is the type of the typed document a link names; "file" for a file with no
// type, "" when it names none.
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
	if d := idx.byPath[paths[0]]; d != nil && idx.byID[d.ID()] == d {
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
	if d := idx.byPath[paths[0]]; d != nil && idx.byID[d.ID()] == d {
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

// DocTags are a document's own tags and the tag it defines, as written.
func DocTags(d *doc.Doc) []string {
	out := slices.Clone(d.List("tags"))
	if def := d.Str("defines"); def != "" {
		out = append(out, def)
	}
	for i, t := range out {
		out[i] = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(t), "#"))
	}
	return out
}

// Holds reports whether a document holds every tag of want: it lists each tag, or a tag
// below it, in its tags or its defines.
func Holds(d *doc.Doc, want ...string) bool { return tags.HoldsAll(DocTags(d), want) }

func (idx *Index) readTags() {
	idx.counts = map[string]int{}
	idx.pages = map[string][]*doc.Doc{}
	idx.children = map[string][]string{}
	for _, d := range idx.Documents() {
		for _, t := range tags.Expand(DocTags(d)) {
			idx.counts[t]++
		}
		if def := d.Str("defines"); def != "" && (d.Type() == "topic" || d.Type() == "repository") {
			k := strings.ToLower(def)
			idx.pages[k] = append(idx.pages[k], d)
		}
	}
	for t := range idx.counts {
		if p := tags.Parent(t); p != "" {
			idx.children[p] = append(idx.children[p], t)
		}
	}
	for _, c := range idx.children {
		sort.Strings(c)
	}
}

// TagCounts maps every tag that some document holds, and every tag above one, to the
// count of documents that hold it.
func (idx *Index) TagCounts() map[string]int {
	idx.tagOnce.Do(idx.readTags)
	return idx.counts
}

// TagExists reports whether some document holds the tag, or a tag below it.
func (idx *Index) TagExists(t string) bool {
	return idx.TagCounts()[strings.ToLower(t)] > 0
}

// TagPage is the one document that defines a tag, or nil.
func (idx *Index) TagPage(t string) *doc.Doc {
	idx.tagOnce.Do(idx.readTags)
	if p := idx.pages[strings.ToLower(t)]; len(p) > 0 {
		return p[0]
	}
	return nil
}

// TagPages maps each defined tag to the documents that define it; lint reports two.
func (idx *Index) TagPages() map[string][]*doc.Doc {
	idx.tagOnce.Do(idx.readTags)
	return idx.pages
}

// TagChildren are the tags directly below t, sorted.
func (idx *Index) TagChildren(t string) []string {
	idx.tagOnce.Do(idx.readTags)
	return idx.children[t]
}

// TopTags are the tags with no parent, the most used first.
func (idx *Index) TopTags() []string {
	var out []string
	for t := range idx.TagCounts() {
		if !strings.Contains(t, "/") {
			out = append(out, t)
		}
	}
	idx.SortByCount(out)
	return out
}

// SortByCount orders tags by their counts, the most used first, then by name.
func (idx *Index) SortByCount(list []string) {
	c := idx.TagCounts()
	sort.SliceStable(list, func(i, j int) bool {
		if c[list[i]] != c[list[j]] {
			return c[list[i]] > c[list[j]]
		}
		return list[i] < list[j]
	})
}

// Ref is how a tool names one document.
type Ref struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Kind        string         `json:"kind,omitempty"`
	Title       string         `json:"title"`
	Path        string         `json:"path"`
	Tags        []string       `json:"tags"`
	Description string         `json:"description,omitempty"`
	Status      string         `json:"status,omitempty"`
	State       map[string]any `json:"state,omitempty"`
}

// Ref builds the reference to a document.
func (idx *Index) Ref(d *doc.Doc) Ref {
	r := Ref{ID: d.ID(), Type: d.Type(), Kind: d.Str("kind"), Title: Title(d), Path: d.Path, Tags: doc.NonNil(d.List("tags")), Description: d.Str("description"), Status: d.Str("status")}
	state := map[string]any{}
	switch d.Type() {
	case "source":
		r.Status = "absorbed"
		if idx.Pending(d) {
			r.Status = "pending"
		}
		state["media"] = d.Str("media")
		state["authority"] = d.Str("authority")
	case "repository":
		state["path"] = d.Str("path")
		state["defines"] = d.Str("defines")
		state["behind"] = d.Front.Int("behind")
		if d.Front.Bool("unlinked") {
			state["unlinked"] = true
		}
	case "topic":
		if def := d.Str("defines"); def != "" {
			state["defines"] = def
		}
		if s := d.Str("strength"); s != "" {
			state["strength"] = s
		}
		state["sources"] = len(d.List("sources"))
	case "change":
		state["counts"] = d.Str("counts")
		if notes, ok := doc.Section(d.Body, "Notes"); ok {
			r.Description = doc.FirstLine(notes)
		}
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

// Hash is a source's content hash for pending: the hash of its file.
func Hash(d *doc.Doc) string { return d.Str("sha256") }

// Pending reports whether the wiki has not absorbed a document's current content.
func (idx *Index) Pending(d *doc.Doc) bool {
	if d.Type() != "source" {
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
		if !schema.IDPattern.MatchString(id) {
			continue
		}
		out = append(out, Absorbed{Title: doc.LinkTarget(strings.TrimSpace(cells[0])), ID: id, Hash: hash})
	}
	return out
}

func (idx *Index) readAbsorbed() {
	idx.absorbed = map[string]map[string]bool{}
	idx.absorbers = map[string][]absorber{}
	for _, c := range idx.Of("change") {
		if c.Str("status") != "applied" {
			continue
		}
		for _, a := range ParseAbsorbed(c.Body) {
			if idx.absorbed[a.ID] == nil {
				idx.absorbed[a.ID] = map[string]bool{}
			}
			idx.absorbed[a.ID][a.Hash] = true
			idx.absorbers[a.ID] = append(idx.absorbers[a.ID], absorber{hash: a.Hash, change: c})
		}
	}
}

// AbsorbedBy is the last applied change that absorbed a document's current content, or
// nil.
func (idx *Index) AbsorbedBy(d *doc.Doc) *doc.Doc {
	idx.pendingOnce.Do(idx.readAbsorbed)
	h := Hash(d)
	var best *doc.Doc
	for _, a := range idx.absorbers[d.ID()] {
		if doc.SameHash(a.hash, h) && (best == nil || a.change.Str("applied") > best.Str("applied")) {
			best = a.change
		}
	}
	return best
}
