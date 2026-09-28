package change

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/doc"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/links"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/schema"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/scope"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/vault"
)

// Trailer names the change a commit applied.
const Trailer = "Atlas-Change"

// Preview is what a change does, enough to show it in the chat.
type Preview struct {
	Ref          vault.Ref   `json:"ref"`
	Status       string      `json:"status"`
	Counts       Counts      `json:"counts"`
	Writes       []WriteLine `json:"writes"`
	LinkRewrites []vault.Ref `json:"link_rewrites"`
	Absorbs      []vault.Ref `json:"absorbs"`
	Warnings     []string    `json:"warnings"`
	Commit       string      `json:"commit,omitempty"`
	Reason       string      `json:"reason,omitempty"`
}

// WriteLine is one write of a preview.
type WriteLine struct {
	Op    string `json:"op"`
	Title string `json:"title"`
	Path  string `json:"path"`
	Lines string `json:"lines,omitempty"`
	Note  string `json:"note,omitempty"`
}

// Begin starts a write that ends in one commit: the lock, the repair of a change a crash
// left applying, and the snapshot of hand edits. Every write tool starts with it.
func Begin(v *vault.Vault) (*vault.Tx, error) {
	return vault.Begin(v, func() error { return Recover(v) })
}

// Recover finds each change a crash left applying, puts back every path it may have
// written, and sets it to proposed. The caller holds the lock.
func Recover(v *vault.Vault) error {
	files, _ := filepath.Glob(v.Abs(vault.Changes + "/*/*.md"))
	g := v.Git()
	for _, abs := range files {
		data, err := os.ReadFile(abs)
		if err != nil || !strings.Contains(string(data), "status: "+Applying) {
			continue
		}
		d := doc.Parse(v.Rel(abs), data)
		if d.Str("status") != Applying {
			continue
		}
		for _, p := range d.List("paths") {
			if err := g.RestoreFrom("HEAD", p); err != nil {
				return fmt.Errorf("recover %s: %w", vault.Title(d), err)
			}
		}
		content := doc.RemoveField(setStatus(d.Content, Proposed), "paths")
		if err := v.Write(d.Path, []byte(content)); err != nil {
			return err
		}
	}
	return nil
}

// Propose validates a plan and writes its change document. It commits nothing; the next
// commit of any kind keeps the document.
func Propose(v *vault.Vault, plan Plan, now time.Time) (*Preview, error) {
	unlock, err := v.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := Recover(v); err != nil {
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
		old := setStatus(p.Supersedes.Content, Superseded, doc.Field{Key: "updated", Value: vault.Date(now)})
		if err := v.Write(p.Supersedes.Path, []byte(old)); err != nil {
			return nil, err
		}
	}
	idx, err = vault.Load(v)
	if err != nil {
		return nil, err
	}
	d := idx.ByPath(rel)
	return preview(idx, d, p.Ops, outsideRefs(idx, p.Outside), p.Warnings, current(idx)), nil
}

// freePath is the path of a new change document: the date and the title, with (2) and
// on when the day already has one of that name.
func freePath(idx *vault.Index, now time.Time, title string) string {
	base := vault.Date(now) + " " + title
	name := base
	for n := 2; ; n++ {
		if len(idx.TitleHolders(name)) == 0 {
			break
		}
		name = fmt.Sprintf("%s (%d)", base, n)
	}
	return fmt.Sprintf("%s/%s/%s.md", vault.Changes, now.Format("2006-01"), name)
}

func outsideRefs(idx *vault.Index, outside []Rewrite) []vault.Ref {
	out := []vault.Ref{}
	for _, rw := range outside {
		if d := idx.ByPath(rw.Path); d != nil {
			out = append(out, idx.Ref(d))
		}
	}
	return out
}

// before gives a page's content before a change wrote it, and the page's path then.
type before func(o *op) (content, path string)

// current reads a page as the vault holds it now: the before of a change not yet applied.
func current(idx *vault.Index) before {
	return func(o *op) (string, string) {
		if cur := idx.ByID(o.ID); cur != nil {
			return cur.Content, cur.Path
		}
		return "", o.Path
	}
}

// atParent reads a page as the parent of a change's commit held it: the before of an
// applied change.
func atParent(idx *vault.Index, sha string) before {
	g := idx.V.Git()
	parent := g.Parent(sha)
	return func(o *op) (string, string) {
		if parent != "" {
			if data, err := g.ShowFile(parent, o.Path); err == nil {
				return string(data), o.Path
			}
		}
		return "", o.Path
	}
}

// preview describes a change document and its ops.
func preview(idx *vault.Index, d *doc.Doc, ops []*op, outside []vault.Ref, warnings []string, prior before) *Preview {
	pv := &Preview{Ref: idx.Ref(d), Status: d.Str("status"), Counts: ParseCounts(d.Str("counts")), Writes: []WriteLine{}, LinkRewrites: outside, Absorbs: []vault.Ref{}, Warnings: warnings, Reason: d.Str("reason")}
	if pv.Warnings == nil {
		pv.Warnings = []string{}
	}
	if pv.LinkRewrites == nil {
		pv.LinkRewrites = []vault.Ref{}
	}
	for _, a := range d.List("absorbs") {
		if ad := idx.Linked(a); ad != nil {
			pv.Absorbs = append(pv.Absorbs, idx.Ref(ad))
		}
	}
	for _, o := range ops {
		w := WriteLine{Op: o.Kind, Title: o.Title, Path: o.Path}
		switch o.Kind {
		case "create":
			w.Lines = fmt.Sprintf("+%d", lineCount(o.Content))
		case "modify":
			var was string
			was, w.Path = prior(o)
			added, removed := diffLines(was, o.Content)
			w.Lines = fmt.Sprintf("+%d −%d", added, removed)
			if o.Rewrite {
				w.Note = "link rewrite"
			}
		case "rename":
			w.Note = "→ " + o.NewTitle
		case "remove":
			if r := idx.ByID(o.Redirect); r != nil {
				w.Note = "links go to " + vault.Title(r)
			}
		}
		pv.Writes = append(pv.Writes, w)
	}
	return pv
}

func lineCount(s string) int {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
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
		if cur := idx.ByID(o.ID); cur != nil && o.Kind != "create" {
			o.Path = cur.Path
		} else if o.Kind == "create" {
			o.Path = Route(o.Type, o.Title)
		}
	}
	var outside []vault.Ref
	for _, t := range listedRewrites(d.Body) {
		if od, err := idx.Resolve(t); err == nil {
			outside = append(outside, idx.Ref(od))
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

// listedRewrites are the titles under "### link rewrites".
func listedRewrites(body string) []string {
	var out []string
	in := false
	for _, line := range strings.Split(body, "\n") {
		switch {
		case strings.HasPrefix(line, "### "):
			in = strings.TrimSpace(strings.TrimPrefix(line, "### ")) == "link rewrites"
		case strings.HasPrefix(line, "## "):
			in = false
		case in && strings.HasPrefix(line, "- "):
			title, _, _ := strings.Cut(strings.TrimPrefix(line, "- "), ": ")
			out = append(out, strings.TrimSpace(title))
		}
	}
	return out
}

// Conflict is an apply that found a page changed since the change read it.
type Conflict struct {
	Paths []string
}

func (c *Conflict) Error() string {
	return fmt.Sprintf("conflict: %s changed since the change read it. Read the page again and propose a change that supersedes this one", strings.Join(c.Paths, ", "))
}

// Apply reads a proposed change document again, validates it again, writes its pages,
// and makes one commit.
func Apply(v *vault.Vault, key string, now time.Time) (*Preview, error) {
	tx, err := Begin(v)
	if err != nil {
		return nil, err
	}
	defer tx.Close()
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
	c := &check{idx: idx, now: now, claimed: map[string]*op{}, gone: map[string]bool{}, byID: map[string][]*op{}}
	if err := c.revalidate(ops); err != nil {
		return nil, err
	}
	p := &planned{Title: strings.TrimPrefix(vault.Title(d), d.Str("created")+" "), Ops: ops}
	for _, a := range d.List("absorbs") {
		if ad := idx.Linked(a); ad != nil {
			p.Absorbs = append(p.Absorbs, ad)
		}
	}
	c.rewrites(p)
	if len(c.problems) > 0 {
		return nil, &Refusal{Problems: c.problems}
	}
	// The change's own document may link a page it renames, in its absorbs, its Absorbed
	// table, or its notes. Its record starts from the rewritten text, and is no outside
	// rewrite of its own.
	record := d.Content
	for i, rw := range p.Outside {
		if rw.Path == d.Path {
			record = rw.Content
			p.Outside = append(p.Outside[:i], p.Outside[i+1:]...)
			break
		}
	}
	derived := describedWrites(idx, p, now)
	before := repoPaths(idx)
	lines := preview(idx, d, p.Ops, nil, nil, current(idx)).Writes

	// Record every path the apply may write, so a crash can put them back.
	var paths []string
	for _, o := range p.Ops {
		paths = append(paths, o.Path)
		if o.NewPath != "" {
			paths = append(paths, o.NewPath)
		}
	}
	for _, rw := range p.Outside {
		paths = append(paths, rw.Path)
	}
	for rel := range derived {
		paths = append(paths, rel)
	}
	paths = uniqueSorted(paths)
	applying := doc.SetField(setStatus(d.Content, Applying), "paths", paths)
	if err := tx.Write(d.Path, []byte(applying)); err != nil {
		return nil, err
	}
	if err := writeOps(tx, p, derived); err != nil {
		return nil, err
	}
	counts := countOps(p.Ops, len(p.Outside))
	if err := healScopes(v, tx); err != nil {
		return nil, err
	}
	final := replaceWrites(record, renderWrites(p.Ops, p.Outside))
	final = doc.SetFields(final, []doc.Field{{Key: "counts", Value: counts.String()}, {Key: "applied", Value: vault.Stamp(now)}, {Key: "updated", Value: vault.Date(now)}})
	final = doc.RemoveField(setStatus(final, Applied), "paths")
	if err := tx.Write(d.Path, []byte(final)); err != nil {
		return nil, err
	}
	sha, err := tx.Commit("change: "+p.Title, Trailer+": "+d.ID())
	if err != nil {
		return nil, err
	}
	v.SyncSettings(before)
	idx, err = vault.Load(v)
	if err != nil {
		return nil, err
	}
	pv := preview(idx, idx.ByPath(d.Path), p.Ops, outsideRefs(idx, p.Outside), c.warnings, current(idx))
	pv.Writes = lines
	pv.Commit = sha
	return pv, nil
}

// revalidate checks the ops a change document holds against the vault as it is now: the
// pages still exist and did not change since, the titles are free, and each page's
// content, which the user may have edited, still fits its type.
func (c *check) revalidate(ops []*op) error {
	var conflicts []string
	for _, o := range ops {
		if o.Kind == "create" {
			continue
		}
		d := c.idx.ByID(o.ID)
		if d == nil {
			c.refuse("%s: the page %s is gone", o.label(), o.ID)
			continue
		}
		o.Path, o.Type = d.Path, d.Type()
		if o.Base != "" && !doc.SameHash(o.Base, doc.FileHash([]byte(d.Content))) {
			conflicts = append(conflicts, d.Path)
			continue
		}
		switch o.Kind {
		case "rename":
			o.NewPath = path.Dir(d.Path) + "/" + o.NewTitle + ".md"
			c.gone[links.Key(vault.Title(d))] = true
			if links.Key(o.NewTitle) != links.Key(vault.Title(d)) {
				c.takeTitle(o, o.NewTitle, d.Path)
			} else {
				c.claimed[links.Key(o.NewTitle)] = o
			}
			o.Title = vault.Title(d)
		case "remove":
			c.gone[links.Key(vault.Title(d))] = true
			o.Title = vault.Title(d)
		}
		c.byID[o.ID] = append(c.byID[o.ID], o)
	}
	if len(conflicts) > 0 {
		return &Conflict{Paths: conflicts}
	}
	for _, o := range ops {
		if o.Kind == "create" {
			if !contains(modelTypes, o.Type) {
				c.refuse("%s: a change creates an %s", o.label(), strings.Join(modelTypes, ", "))
				continue
			}
			o.Path = Route(o.Type, o.Title)
			c.takeTitle(o, o.Title, "")
		}
	}
	for _, o := range ops {
		switch o.Kind {
		case "create":
			o.Content = forceCode(o.Content, o, "", c.now)
			c.checkPage(o, o.Content)
		case "modify":
			o.Content = forceCode(o.Content, o, c.idx.ByID(o.ID).Str("created"), c.now)
			c.checkPage(o, o.Content)
		case "remove":
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

// forceCode writes the fields code owns into a page the user may have edited: its id and
// type from the heading, its created date, and updated today.
func forceCode(content string, o *op, created string, now time.Time) string {
	content = doc.SetField(content, "id", o.ID)
	content = doc.SetField(content, "type", o.Type)
	if created != "" {
		content = doc.SetField(content, "created", created)
	} else if d := doc.Parse("", []byte(content)); d.Str("created") == "" {
		content = doc.SetField(content, "created", vault.Date(now))
	}
	return doc.SetField(content, "updated", vault.Date(now))
}

// describedWrites sets described on each repository page whose snapshot the change
// absorbs. A page the change also modifies takes the field in its content.
func describedWrites(idx *vault.Index, p *planned, now time.Time) map[string]string {
	out := map[string]string{}
	for _, a := range p.Absorbs {
		if a.Type() != "source" || a.Str("origin") != "repository" {
			continue
		}
		repoID, commit, ok := strings.Cut(a.Str("locator"), "@")
		if !ok {
			continue
		}
		short := commit[:min(7, len(commit))]
		repo := idx.ByID(repoID)
		if repo == nil {
			continue
		}
		done := false
		for _, o := range p.Ops {
			if o.ID == repo.ID() && o.writesContent() {
				o.Content = doc.SetField(o.Content, "described", short)
				done = true
			}
		}
		if !done {
			out[repo.Path] = doc.SetFields(repo.Content, []doc.Field{{Key: "described", Value: short}, {Key: "updated", Value: vault.Date(now)}})
		}
	}
	return out
}

// writeOps writes the pages of a change, its link rewrites, and the derived fields.
func writeOps(tx *vault.Tx, p *planned, derived map[string]string) error {
	for _, o := range p.Ops {
		switch o.Kind {
		case "create":
			if err := tx.Write(o.Path, []byte(o.Content)); err != nil {
				return err
			}
		case "modify":
			target := o.Path
			for _, r := range p.Ops {
				if r.ID == o.ID && r.Kind == "rename" {
					target = r.NewPath
				}
			}
			if target != o.Path {
				if err := tx.Remove(o.Path); err != nil {
					return err
				}
			}
			if err := tx.Write(target, []byte(o.Content)); err != nil {
				return err
			}
		}
	}
	for _, o := range p.Ops {
		switch o.Kind {
		case "rename":
			modified := false
			for _, m := range p.Ops {
				if m.ID == o.ID && m.Kind == "modify" {
					modified = true
				}
			}
			if !modified {
				if err := tx.Move(o.Path, o.NewPath); err != nil {
					return err
				}
			}
		case "remove":
			if err := tx.Remove(o.Path); err != nil {
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

// repoPaths are the paths the repository pages name.
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
	if err := Recover(v); err != nil {
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
	content := setStatus(d.Content, Rejected, doc.Field{Key: "reason", Value: strings.TrimSpace(reason)}, doc.Field{Key: "updated", Value: vault.Date(now)})
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
func Undo(v *vault.Vault, key string, now time.Time) (*Preview, error) {
	tx, err := Begin(v)
	if err != nil {
		return nil, err
	}
	defer tx.Close()
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
	if err := g.RestoreFrom(g.Parent(sha), paths...); err != nil {
		return nil, err
	}
	tx.Mark(paths...)
	// Apply rewrote the record's links to the titles the change gave; undo takes them back.
	record := d.Content
	if ops, err := parseWrites(d.Body); err == nil {
		back := links.Rename{}
		for _, o := range ops {
			if o.Kind == "rename" {
				back[o.NewTitle] = o.Title
			}
		}
		if len(back) > 0 {
			record, _, _ = rewriteContent(d, record, back, nil)
		}
	}
	content := setStatus(record, Undone, doc.Field{Key: "updated", Value: vault.Date(now)})
	if err := tx.Write(d.Path, []byte(content)); err != nil {
		return nil, err
	}
	if err := healScopes(v, tx); err != nil {
		return nil, err
	}
	title := strings.TrimSpace(strings.TrimPrefix(vault.Title(d), d.Str("created")))
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

// healScopes brings every page's chain, the scope callouts, and the map in Atlas.md up
// to date with the writes, in the same commit: a new area or a new parent moves the pages
// below it.
func healScopes(v *vault.Vault, tx *vault.Tx) error {
	idx, err := vault.Load(v)
	if err != nil {
		return err
	}
	_, err = scope.Heal(idx, tx.WriteIfChanged)
	return err
}

// Wiki reports whether a type's documents are pages a change writes.
func Wiki(typ string) bool {
	t := schema.Get(typ)
	return t != nil && t.Wiki()
}
