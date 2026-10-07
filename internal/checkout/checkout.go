// Package checkout is the librarian's desk: it ranks the documents a request may need,
// copies the chosen ones into a folder of checkout/ for the user to read and mark up, and,
// at the return, turns the edited copies back into one change and moves the checkout to
// tool/returned/, where it stays as the user left it. A copy takes the name "<Title>
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

	"github.com/nathanaday/almagest/internal/change"
	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/links"
	"github.com/nathanaday/almagest/internal/schema"
	"github.com/nathanaday/almagest/internal/search"
	"github.com/nathanaday/almagest/internal/vault"
)

// Ledger is the note of every checkout, in checkout/: a Base of the checkouts' indexes,
// out and returned. Its prefix is reserved, so no document takes the title.
const (
	Ledger     = "Checkout · Ledger"
	copySuffix = " (checkout)"
	// IndexName is each checkout's own note: its name, request, dates, status, and
	// reading order.
	IndexName = "_index.md"
)

// The status of a checkout: out in checkout/, or returned to tool/returned/.
const (
	StatusOut      = "out"
	StatusReturned = "returned"
)

// IndexPath is the path of a checkout's index.
func IndexPath(folder string) string { return path.Join(folder, IndexName) }

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

// Made is a checkout as made: its folder, its index, and its copies.
type Made struct {
	Folder string   `json:"folder"`
	Index  string   `json:"index"`
	Copies []string `json:"copies"`
	Commit string   `json:"commit,omitempty"`
}

// MaxDocuments bounds one checkout.
const MaxDocuments = 60

// Make copies the chosen documents into a new folder of checkout/, with its index, and
// writes the ledger, in one commit.
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
	out := &Made{Folder: folder, Index: IndexPath(folder), Copies: []string{}}
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
		callout := doc.Callout("almagest", "A copy of "+doc.Link(title)+", checked out "+vault.Date(now),
			"Edit it freely. Return in the Almagest palette proposes your edits to the wiki as a change.")
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
		{Key: "name", Value: name},
		{Key: "request", Value: doc.OneLine(strings.TrimSpace(o.Request), 300)},
		{Key: "checked_out", Value: stamp},
		{Key: "documents", Value: len(docs)},
		{Key: "status", Value: StatusOut},
		{Key: "returned", Value: ""},
		{Key: "return_change", Value: ""},
	}
	body := "# " + name + "\n\n" +
		doc.Callout("almagest", "Checked out "+vault.Date(now), "The librarian's picks, in reading order. The copies are yours to read and edit; Return proposes your edits to the wiki and keeps this checkout in tool/returned/.") +
		"\n\n## Request\n\n" + strings.TrimSpace(o.Request) + "\n\n## Reading order\n\n" + list.String()
	if notes := strings.TrimSpace(o.Notes); notes != "" {
		body += "\n## Notes\n\n" + notes + "\n"
	}
	if err := tx.Write(out.Index, []byte(doc.Render(fields, body))); err != nil {
		return nil, err
	}
	if err := writeLedger(tx); err != nil {
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
		// The link code wrote names the copy by its path; one the user wrote in Obsidian
		// may name it by its file name alone.
		if (dir != "" && strings.TrimSuffix(dir, "/") != folder) || !strings.HasSuffix(file, copySuffix) {
			return l, false
		}
		l.Target = strings.TrimSuffix(file, copySuffix)
		if links.Key(l.Alias) == links.Key(l.Target) {
			l.Alias = ""
		}
		return l, true
	})
}

// Rehash keeps a copy that the user did not edit unedited after code changed its text,
// such as its links at a move: when before's body is the body as checked out, after takes
// its body's hash. An edited copy, or a note that is no copy, comes back as after.
func Rehash(before, after string) string {
	b, a := aCopy{d: doc.Parse("", []byte(before))}, aCopy{d: doc.Parse("", []byte(after))}
	if before == after || b.d.Str("checkout_id") == "" || b.edited() {
		return after
	}
	return doc.SetField(after, "checkout_hash", doc.ContentHash(a.body()))
}

// MoveLinks points every link to the path from, or into the folder from, at the same
// place under to.
func MoveLinks(text, from, to string) string {
	if from == to {
		return text
	}
	return relink(text, func(l links.Link) (links.Link, bool) {
		if l.Target != from && !strings.HasPrefix(l.Target, from+"/") {
			return l, false
		}
		l.Target = to + strings.TrimPrefix(l.Target, from)
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

// Entry is one checkout as the status shows it.
type Entry struct {
	Folder    string `json:"folder"`
	Name      string `json:"name"`
	Request   string `json:"request"`
	Date      string `json:"date"`
	Documents int    `json:"documents"`
	Edited    int    `json:"edited"`
	// Status is out (in checkout/) or returned (in tool/returned/).
	Status   string `json:"status"`
	Returned string `json:"returned"`
}

// List reads every checkout, out and returned, newest first.
func List(v *vault.Vault) []Entry {
	out := []Entry{}
	for _, dir := range []string{vault.Checkout, vault.Returned} {
		entries, _ := os.ReadDir(v.Abs(dir))
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			folder := path.Join(dir, e.Name())
			data, err := v.Read(IndexPath(folder))
			if err != nil {
				continue
			}
			index := doc.Parse("", data)
			copies, _ := readCopies(v, folder)
			edited := 0
			for _, c := range copies {
				if c.edited() {
					edited++
				}
			}
			status := StatusOut
			if dir == vault.Returned {
				status = StatusReturned
			}
			out = append(out, Entry{
				Folder: folder, Name: cmp.Or(index.Str("name"), checkoutName(folder)), Request: index.Str("request"),
				Date: vault.Day(index.Str("checked_out")), Documents: len(copies), Edited: edited, Status: status, Returned: index.Str("returned"),
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return path.Base(out[i].Folder) > path.Base(out[j].Folder) })
	return out
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

// LedgerNote is the ledger's text: a Base of every checkout's index, out or returned,
// newest first. Each row opens the checkout's index.
var LedgerNote = doc.Callout("almagest", "Written by Almagest", "Every checkout: the ones out, in "+vault.Checkout+"/, and the ones returned, in "+vault.Returned+"/. Each row opens the checkout's index.") + `

` + "```base" + `
filters:
  and:
    - 'file.basename == "_index"'
    - or:
        - file.inFolder("` + vault.Checkout + `")
        - file.inFolder("` + vault.Returned + `")
formulas:
  checkout: file.asLink(name)
  out: date(checked_out)
  back: if(returned, date(returned), "")
properties:
  formula.checkout:
    displayName: Checkout
  formula.out:
    displayName: Checked out
  note.documents:
    displayName: Documents
  note.status:
    displayName: Status
  formula.back:
    displayName: Returned
views:
  - type: table
    name: Every checkout
    order:
      - formula.checkout
      - formula.out
      - documents
      - status
      - formula.back
    sort:
      - property: checked_out
        direction: DESC
` + "```" + "\n"

// writeLedger writes the ledger's Base, when it is missing or another.
func writeLedger(tx *vault.Tx) error {
	_, err := tx.WriteIfChanged(path.Join(vault.Checkout, Ledger+".md"), []byte(LedgerNote))
	return err
}

// Returned is what a return did: the change it proposed of the edited copies, if any, the
// copies it left out, and where the checkout went.
type Returned struct {
	Change *change.Preview `json:"change,omitempty"`
	// Folder and Index are the checkout's place in tool/returned/.
	Folder  string   `json:"folder"`
	Index   string   `json:"index"`
	Skipped []string `json:"skipped"`
	// Warning says what went wrong after the change was proposed.
	Warning string `json:"warning,omitempty"`
}

// Return closes a checkout. The edited copies become one proposed change of their
// originals, with each copy's base, so a later edit of an original is a conflict, not a
// loss; a copy whose original is gone or changed since stays out of it, with its edits in
// the copy. Then the checkout moves, every file of it, to tool/returned/, with its links,
// its index marked returned, and the ledger, in one commit.
func Return(v *vault.Vault, key string, now time.Time) (*Returned, error) {
	folder := strings.Trim(path.Clean(strings.TrimPrefix(key, vault.Checkout+"/")), "/")
	if folder == "" || folder == "." || strings.Contains(folder, "/") {
		return nil, fmt.Errorf("%q: name a checkout, a folder of %s/", key, vault.Checkout)
	}
	base := folder
	folder = path.Join(vault.Checkout, folder)
	if err := v.Contain(folder); err != nil {
		return nil, err
	}
	if st, err := os.Stat(v.Abs(folder)); err != nil || !st.IsDir() {
		if st, err := os.Stat(v.Abs(path.Join(vault.Returned, base))); err == nil && st.IsDir() {
			return nil, fmt.Errorf("%s was returned already: it is in %s/; check the documents out again to work on them", base, vault.Returned)
		}
		return nil, fmt.Errorf("no checkout %s", folder)
	}
	copies, err := readCopies(v, folder)
	if err != nil {
		return nil, err
	}
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	dest := path.Join(vault.Returned, base)
	for n := 2; v.Exists(dest); n++ {
		dest = path.Join(vault.Returned, fmt.Sprintf("%s (%d)", base, n))
	}
	out := &Returned{Folder: dest, Index: IndexPath(dest), Skipped: []string{}}
	var writes []change.Write
	name := checkoutName(folder)
	for _, c := range copies {
		if !c.edited() {
			continue
		}
		orig := idx.ByID(c.d.Str("checkout_id"))
		switch {
		case orig == nil:
			out.Skipped = append(out.Skipped, path.Base(c.path)+": its original is gone")
			continue
		case change.BaseHash(orig) != c.d.Str("checkout_base"):
			out.Skipped = append(out.Skipped, path.Base(c.path)+": "+vault.Title(orig)+" changed since the checkout")
			continue
		}
		body := toWiki(c.body(), folder)
		writes = append(writes, change.Write{Op: change.OpModify, ID: orig.ID(), Base: c.d.Str("checkout_base"), Body: &body, Why: "edited in the checkout " + cmp.Or(name, base)})
	}
	changeID := ""
	if len(writes) > 0 {
		pv, err := change.Propose(v, change.Plan{Title: "Return " + name, Notes: "The edits made in the checkout [[" + strings.TrimSuffix(out.Index, ".md") + "|" + base + "]].", Writes: writes}, now)
		if err != nil {
			return out, err
		}
		out.Change = pv
		changeID = pv.Ref.ID
	}
	if err := moveReturned(v, folder, dest, changeID, now); err != nil {
		if out.Change == nil {
			return out, err
		}
		// The change exists; the user decides it in its document. Saying so beats an
		// error that hides it.
		out.Warning = "the change is proposed, but the checkout stays in " + vault.Checkout + "/: " + err.Error()
		out.Folder, out.Index = folder, IndexPath(folder)
	}
	return out, nil
}

// moveReturned moves every file of a checkout to dest, points the links that name the
// checkout's folder at dest, marks the index returned, and writes the ledger, in a commit.
func moveReturned(v *vault.Vault, folder, dest, changeID string, now time.Time) (err error) {
	tx, err := vault.BeginWrite(v)
	if err != nil {
		return err
	}
	defer tx.End(&err)
	var files []string
	err = filepath.WalkDir(v.Abs(folder), func(abs string, e os.DirEntry, err error) error {
		if err == nil && !e.IsDir() && e.Name() != ".DS_Store" {
			files = append(files, v.Rel(abs))
		}
		return err
	})
	if err != nil {
		return err
	}
	for _, rel := range files {
		to := dest + strings.TrimPrefix(rel, folder)
		if err := tx.Move(rel, to); err != nil {
			return err
		}
		if !strings.HasSuffix(strings.ToLower(rel), ".md") {
			continue
		}
		data, err := v.Read(to)
		if err != nil {
			return err
		}
		content := Rehash(string(data), MoveLinks(string(data), folder, dest))
		if to == IndexPath(dest) {
			content = doc.SetFields(content, []doc.Field{{Key: "status", Value: StatusReturned}, {Key: "returned", Value: vault.Stamp(now)}, {Key: "return_change", Value: changeID}})
		}
		if content != string(data) {
			if err := tx.Write(to, []byte(content)); err != nil {
				return err
			}
		}
	}
	prune(v.Abs(folder))
	if err := writeLedger(tx); err != nil {
		return err
	}
	_, err = tx.Commit("checkout: return " + path.Base(folder))
	return err
}

// prune removes the folders under root that the moves left empty, and root, deepest
// first, with the .DS_Store files that Finder leaves. A folder that still holds a file
// stays.
func prune(root string) {
	var dirs []string
	filepath.WalkDir(root, func(p string, e os.DirEntry, err error) error {
		switch {
		case err != nil:
		case e.IsDir():
			dirs = append(dirs, p)
		case e.Name() == ".DS_Store":
			os.Remove(p)
		}
		return nil
	})
	for i := len(dirs) - 1; i >= 0; i-- {
		os.Remove(dirs[i])
	}
}
