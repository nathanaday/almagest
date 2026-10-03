package change

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/derive"
	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/links"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/thread"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Trailer names the change a commit applied.
const Trailer = vault.ChangeTrailer

// Preview is what a change does, enough to show it in the chat.
type Preview struct {
	Ref    vault.Ref   `json:"ref"`
	Status string      `json:"status"`
	Counts Counts      `json:"counts"`
	Writes []WriteLine `json:"writes"`
	// Rewrites are the files outside the writes whose links or tags the change rewrites.
	Rewrites []vault.Ref `json:"rewrites"`
	NewTags  []string    `json:"new_tags"`
	Absorbs  []vault.Ref `json:"absorbs"`
	// Events are the events apply wrote: the promoted events of its promotes.
	Events   []vault.Ref `json:"events,omitempty"`
	Warnings []string    `json:"warnings"`
	Commit   string      `json:"commit,omitempty"`
	Reason   string      `json:"reason,omitempty"`
	// MovedFromViews are notes of the user's that the views step after the write moved out of
	// views/ into inbox/.
	MovedFromViews []vault.Moved `json:"moved_from_views,omitempty"`
}

// WriteLine is one write of a preview.
type WriteLine struct {
	Op    string   `json:"op"`
	Title string   `json:"title"`
	Type  string   `json:"type,omitempty"`
	Kind  string   `json:"kind,omitempty"`
	Tags  []string `json:"tags,omitempty"`
	Path  string   `json:"path,omitempty"`
	Lines string   `json:"lines,omitempty"`
	Note  string   `json:"note,omitempty"`
}

// Propose validates a plan and writes its change document. It commits nothing; the next
// commit of any kind keeps the document.
func Propose(v *vault.Vault, plan Plan, now time.Time) (*Preview, error) {
	if err := v.CheckLayout(); err != nil {
		return nil, err
	}
	now = now.Truncate(time.Second)
	unlock, err := v.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := vault.Recover(v); err != nil {
		return nil, err
	}
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	p, err := validate(idx, plan, now)
	if err != nil {
		return nil, err
	}
	id := doc.NewID("chg", func(s string) bool { return idx.ByID(s) != nil })
	rel := freePath(idx, now, p.Title)
	content := renderDocument(p, id, now)
	if err := v.Write(rel, []byte(content)); err != nil {
		return nil, err
	}
	if p.Supersedes != nil {
		old := setStatus(p.Supersedes.Content, Superseded, doc.Field{Key: "updated", Value: vault.Stamp(now)})
		if err := v.Write(p.Supersedes.Path, []byte(old)); err != nil {
			return nil, err
		}
	}
	idx, err = vault.Load(v)
	if err != nil {
		return nil, err
	}
	d := idx.ByPath(rel)
	if d == nil {
		return nil, readBack(v, rel)
	}
	return preview(idx, d, p.Ops, outsideRefs(idx, p.Outside), p.Warnings, current(idx)), nil
}

// readBack is the error of a change document the index cannot find after its write.
// The index does not walk into a folder that is a link, so the error names that folder
// and where the file really is.
func readBack(v *vault.Vault, rel string) error {
	parts := strings.Split(rel, "/")
	for i := 1; i < len(parts); i++ {
		folder := strings.Join(parts[:i], "/")
		st, err := os.Lstat(v.Abs(folder))
		if err != nil || st.Mode()&fs.ModeSymlink == 0 {
			continue
		}
		where := v.Abs(rel)
		if real, err := filepath.EvalSymlinks(where); err == nil {
			where = real
			if r := v.Rel(real); r != "" {
				where = r
			}
		}
		return fmt.Errorf("the change document %s was written, at %s, but the vault's index cannot read it back: the folder %s is a link, and the index does not read a linked folder. Make %s a plain folder and move the document into it; the proposal is then ready to review with change show", rel, where, folder, folder)
	}
	return fmt.Errorf("the change document %s was written but the vault's index cannot read it back; check that its frontmatter parses, and propose again", rel)
}

// freePath is the path of a new change document: the date and the title, with (2) and
// on when the day already has one of that name.
func freePath(idx *vault.Index, now time.Time, title string) string {
	base := vault.Date(now) + " " + title
	name := base
	folder := vault.Changes + "/" + now.Format("2006-01") + "/"
	taken := func(name string) bool {
		return len(idx.TitleHolders(name)) > 0 || (idx.V != nil && idx.V.Occupied(folder+name+".md"))
	}
	for n := 2; taken(name); n++ {
		name = fmt.Sprintf("%s (%d)", base, n)
	}
	return folder + name + ".md"
}

func outsideRefs(idx *vault.Index, outside []vault.Rewrite) []vault.Ref {
	out := []vault.Ref{}
	for _, rw := range outside {
		if d := idx.ByPath(rw.Path); d != nil {
			ref := idx.Ref(d)
			if ref.Type == "" {
				ref = vault.Ref{Title: vault.Title(d), Path: d.Path, Tags: []string{}}
			}
			out = append(out, ref)
		}
	}
	return out
}

// before gives a document's content before a change wrote it.
type before func(o *op) string

// current reads a document as the vault holds it now: the before of a change not yet
// applied.
func current(idx *vault.Index) before {
	return func(o *op) string {
		if cur := idx.ByID(o.ID); cur != nil {
			return cur.Content
		}
		return ""
	}
}

// atParent reads a document as the parent of a change's commit held it: the before of an
// applied change.
func atParent(idx *vault.Index, sha string) before {
	g := idx.V.Git()
	parent := g.Parent(sha)
	return func(o *op) string {
		if parent != "" {
			if data, err := g.ShowFile(parent, o.Path); err == nil {
				return string(data)
			}
		}
		return ""
	}
}

// preview describes a change document and its ops.
func preview(idx *vault.Index, d *doc.Doc, ops []*op, outside []vault.Ref, warnings []string, prior before) *Preview {
	pv := &Preview{Ref: idx.Ref(d), Status: d.Str("status"), Counts: ParseCounts(d.Str("counts")), Writes: []WriteLine{}, Rewrites: outside, NewTags: doc.NonNil(d.List("new_tags")), Absorbs: []vault.Ref{}, Warnings: warnings, Reason: d.Str("reason")}
	if pv.Warnings == nil {
		pv.Warnings = []string{}
	}
	if pv.Rewrites == nil {
		pv.Rewrites = []vault.Ref{}
	}
	for _, a := range d.List("absorbs") {
		if ad := idx.Linked(a); ad != nil {
			pv.Absorbs = append(pv.Absorbs, idx.Ref(ad))
		}
	}
	for _, o := range ops {
		w := WriteLine{Op: o.Kind, Title: o.Title, Type: o.Type, Kind: o.TopicKind, Path: o.finalPath()}
		if o.writesContent() {
			nd := doc.Parse(o.Path, []byte(o.Content))
			w.Tags = nd.List("tags")
			if w.Kind == "" {
				w.Kind = nd.Str("kind")
			}
		}
		switch o.Kind {
		case OpCreate:
			w.Lines = fmt.Sprintf("+%d", doc.LineCount(o.Content))
		case OpModify, OpPromote:
			added, removed := diffLines(prior(o), o.Content)
			w.Lines = fmt.Sprintf("+%d −%d", added, removed)
			if o.Rewrite {
				w.Note = "rewrite"
			}
			if o.Kind == OpPromote {
				w.Note = "stub → topic " + o.TopicKind
			}
		case OpRemove:
			if r := idx.ByID(o.Redirect); r != nil {
				w.Note = "links go to " + vault.Title(r)
			}
		case OpConfirm:
			w.Note = "refreshed, no edit"
		case OpRetag:
			w.Title = o.From + " → " + o.To
			w.Note = fmt.Sprintf("%d %s", o.Files, doc.Plural(o.Files, "file", "files"))
			w.Path = ""
		}
		if o.NewTitle != "" && o.Kind != OpRemove {
			w.Note = strings.TrimPrefix(strings.TrimSpace(w.Note+" · → "+o.NewTitle), "· ")
		}
		pv.Writes = append(pv.Writes, w)
	}
	return pv
}

// diffLines counts the lines added and removed between two texts, by their longest
// common subsequence.
func diffLines(a, b string) (int, int) {
	x, y := splitLines(a), splitLines(b)
	if len(x)*len(y) > 4_000_000 {
		return len(y), len(x)
	}
	prev := make([]int, len(y)+1)
	cur := make([]int, len(y)+1)
	for i := 1; i <= len(x); i++ {
		for j := 1; j <= len(y); j++ {
			switch {
			case x[i-1] == y[j-1]:
				cur[j] = prev[j-1] + 1
			case prev[j] >= cur[j-1]:
				cur[j] = prev[j]
			default:
				cur[j] = cur[j-1]
			}
		}
		prev, cur = cur, prev
	}
	common := prev[len(y)]
	return len(y) - common, len(x) - common
}

func splitLines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// Show reads a change document.
func Show(idx *vault.Index, key string) (*Preview, error) {
	d, err := idx.ResolveType(key, "change")
	if err != nil {
		return nil, err
	}
	ops, err := parseWrites(d.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", d.Path, err)
	}
	for _, o := range ops {
		if cur := idx.ByID(o.ID); cur != nil && o.Kind != OpCreate {
			o.Path = cur.Path
			if o.Type == "" {
				o.Type = cur.Type()
			}
		} else if o.Kind == OpCreate {
			o.Path = vault.DocPath(o.Title)
		}
	}
	var outside []vault.Ref
	for _, t := range listedRewrites(d.Body) {
		if od, err := idx.Resolve(t); err == nil {
			outside = append(outside, idx.Ref(od))
		} else if paths := idx.LinkPaths(t); len(paths) == 1 {
			outside = append(outside, vault.Ref{Title: t, Path: paths[0], Tags: []string{}})
		}
	}
	prior := current(idx)
	sha := ""
	if s := d.Str("status"); s == Applied || s == Undone {
		sha, _ = idx.V.Git().FindTrailer(Trailer, d.ID())
		if sha != "" {
			prior = atParent(idx, sha)
		}
	}
	pv := preview(idx, d, ops, outside, nil, prior)
	if d.Str("status") == Applied {
		pv.Commit = sha
	}
	return pv, nil
}

// listedRewrites are the titles under "### link rewrites" and "### tag rewrites".
func listedRewrites(body string) []string {
	var out []string
	in := false
	for _, line := range strings.Split(body, "\n") {
		switch {
		case strings.HasPrefix(line, "### "):
			t := strings.TrimSpace(strings.TrimPrefix(line, "### "))
			in = t == "link rewrites" || t == "tag rewrites"
		case strings.HasPrefix(line, "## "):
			in = false
		case in && strings.HasPrefix(line, "- "):
			title, _, _ := strings.Cut(strings.TrimPrefix(line, "- "), ": ")
			out = append(out, strings.TrimSpace(title))
		}
	}
	return out
}

// Conflict is an apply that found a document changed since the change read it.
type Conflict struct {
	Paths []string
}

func (c *Conflict) Error() string {
	return fmt.Sprintf("conflict: %s changed since the change read it. Read the document again and propose a change that supersedes this one", strings.Join(c.Paths, ", "))
}

// Gate refuses to apply a change document with this many writes, or returns nil.
type Gate func(d *doc.Doc, writes int) error

// newEvent is an event apply writes, with its path known before it writes.
type newEvent struct {
	rel, content, id string
}

// Apply reads a proposed change document again, validates it again, writes its
// documents, and makes one commit. A gate, when given, judges the document Apply
// resolved, under the lock, before anything is written.
func Apply(v *vault.Vault, key string, now time.Time, gate Gate) (_ *Preview, err error) {
	now = now.Truncate(time.Second)
	tx, err := vault.BeginWrite(v)
	if err != nil {
		return nil, err
	}
	defer tx.End(&err)
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	d, err := idx.ResolveType(key, "change")
	if err != nil {
		return nil, err
	}
	if s := d.Str("status"); s != Proposed {
		return nil, fmt.Errorf("%s is %s; only a proposed change applies", vault.Title(d), s)
	}
	ops, err := parseWrites(d.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", d.Path, err)
	}
	if gate != nil {
		if err := gate(d, len(ops)); err != nil {
			return nil, err
		}
	}
	c := newCheck(idx, now, true)
	if err := c.revalidate(ops); err != nil {
		return nil, err
	}
	p := &planned{Title: strings.TrimPrefix(vault.Title(d), vault.Date(createdOf(d))+" "), Ops: ops}
	for _, a := range d.List("absorbs") {
		if ad := idx.Linked(a); ad != nil {
			p.Absorbs = append(p.Absorbs, ad)
		}
	}
	c.rewrites(p)
	if len(c.problems) > 0 {
		return nil, &Refusal{Problems: c.problems}
	}
	// The change's own document may link a document it renames. Its record starts from
	// the rewritten text, and is no outside rewrite of its own.
	record := d.Content
	for i, rw := range p.Outside {
		if rw.Path == d.Path {
			record = rw.Content
			p.Outside = append(p.Outside[:i], p.Outside[i+1:]...)
			break
		}
	}
	derived := describedWrites(idx, p, now)
	titles := thread.NewTitles(idx)
	for _, o := range p.Ops {
		titles.Take(o.Title)
		if o.NewTitle != "" {
			titles.Take(o.NewTitle)
		}
	}
	var events []newEvent
	for _, o := range p.Ops {
		if o.Kind != OpPromote {
			continue
		}
		title := o.Title
		if o.NewTitle != "" {
			title = o.NewTitle
		}
		nd := doc.Parse(o.Path, []byte(o.Content))
		rel, content, id := thread.NewEvent(titles, thread.EventIn{Kind: "promoted", SubjectTitle: title, SubjectID: o.ID, SubjectTags: nd.List("tags"), At: now, By: thread.ByAgent, FromType: "stub", ToType: "topic", Change: vault.Title(d)})
		events = append(events, newEvent{rel: rel, content: content, id: id})
	}
	before := repoPaths(idx)
	lines := preview(idx, d, p.Ops, nil, nil, current(idx)).Writes

	// Record every path the apply may write, so a crash can put them back.
	var paths []string
	for _, o := range p.Ops {
		paths = append(paths, o.Path, o.finalPath())
	}
	for _, rw := range p.Outside {
		paths = append(paths, rw.Path)
	}
	for rel := range derived {
		paths = append(paths, rel)
	}
	for _, e := range events {
		paths = append(paths, e.rel)
	}
	paths = uniqueSorted(paths)
	applying := doc.SetField(setStatus(d.Content, Applying), "paths", paths)
	if err := tx.Write(d.Path, []byte(applying)); err != nil {
		return nil, err
	}
	if err := writeOps(tx, idx, p, derived, now); err != nil {
		return nil, err
	}
	var eventIDs []string
	for _, e := range events {
		if err := tx.Write(e.rel, []byte(e.content)); err != nil {
			return nil, err
		}
		eventIDs = append(eventIDs, e.id)
	}
	counts := countOps(p.Ops, p.Outside)
	final := replaceWrites(record, renderWrites(p.Ops, p.Outside))
	final = doc.SetFields(final, []doc.Field{{Key: "counts", Value: counts.String()}, {Key: "applied", Value: vault.Stamp(now)}, {Key: "updated", Value: vault.Stamp(now)}})
	final = doc.RemoveField(setStatus(final, Applied), "paths")
	// The file says applied, so the derived parts read the change as applied (a source it
	// absorbs is no longer pending), and keeps paths, the mark of an apply in flight, until
	// the commit lands. The commit records the final document, without paths, from the
	// index. A crash before the commit leaves a change recovery puts back; one after it
	// leaves a change whose commit recovery finds and finishes.
	if err := tx.Write(d.Path, []byte(doc.SetField(final, "paths", paths))); err != nil {
		return nil, err
	}
	flight := &inFlight{tx: tx, doc: d.Path, final: final, paths: paths, listed: map[string]bool{}}
	for _, p := range paths {
		flight.listed[p] = true
	}
	if err := syncDerived(v, flight); err != nil {
		return nil, err
	}
	if beforeApplyCommit != nil {
		beforeApplyCommit()
	}
	tx.Stage(d.Path, []byte(final))
	sha, err := tx.Commit("change: "+p.Title, Trailer+": "+d.ID())
	if err != nil {
		return nil, err
	}
	if err := v.Write(d.Path, []byte(final)); err != nil {
		return nil, err
	}
	v.SyncSettings(before)
	idx, err = vault.Load(v)
	if err != nil {
		return nil, err
	}
	done := idx.ByPath(d.Path)
	if done == nil {
		return nil, readBack(v, d.Path)
	}
	pv := preview(idx, done, p.Ops, outsideRefs(idx, p.Outside), c.warnings, current(idx))
	pv.Writes = lines
	pv.Commit = sha
	for _, id := range eventIDs {
		if e := idx.ByID(id); e != nil {
			pv.Events = append(pv.Events, idx.Ref(e))
		}
	}
	return pv, nil
}

func createdOf(d *doc.Doc) time.Time {
	t, _ := schema.ParseTime(d.Str("created"))
	return t
}

// syncDerived brings the derived parts of every document the writes touch up to date,
// in the same commit: statuses, lead callouts, and the sections code writes.
func syncDerived(v *vault.Vault, tx guardWriter) error {
	idx, err := vault.Load(v)
	if err != nil {
		return err
	}
	if _, err := derive.Sync(idx, vault.NewGuard(idx, tx).Write); err != nil {
		return err
	}
	if idx, err = vault.Load(v); err != nil {
		return err
	}
	_, err = thread.Load(idx).SyncWith(vault.NewGuard(idx, tx))
	return err
}

// guardWriter is what a guard writes the derived parts through: the transaction, or an
// apply's in-flight record of it.
type guardWriter interface {
	WriteIfChanged(rel string, content []byte) (bool, error)
	WriteIfUnchanged(rel string, content []byte, want string) (bool, error)
}

// afterUndoRestore lets a test act right after an undo restored its paths. It is nil
// outside tests.
var afterUndoRestore func()

// beforeApplyCommit lets a test see the vault as a crash right before the apply's commit
// would leave it. It is nil outside tests.
var beforeApplyCommit func()

// inFlight writes an apply's derived parts through its transaction, and lists each path
// in the change document's paths before it writes it, so a crash after any derived write
// leaves that path for recovery to put back.
type inFlight struct {
	tx     *vault.Tx
	doc    string
	final  string
	paths  []string
	listed map[string]bool
}

func (f *inFlight) list(rel string) error {
	if f.listed[rel] || rel == f.doc {
		return nil
	}
	f.listed[rel] = true
	f.paths = uniqueSorted(append(f.paths, rel))
	return f.tx.Write(f.doc, []byte(doc.SetField(f.final, "paths", f.paths)))
}

func (f *inFlight) WriteIfChanged(rel string, content []byte) (bool, error) {
	if have, err := f.tx.V.Read(rel); err == nil && string(have) == string(content) {
		return false, nil
	}
	if err := f.list(rel); err != nil {
		return false, err
	}
	return f.tx.WriteIfChanged(rel, content)
}

func (f *inFlight) WriteIfUnchanged(rel string, content []byte, want string) (bool, error) {
	if want == string(content) {
		return false, nil
	}
	if err := f.list(rel); err != nil {
		return false, err
	}
	return f.tx.WriteIfUnchanged(rel, content, want)
}

// revalidate checks the ops a change document holds against the vault as it is now: the
// documents still exist and did not change since, the titles are free, and each
// document's content, which the user may have edited, still fits its type.
func (c *check) revalidate(ops []*op) error {
	var conflicts []string
	for _, o := range ops {
		if o.Kind == OpCreate || o.Kind == OpRetag {
			continue
		}
		d := c.idx.ByID(o.ID)
		if d == nil {
			c.refuse("%s: the document %s is gone", o.label(), o.ID)
			continue
		}
		o.Path = d.Path
		if o.Kind != OpPromote {
			o.Type = d.Type()
		}
		if o.Base != "" && !sameBase(o.Base, d) {
			conflicts = append(conflicts, d.Path)
			continue
		}
		o.Title = vault.Title(d)
		switch o.Kind {
		case OpRename, OpPromote:
			if o.NewTitle != "" {
				c.gone[links.Key(o.Title)] = true
				if links.Key(o.NewTitle) != links.Key(o.Title) {
					c.takeTitle(o, o.NewTitle, d.Path)
				} else {
					c.claimed[links.Key(o.NewTitle)] = o
				}
			}
		case OpRemove:
			c.gone[links.Key(o.Title)] = true
		}
		c.byID[o.ID] = append(c.byID[o.ID], o)
	}
	if len(conflicts) > 0 {
		return &Conflict{Paths: conflicts}
	}
	for _, o := range ops {
		switch o.Kind {
		case OpCreate:
			if !slices.Contains(createTypes, o.Type) {
				c.refuse("%s: a change creates a topic or a repository", o.label())
				continue
			}
			o.Path = vault.DocPath(o.Title)
			c.takeTitle(o, o.Title, "")
		case OpRetag:
			if !c.idx.TagExists(o.From) {
				c.refuse("%s: no document holds %s now", o.label(), o.From)
			}
		}
	}
	for _, o := range ops {
		switch o.Kind {
		case OpCreate, OpModify, OpPromote:
			created := ""
			if cur := c.idx.ByID(o.ID); cur != nil {
				created = cur.Str("created")
				if o.Kind == OpModify {
					o.Content = keepDerived(o.Content, cur)
				}
			}
			o.Content = forceCode(o.Content, o, created, c.now)
			c.checkPage(o, o.Content)
		case OpRemove:
			if o.Redirect != "" {
				if r := c.idx.ByID(o.Redirect); r == nil || c.removed(r.ID()) {
					c.refuse("%s: the redirect %s is gone", o.label(), o.Redirect)
				}
			}
		}
	}
	if len(c.problems) > 0 {
		return &Refusal{Problems: c.problems}
	}
	return nil
}

// forceCode writes the fields code owns into a document the user may have edited: its id
// and type from the heading, its created time, and updated and refreshed now.
func forceCode(content string, o *op, created string, now time.Time) string {
	content = doc.SetField(content, "id", o.ID)
	content = doc.SetField(content, "type", o.Type)
	if created != "" {
		content = doc.SetField(content, "created", created)
	} else if d := doc.Parse("", []byte(content)); d.Str("created") == "" {
		content = doc.SetField(content, "created", vault.Stamp(now))
	}
	return doc.SetFields(content, []doc.Field{{Key: "updated", Value: vault.Stamp(now)}, {Key: "refreshed", Value: vault.Stamp(now)}})
}

// keepDerived gives a modify's content the code-owned fields of the document as it is
// now, so what a sync derived since the proposal (a repository's head, a source's
// status) is not written back to what it was.
func keepDerived(content string, cur *doc.Doc) string {
	t := schema.Get(cur.Type())
	if t == nil || cur.Front == nil {
		return content
	}
	next := doc.Parse(cur.Path, []byte(content))
	for _, f := range t.Fields {
		if f.Owner != schema.Code || !cur.Front.Has(f.Name) || next.Front == nil || !next.Front.Has(f.Name) {
			continue
		}
		switch {
		case cur.Front.IsList(f.Name):
			content = doc.SetField(content, f.Name, cur.List(f.Name))
		case f.Kind == schema.Int:
			content = doc.SetField(content, f.Name, cur.Front.Int(f.Name))
		case f.Kind == schema.Bool:
			content = doc.SetField(content, f.Name, cur.Front.Bool(f.Name))
		default:
			content = doc.SetField(content, f.Name, cur.Str(f.Name))
		}
	}
	return content
}

// describedWrites sets described on each repository whose snapshot the change absorbs,
// with behind at zero. A repository the change also writes takes the fields in its
// content.
func describedWrites(idx *vault.Index, p *planned, now time.Time) map[string]string {
	out := map[string]string{}
	for _, a := range p.Absorbs {
		if a.Type() != "source" || a.Str("origin") != "repository" {
			continue
		}
		repoKey, commit, ok := strings.Cut(a.Str("locator"), "@")
		if !ok {
			continue
		}
		short := commit[:min(7, len(commit))]
		repo := idx.ByID(repoKey)
		if repo == nil {
			repo, _ = idx.ResolveType(repoKey, "repository")
		}
		if repo == nil {
			continue
		}
		set := []doc.Field{{Key: "described", Value: short}, {Key: "behind", Value: 0}, {Key: "refreshed", Value: vault.Stamp(now)}}
		done := false
		for _, o := range p.Ops {
			if o.ID == repo.ID() && o.writesContent() {
				o.Content = doc.SetFields(o.Content, set)
				done = true
			}
		}
		if !done {
			if n, err := (gitx.Repo{Dir: vault.Expand(repo.Str("path"))}).Behind(commit); err == nil {
				set[1].Value = n
			}
			out[repo.Path] = doc.SetFields(repo.Content, append(set, doc.Field{Key: "updated", Value: vault.Stamp(now)}))
		}
	}
	return out
}

// writeOps writes the documents of a change where they land, and the rewrites and the
// derived fields.
func writeOps(tx *vault.Tx, idx *vault.Index, p *planned, derived map[string]string, now time.Time) error {
	final := map[string]string{} // id → where its document lands
	hasContent := map[string]bool{}
	for _, o := range p.Ops {
		if o.ID == "" {
			continue
		}
		if _, ok := final[o.ID]; !ok {
			final[o.ID] = o.Path
		}
		if o.NewTitle != "" && o.Kind != OpRemove {
			final[o.ID] = vault.DocPath(o.NewTitle)
		}
		if o.writesContent() {
			hasContent[o.ID] = true
		}
	}
	for _, o := range p.Ops {
		switch o.Kind {
		case OpCreate:
			if err := tx.Write(o.Path, []byte(o.Content)); err != nil {
				return err
			}
		case OpModify, OpPromote:
			target := final[o.ID]
			if target != o.Path {
				if err := tx.Remove(o.Path); err != nil {
					return err
				}
			}
			if err := tx.Write(target, []byte(o.Content)); err != nil {
				return err
			}
		case OpRename:
			if !hasContent[o.ID] {
				if err := tx.Move(o.Path, final[o.ID]); err != nil {
					return err
				}
			}
		case OpRemove:
			if err := tx.Remove(o.Path); err != nil {
				return err
			}
		case OpConfirm:
			if hasContent[o.ID] {
				continue
			}
			cur := idx.ByID(o.ID)
			if cur == nil {
				continue
			}
			if err := tx.Write(final[o.ID], []byte(doc.SetField(cur.Content, "refreshed", vault.Stamp(now)))); err != nil {
				return err
			}
		}
	}
	for _, rw := range p.Outside {
		if err := tx.Write(rw.Path, []byte(rw.Content)); err != nil {
			return err
		}
	}
	for rel, content := range derived {
		if err := tx.Write(rel, []byte(content)); err != nil {
			return err
		}
	}
	return nil
}

// replaceWrites puts a new Writes section in a change document.
func replaceWrites(content, writes string) string {
	front, body, _ := doc.Split(content)
	for _, h := range doc.Headings(body) {
		if h.Level == 2 && strings.EqualFold(h.Title, "Writes") {
			lines := strings.Split(body, "\n")
			body = strings.Join(lines[:h.Line+1], "\n") + "\n" + writes
			break
		}
	}
	return doc.Join(front, body)
}

func uniqueSorted(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// repoPaths are the paths the repository documents name.
func repoPaths(idx *vault.Index) []string {
	var out []string
	for _, r := range idx.Of("repository") {
		if p := r.Str("path"); p != "" {
			out = append(out, vault.Expand(p))
		}
	}
	return out
}

// Reject sets a proposed change to rejected, with the reason. It commits nothing.
func Reject(v *vault.Vault, key, reason string, now time.Time) (*Preview, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, errors.New("reject needs the reason, in one line")
	}
	unlock, err := v.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := vault.Recover(v); err != nil {
		return nil, err
	}
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	d, err := idx.ResolveType(key, "change")
	if err != nil {
		return nil, err
	}
	if s := d.Str("status"); s != Proposed {
		return nil, fmt.Errorf("%s is %s; only a proposed change can be rejected", vault.Title(d), s)
	}
	content := setStatus(d.Content, Rejected, doc.Field{Key: "reason", Value: strings.TrimSpace(reason)}, doc.Field{Key: "updated", Value: vault.Stamp(now)})
	if err := v.Write(d.Path, []byte(content)); err != nil {
		return nil, err
	}
	idx, err = vault.Load(v)
	if err != nil {
		return nil, err
	}
	return Show(idx, d.ID())
}

// Undo restores the paths of one applied change from the commit before it. It refuses
// when a path changed since, so it never takes back a later edit. It never uses git
// revert, so it needs no clean tree and touches no other path.
func Undo(v *vault.Vault, key string, now time.Time) (_ *Preview, err error) {
	tx, err := vault.BeginWrite(v)
	if err != nil {
		return nil, err
	}
	defer tx.End(&err)
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	d, err := idx.ResolveType(key, "change")
	if err != nil {
		return nil, err
	}
	if s := d.Str("status"); s != Applied {
		return nil, fmt.Errorf("%s is %s; only an applied change can be undone", vault.Title(d), s)
	}
	g := v.Git()
	sha, err := g.FindTrailer(Trailer, d.ID())
	if err != nil {
		return nil, err
	}
	if sha == "" {
		return nil, fmt.Errorf("no commit carries %s: %s", Trailer, d.ID())
	}
	changed, err := g.ChangedPaths(sha)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, p := range changed {
		if p != d.Path && idAt(g, sha, p) != d.ID() {
			paths = append(paths, p)
		}
	}
	var moved []string
	for _, p := range paths {
		same, err := g.Unchanged(sha, p)
		if err != nil {
			return nil, err
		}
		if !same {
			moved = append(moved, p)
		}
	}
	if len(moved) > 0 {
		return nil, fmt.Errorf("undo refused: %s changed since the change applied. Undo the later change first, or make the fix as a new change", strings.Join(moved, ", "))
	}
	before := repoPaths(idx)
	if err := tx.Keep(paths...); err != nil {
		return nil, err
	}
	tx.Indexed()
	if err := g.RestoreFrom(g.Parent(sha), paths...); err != nil {
		return nil, err
	}
	tx.Settle(paths...)
	if afterUndoRestore != nil {
		afterUndoRestore()
	}
	v.Prune(paths...)
	tx.Mark(paths...)
	// Apply rewrote the record's links to the titles the change gave; undo takes them back.
	record := d.Content
	if ops, err := parseWrites(d.Body); err == nil {
		back := links.Rename{}
		for _, o := range ops {
			if o.NewTitle != "" {
				back[o.NewTitle] = o.Title
			}
		}
		if len(back) > 0 {
			record, _, _ = vault.RewriteContent(d, record, back, nil)
		}
	}
	content := setStatus(record, Undone, doc.Field{Key: "updated", Value: vault.Stamp(now)})
	if err := tx.Write(d.Path, []byte(content)); err != nil {
		return nil, err
	}
	if err := syncDerived(v, tx); err != nil {
		return nil, err
	}
	title := strings.TrimSpace(strings.TrimPrefix(vault.Title(d), vault.Date(createdOf(d))))
	undo, err := tx.Commit("undo: "+title, "Atlas-Undo: "+d.ID())
	if err != nil {
		return nil, err
	}
	v.SyncSettings(before)
	idx, err = vault.Load(v)
	if err != nil {
		return nil, err
	}
	pv, err := Show(idx, d.ID())
	if err != nil {
		return nil, err
	}
	pv.Commit = undo
	return pv, nil
}

// idAt is the id of the change document at path in rev, or "".
func idAt(g gitx.Repo, rev, p string) string {
	if !strings.HasPrefix(p, vault.Changes+"/") {
		return ""
	}
	data, err := g.ShowFile(rev, p)
	if err != nil {
		return ""
	}
	return doc.Parse(p, data).ID()
}
