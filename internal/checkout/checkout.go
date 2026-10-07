// Package checkout is the librarian's desk: it ranks the documents a request may need,
// copies the chosen ones into a folder of checkout/ for the user to read and mark up, and
// turns the edited copies back into one change. A copy takes the name "<Title>
// (checkout)", so no link of the wiki ever names it by accident.
package checkout

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/change"
	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/links"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/search"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Names in a checkout's folder.
const (
	ReadingList = "Reading list"
	Ledger      = "Ledger"
	copySuffix  = " (checkout)"
)

// Bounds of a candidate list.
const (
	DefaultLimit = 40
	MaxDistance  = 2
	decay        = 0.6
	seeds        = 12
)

// Candidate is one document the librarian may check out.
type Candidate struct {
	Ref      vault.Ref `json:"ref"`
	Score    float64   `json:"score"`
	Distance int       `json:"distance"`
	Via      string    `json:"via"`
	Words    int       `json:"words"`
}

// Candidates ranks what a request may need: the search's best hits, and the documents
// linked to or from them within two steps. A linked document scores the larger of its
// own rank and its seed's rank, cut by a step's decay. The choice stays the librarian's.
func Candidates(idx *vault.Index, text string, tagList, types []string, limit int) ([]Candidate, error) {
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("candidates needs the request")
	}
	if len(types) == 0 {
		types = []string{"topic", "repository"}
	}
	if limit <= 0 {
		limit = DefaultLimit
	}
	hits, err := search.Search(idx, search.Query{Text: text, Tags: tagList, Types: types, Limit: 1000})
	if err != nil {
		return nil, err
	}
	own := map[string]float64{}
	for _, h := range hits.Hits {
		own[h.Ref.ID] = h.Score
	}
	type seen struct {
		score    float64
		distance int
		via      string
	}
	best := map[string]*seen{}
	var frontier []string
	for i, h := range hits.Hits {
		if i == seeds {
			break
		}
		best[h.Ref.ID] = &seen{score: h.Score}
		frontier = append(frontier, h.Ref.ID)
	}
	allowed := func(d *doc.Doc) bool { return d != nil && slicesContain(types, d.Type()) }
	for step := 1; step <= MaxDistance; step++ {
		var next []string
		for _, id := range frontier {
			from := idx.ByID(id)
			if from == nil {
				continue
			}
			inherited := best[id].score * decay
			for _, nb := range neighbors(idx, from) {
				if !allowed(nb) || nb.ID() == "" {
					continue
				}
				score := max(inherited, own[nb.ID()])
				if s, ok := best[nb.ID()]; ok {
					if score > s.score && s.distance > 0 {
						s.score, s.via = score, vault.Title(from)
					}
					continue
				}
				best[nb.ID()] = &seen{score: score, distance: step, via: vault.Title(from)}
				next = append(next, nb.ID())
			}
		}
		frontier = next
	}
	out := make([]Candidate, 0, len(best))
	for id, s := range best {
		d := idx.ByID(id)
		if d == nil {
			continue
		}
		out = append(out, Candidate{Ref: idx.Ref(d), Score: round(s.score), Distance: s.distance, Via: s.via, Words: len(strings.Fields(doc.StripLead(d.Body)))})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Ref.Title < out[j].Ref.Title
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func round(f float64) float64 { return float64(int(f*1000+0.5)) / 1000 }

func slicesContain(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// neighbors are the documents a document links, and the documents that link it.
func neighbors(idx *vault.Index, d *doc.Doc) []*doc.Doc {
	var out []*doc.Doc
	for _, t := range linkTargets(d) {
		if n := idx.Linked(t); n != nil && n.ID() != d.ID() {
			out = append(out, n)
		}
	}
	for _, p := range idx.Backlinks(d.Path) {
		if n := idx.ByPath(p); n != nil && n.ID() != "" && idx.ByID(n.ID()) == n {
			out = append(out, n)
		}
	}
	return out
}

func linkTargets(d *doc.Doc) []string {
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
	for _, l := range links.Find(d.Body) {
		out = append(out, l.Target)
	}
	return out
}

// Pick is one document the librarian chose, with why.
type Pick struct {
	ID  string `json:"id" jsonschema:"the document, by id or title"`
	Why string `json:"why,omitempty" jsonschema:"one line: why it serves the request"`
}

// Order is what make takes.
type Order struct {
	Request   string `json:"request,omitempty" jsonschema:"make: the user's request, in the user's words"`
	Name      string `json:"name,omitempty" jsonschema:"make: a short name for the checkout; the folder's name"`
	Documents []Pick `json:"documents,omitempty" jsonschema:"make: the documents in reading order, foundations first"`
	Notes     string `json:"notes,omitempty" jsonschema:"make: what the reader should know: what was left out, and why"`
}

// Made is a checkout as made: its folder, its reading list, and its copies.
type Made struct {
	Folder      string   `json:"folder"`
	ReadingList string   `json:"reading_list"`
	Copies      []string `json:"copies"`
	Commit      string   `json:"commit,omitempty"`
}

// MaxDocuments bounds one checkout.
const MaxDocuments = 60

// Make copies the chosen documents into a new folder of checkout/, with a reading list,
// and writes the ledger, in one commit.
func Make(v *vault.Vault, o Order, now time.Time) (_ *Made, err error) {
	if err := v.CheckLayout(); err != nil {
		return nil, err
	}
	now = now.Truncate(time.Second)
	name := doc.CleanTitle(o.Name)
	switch {
	case strings.TrimSpace(o.Request) == "":
		return nil, errors.New("make needs the request")
	case name == "":
		return nil, errors.New("make needs a name for the checkout")
	case len(o.Documents) == 0:
		return nil, errors.New("make needs at least one document")
	case len(o.Documents) > MaxDocuments:
		return nil, fmt.Errorf("a checkout holds at most %d documents; narrow the request", MaxDocuments)
	}
	if err := doc.CheckTitle(name); err != nil {
		return nil, fmt.Errorf("name: %w", err)
	}
	tx, err := vault.BeginWrite(v)
	if err != nil {
		return nil, err
	}
	defer tx.End(&err)
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	var docs []*doc.Doc
	whys := map[string]string{}
	for _, p := range o.Documents {
		d, err := idx.Resolve(p.ID)
		if err != nil {
			return nil, err
		}
		if !schema.IsDocument(d.Type()) || !vault.InPlace(d) {
			return nil, fmt.Errorf("%s is a %s; a checkout copies sources, repositories, and topics", vault.Title(d), d.Type())
		}
		if _, dup := whys[d.ID()]; dup {
			return nil, fmt.Errorf("%s is named twice", vault.Title(d))
		}
		whys[d.ID()] = doc.OneLine(strings.TrimSpace(p.Why), 200)
		docs = append(docs, d)
	}
	base := vault.Date(now) + " " + name
	folder := path.Join(vault.Checkout, base)
	for n := 2; v.Exists(folder); n++ {
		folder = path.Join(vault.Checkout, fmt.Sprintf("%s (%d)", base, n))
	}
	copyOf := map[string]string{} // original title key → the copy's path without .md
	for _, d := range docs {
		copyOf[links.Key(vault.Title(d))] = path.Join(folder, vault.Title(d)+copySuffix)
		for _, a := range d.List("aliases") {
			copyOf[links.Key(a)] = path.Join(folder, vault.Title(d)+copySuffix)
		}
	}
	out := &Made{Folder: folder, ReadingList: path.Join(folder, ReadingList+".md"), Copies: []string{}}
	stamp := vault.Stamp(now)
	var list strings.Builder
	for i, d := range docs {
		title := vault.Title(d)
		body := toCopies(strings.TrimSpace(doc.StripLead(d.Body)), copyOf) + "\n"
		rel := path.Join(folder, title+copySuffix+".md")
		fields := []doc.Field{
			{Key: "checkout_of", Value: doc.Link(title)},
			{Key: "checkout_id", Value: d.ID()},
			{Key: "checkout_base", Value: change.BaseHash(d)},
			{Key: "checkout_hash", Value: doc.ContentHash(body)},
			{Key: "checked_out", Value: stamp},
			{Key: "description", Value: d.Str("description")},
		}
		callout := doc.Callout("atlas", "A copy of "+doc.Link(title)+", checked out "+vault.Date(now),
			"Edit it freely. Return in the Atlas palette proposes your edits to the wiki as a change.")
		if err := tx.Write(rel, []byte(doc.Render(fields, callout+"\n\n"+body))); err != nil {
			return nil, err
		}
		out.Copies = append(out.Copies, rel)
		fmt.Fprintf(&list, "%d. [[%s|%s]]", i+1, strings.TrimSuffix(rel, ".md"), title)
		if why := whys[d.ID()]; why != "" {
			list.WriteString(" · " + why)
		}
		list.WriteString("\n")
	}
	fields := []doc.Field{
		{Key: "request", Value: doc.OneLine(strings.TrimSpace(o.Request), 300)},
		{Key: "checked_out", Value: stamp},
		{Key: "documents", Value: len(docs)},
		{Key: "returned", Value: ""},
	}
	body := doc.Callout("atlas", "Checked out "+vault.Date(now), "The librarian's picks, in reading order. The copies are yours to read and edit.") +
		"\n\n## Request\n\n" + strings.TrimSpace(o.Request) + "\n\n## Reading order\n\n" + list.String()
	if notes := strings.TrimSpace(o.Notes); notes != "" {
		body += "\n## Notes\n\n" + notes + "\n"
	}
	if err := tx.Write(out.ReadingList, []byte(doc.Render(fields, body))); err != nil {
		return nil, err
	}
	if err := writeLedger(v, tx, folder); err != nil {
		return nil, err
	}
	if out.Commit, err = tx.Commit("checkout: " + path.Base(folder)); err != nil {
		return nil, err
	}
	return out, nil
}

// toCopies points each link to a document of the checkout at its copy; every other
// link keeps naming the wiki.
func toCopies(text string, copyOf map[string]string) string {
	return relink(text, func(l links.Link) (links.Link, bool) {
		to, ok := copyOf[links.Key(l.Target)]
		if !ok || strings.Contains(l.Target, "/") {
			return l, false
		}
		if l.Alias == "" {
			l.Alias = l.Target
		}
		l.Target = to
		return l, true
	})
}

// toWiki points each link at a copy back at the copy's original. A link whose text is
// the original's title goes back to the bare title, as the original wrote it.
func toWiki(text, folder string) string {
	return relink(text, func(l links.Link) (links.Link, bool) {
		dir, file := path.Split(l.Target)
		if strings.TrimSuffix(dir, "/") != folder || !strings.HasSuffix(file, copySuffix) {
			return l, false
		}
		l.Target = strings.TrimSuffix(file, copySuffix)
		if links.Key(l.Alias) == links.Key(l.Target) {
			l.Alias = ""
		}
		return l, true
	})
}

func relink(text string, fn func(links.Link) (links.Link, bool)) string {
	var b strings.Builder
	last := 0
	for _, l := range links.Find(text) {
		next, ok := fn(l)
		if !ok {
			continue
		}
		b.WriteString(text[last:l.Start])
		b.WriteString(next.String())
		last = l.End
	}
	b.WriteString(text[last:])
	return b.String()
}

// Entry is one checkout as the ledger and the status show it.
type Entry struct {
	Folder    string `json:"folder"`
	Request   string `json:"request"`
	Date      string `json:"date"`
	Documents int    `json:"documents"`
	Edited    int    `json:"edited"`
	Returned  string `json:"returned"`
}

// List reads every checkout, newest first.
func List(v *vault.Vault) []Entry {
	out := []Entry{}
	entries, _ := os.ReadDir(v.Abs(vault.Checkout))
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		folder := path.Join(vault.Checkout, e.Name())
		data, err := v.Read(path.Join(folder, ReadingList+".md"))
		if err != nil {
			continue
		}
		rl := doc.Parse("", data)
		copies, _ := readCopies(v, folder)
		edited := 0
		for _, c := range copies {
			if c.edited() {
				edited++
			}
		}
		out = append(out, Entry{Folder: folder, Request: rl.Str("request"), Date: vault.Day(rl.Str("checked_out")), Documents: len(copies), Edited: edited, Returned: returned(v, rl)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Folder > out[j].Folder })
	return out
}

// returned is when a checkout was returned, or "" when its return change is gone,
// rejected, superseded, or undone, which frees the checkout to return again.
func returned(v *vault.Vault, rl *doc.Doc) string {
	when, id := rl.Str("returned"), rl.Str("return_change")
	if when == "" || id == "" {
		return when
	}
	files, _ := filepath.Glob(v.Abs(vault.Changes + "/*/*.md"))
	for _, abs := range files {
		f, err := vault.ReadFront(abs)
		if err != nil || f.Str("id") != id {
			continue
		}
		switch f.Str("status") {
		case change.Proposed, change.Applying, change.Applied:
			return when
		}
		return ""
	}
	return ""
}

// checkoutName is a checkout's name: its folder's, without the date.
func checkoutName(folder string) string {
	base := path.Base(folder)
	if len(base) > 11 && base[4] == '-' && base[7] == '-' && base[10] == ' ' {
		return base[11:]
	}
	return base
}

// aCopy is one copy of a checkout, as it lies now.
type aCopy struct {
	path string
	d    *doc.Doc
}

func (c aCopy) body() string {
	return strings.TrimSpace(doc.StripLead(c.d.Body)) + "\n"
}

func (c aCopy) edited() bool {
	return !doc.SameHash(c.d.Str("checkout_hash"), doc.ContentHash(c.body()))
}

func readCopies(v *vault.Vault, folder string) ([]aCopy, error) {
	entries, err := os.ReadDir(v.Abs(folder))
	if err != nil {
		return nil, err
	}
	var out []aCopy
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), copySuffix+".md") {
			continue
		}
		rel := path.Join(folder, e.Name())
		data, err := v.Read(rel)
		if err != nil {
			continue
		}
		if d := doc.Parse(rel, data); d.Str("checkout_id") != "" {
			out = append(out, aCopy{rel, d})
		}
	}
	return out, nil
}

// writeLedger writes checkout/Ledger.md from the reading lists. The folder being made
// counts, though the index has not read it.
func writeLedger(v *vault.Vault, tx *vault.Tx, _ string) error {
	var b strings.Builder
	b.WriteString(doc.Callout("atlas", "Written by Atlas at each checkout and return", "Every checkout, newest first.") + "\n\n")
	b.WriteString("| Checked out | Request | Documents | Edited | Returned |\n|---|---|---|---|---|\n")
	for _, e := range List(v) {
		returned := "no"
		if e.Returned != "" {
			returned = vault.Day(e.Returned)
		}
		req := strings.ReplaceAll(e.Request, "|", "\\|")
		fmt.Fprintf(&b, "| %s | [[%s/%s\\|%s]] | %d | %d | %s |\n", e.Date, e.Folder, ReadingList, req, e.Documents, e.Edited, returned)
	}
	return tx.Write(path.Join(vault.Checkout, Ledger+".md"), []byte(b.String()))
}

// Returned is what a return proposed: the change, and the copies left out.
type Returned struct {
	Change  *change.Preview `json:"change,omitempty"`
	Skipped []string        `json:"skipped"`
}

// Return turns the edited copies of a checkout into one proposed change of their
// originals, with each copy's base, so a later edit of an original is a conflict, not a
// loss. A copy whose original is gone or changed since is left out.
func Return(v *vault.Vault, key string, now time.Time) (*Returned, error) {
	folder := strings.Trim(path.Clean(strings.TrimPrefix(key, vault.Checkout+"/")), "/")
	if folder == "" || folder == "." || strings.Contains(folder, "/") {
		return nil, fmt.Errorf("%q: name a checkout, a folder of %s/", key, vault.Checkout)
	}
	folder = path.Join(vault.Checkout, folder)
	if err := v.Contain(folder); err != nil {
		return nil, err
	}
	copies, err := readCopies(v, folder)
	if err != nil {
		return nil, fmt.Errorf("no checkout %s", folder)
	}
	// A return proposes against each original as it was checked out. While that change
	// is proposed or applied, the checkout is returned; a cancelled or undone one frees it.
	if data, err := v.Read(path.Join(folder, ReadingList+".md")); err == nil {
		if when := returned(v, doc.Parse("", data)); when != "" {
			return nil, fmt.Errorf("%s was returned %s; check the documents out again to edit them further", folder, vault.Day(when))
		}
	}
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	out := &Returned{Skipped: []string{}}
	var writes []change.Write
	name := checkoutName(folder)
	for _, c := range copies {
		if !c.edited() {
			continue
		}
		orig := idx.ByID(c.d.Str("checkout_id"))
		switch {
		case orig == nil:
			out.Skipped = append(out.Skipped, c.path+": its original is gone")
			continue
		case change.BaseHash(orig) != c.d.Str("checkout_base"):
			out.Skipped = append(out.Skipped, c.path+": "+vault.Title(orig)+" changed since the checkout")
			continue
		}
		body := toWiki(c.body(), folder)
		writes = append(writes, change.Write{Op: change.OpModify, ID: orig.ID(), Base: c.d.Str("checkout_base"), Body: &body, Why: "edited in the checkout " + cmp.Or(name, path.Base(folder))})
	}
	if len(writes) == 0 {
		if len(out.Skipped) > 0 {
			return out, fmt.Errorf("no edited copy can return: %s", strings.Join(out.Skipped, "; "))
		}
		return out, errors.New("no copy of " + folder + " was edited; there is nothing to return")
	}
	pv, err := change.Propose(v, change.Plan{Title: "Return " + name, Notes: "The edits made in the checkout [[" + path.Join(folder, ReadingList) + "|" + path.Base(folder) + "]].", Writes: writes}, now)
	if err != nil {
		return out, err
	}
	out.Change = pv
	if err := markReturned(v, folder, pv.Ref.ID, now); err != nil {
		return out, err
	}
	return out, nil
}

// markReturned records the return in the reading list and the ledger, in a commit.
func markReturned(v *vault.Vault, folder, changeID string, now time.Time) (err error) {
	tx, err := vault.BeginWrite(v)
	if err != nil {
		return err
	}
	defer tx.End(&err)
	rel := path.Join(folder, ReadingList+".md")
	data, err := v.Read(rel)
	if err != nil {
		return err
	}
	content := doc.SetFields(string(data), []doc.Field{{Key: "returned", Value: vault.Stamp(now)}, {Key: "return_change", Value: changeID}})
	if err := tx.Write(rel, []byte(content)); err != nil {
		return err
	}
	if err := writeLedger(v, tx, folder); err != nil {
		return err
	}
	_, err = tx.Commit("checkout: return " + path.Base(folder))
	return err
}
