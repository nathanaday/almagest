// Package source brings outside documents into the vault and reads any document in
// chunks. Capture copies a file into tool/source-core/originals/, never to be edited again, and writes
// its source document; the source stays pending until a change absorbs it.
package source

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/nathanaday/almagest/internal/derive"
	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/gitx"
	"github.com/nathanaday/almagest/internal/schema"
	"github.com/nathanaday/almagest/internal/tags"
	"github.com/nathanaday/almagest/internal/vault"
)

// MaxFileSize bounds a file capture takes.
const MaxFileSize = 200 << 20

// Request is what to capture: files in ingest/, pasted text, or a repository.
type Request struct {
	Ingest     []string `json:"ingest,omitempty" jsonschema:"names of files waiting in ingest/"`
	Text       string   `json:"text,omitempty" jsonschema:"text pasted in the conversation, or a passage to keep"`
	Title      string   `json:"title,omitempty" jsonschema:"the title of pasted text"`
	Locator    string   `json:"locator,omitempty" jsonschema:"for text: where it came from, such as this session's document or a URL"`
	Repository string   `json:"repository,omitempty" jsonschema:"a repository document (id or title): capture a snapshot of it at its head"`
	Tags       []string `json:"tags,omitempty" jsonschema:"the categories of the sources"`
	NewTags    bool     `json:"new_tags,omitempty" jsonschema:"allow a tag no document holds, in tagging: known; set it only after the user agreed"`
	// Journal makes captured text an edition of a journal volume: code's, never a tool's.
	Journal *Edition `json:"-"`
}

// Edition is what a published journal edition records, and the notes its commit writes
// with it.
type Edition struct {
	Volume string
	Date   string
	Hash   string
	Also   map[string]string
}

// Captured is one source capture made or found.
type Captured struct {
	Ref     vault.Ref `json:"ref"`
	SHA256  string    `json:"sha256"`
	Measure string    `json:"measure"`
	Chunks  []Chunk   `json:"chunks,omitempty"`
	// Duplicate is the id of the source that already held this content.
	Duplicate string `json:"duplicate,omitempty"`
}

// Result is the output of capture.
type Result struct {
	Captured []Captured `json:"captured"`
	Commit   string     `json:"commit,omitempty"`
}

// item is one file to capture.
type item struct {
	title   string
	ext     string
	data    []byte
	origin  string
	locator string
	ingest  string
	tags    []string
	journal *Edition
}

// Media is what a file is, by its extension.
func Media(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".pdf":
		return "pdf"
	case ".md", ".markdown":
		return "markdown"
	case ".txt", ".text", ".csv", ".tsv", ".json", ".yaml", ".yml", ".html", ".htm", ".xml", ".log":
		return "text"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp", ".tif", ".tiff":
		return "image"
	case ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".odt", ".ods", ".odp", ".rtf", ".pages", ".key", ".numbers":
		return "office"
	case ".mp3", ".wav", ".m4a", ".ogg", ".flac":
		return "audio"
	case ".mp4", ".mov", ".webm", ".mkv":
		return "video"
	}
	return "other"
}

// Capture brings the requested documents into the vault as one commit: each file becomes
// an original in tool/source-core/originals and a pending source that names it. A file that
// the vault holds already (the same sha256) is reported as a duplicate, and its ingest copy
// goes.
func Capture(v *vault.Vault, req Request, now time.Time) (_ *Result, err error) {
	if err := checkOneInput(req); err != nil {
		return nil, err
	}
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
	tagList, err := checkTags(v, idx, req)
	if err != nil {
		return nil, err
	}
	items, err := itemsOf(v, idx, req, tagList)
	if err != nil {
		return nil, err
	}
	out := &Result{Captured: []Captured{}}
	var titles, created []string
	taken := map[string]bool{}
	for _, it := range items {
		sum := doc.FileHash(it.data)
		if dup := bySHA(idx, sum); dup != nil {
			out.Captured = append(out.Captured, Captured{Ref: idx.Ref(dup), SHA256: sum, Measure: dup.Str("measure"), Duplicate: dup.ID()})
		} else {
			title, rel, err := writeSource(tx, idx, it, sum, taken, now)
			if err != nil {
				return nil, err
			}
			titles = append(titles, title)
			created = append(created, rel)
		}
		if it.ingest != "" {
			if err := tx.Remove(it.ingest); err != nil {
				return nil, err
			}
		}
	}
	if idx2, err := vault.Load(v); err == nil {
		if _, err := derive.Sync(idx2, vault.NewGuard(idx2, tx).Write); err != nil {
			return nil, err
		}
	}
	subject := "capture: " + strings.Join(titles, ", ")
	if len(titles) == 0 {
		subject = "capture: duplicates only"
	}
	if out.Commit, err = tx.Commit(subject); err != nil {
		return nil, err
	}
	if idx, err = vault.Load(v); err != nil {
		return nil, err
	}
	for _, rel := range created {
		d := idx.ByPath(rel)
		if d == nil {
			continue
		}
		chunks, _ := Chunks(idx, d.ID())
		out.Captured = append(out.Captured, Captured{Ref: idx.Ref(d), SHA256: d.Str("sha256"), Measure: d.Str("measure"), Chunks: chunks})
	}
	return out, nil
}

// checkOneInput refuses a request that names none, or more than one, of its inputs.
func checkOneInput(req Request) error {
	n := 0
	for _, given := range []bool{len(req.Ingest) > 0, strings.TrimSpace(req.Text) != "", req.Repository != ""} {
		if given {
			n++
		}
	}
	if n != 1 {
		return errors.New("capture takes one of ingest, text, or repository")
	}
	return nil
}

// checkTags normalizes the request's tags, and refuses a new one in a vault of known tags
// unless the request says the user agreed.
func checkTags(v *vault.Vault, idx *vault.Index, req Request) ([]string, error) {
	tagList, err := tags.NormalizeAll(req.Tags)
	if err != nil {
		return nil, err
	}
	for _, t := range tagList {
		if v.Tagging() == "known" && !req.NewTags && !idx.TagExists(t) {
			return nil, fmt.Errorf("the tag %q is new, and this vault uses known tags; use a tag that exists, or ask the user and call again with new_tags: true", t)
		}
	}
	return tagList, nil
}

// itemsOf turns the request's one input into the files to capture.
func itemsOf(v *vault.Vault, idx *vault.Index, req Request, tagList []string) ([]item, error) {
	switch {
	case len(req.Ingest) > 0:
		return ingestItems(v, req.Ingest, tagList)
	case strings.TrimSpace(req.Text) != "":
		it, err := textItem(req, tagList)
		return []item{it}, err
	}
	it, err := repositoryItem(idx, req.Repository, tagList)
	return []item{it}, err
}

// ingestItems reads the named files of ingest/.
func ingestItems(v *vault.Vault, names, tagList []string) ([]item, error) {
	var items []item
	for _, name := range names {
		rel, err := v.IngestFile(name)
		if err != nil {
			return nil, err
		}
		st, err := os.Stat(v.Abs(rel))
		if err != nil {
			return nil, err
		}
		if st.Size() > MaxFileSize {
			return nil, fmt.Errorf("ingest: %s is %d MB; capture takes files up to 200 MB", name, st.Size()>>20)
		}
		data, err := v.Read(rel)
		if err != nil {
			return nil, err
		}
		base := path.Base(rel)
		// The name is the user's file's, so a long one is cut, not refused; room is left
		// for the " (n)" that tells two captures of one name apart.
		title := doc.CutTitle(doc.CleanTitle(strings.TrimSuffix(base, path.Ext(base))), doc.MaxTitleBytes-len(" (99)"))
		items = append(items, item{title: title, ext: strings.ToLower(path.Ext(base)), data: data, origin: "ingest", locator: base, ingest: rel, tags: tagList})
	}
	return items, nil
}

// textItem is pasted text, a page from a URL, or a journal edition, as one markdown file.
func textItem(req Request, tagList []string) (item, error) {
	title := doc.CleanTitle(req.Title)
	if title == "" {
		return item{}, errors.New("captured text needs a title")
	}
	if err := doc.CheckTitle(title); err != nil {
		return item{}, err
	}
	origin := "pasted"
	if strings.HasPrefix(req.Locator, "http://") || strings.HasPrefix(req.Locator, "https://") {
		origin = "url"
	}
	if req.Journal != nil {
		origin = "journal"
	}
	return item{title: title, ext: ".md", data: []byte(strings.TrimSpace(req.Text) + "\n"), origin: origin, locator: req.Locator, tags: tagList, journal: req.Journal}, nil
}

// repositoryItem is a snapshot of a linked repository at its current commit. With no tag
// asked for, it takes the tag the repository defines.
func repositoryItem(idx *vault.Index, key string, tagList []string) (item, error) {
	repo, err := idx.ResolveType(key, "repository")
	if err != nil {
		return item{}, err
	}
	if repo.Front.Bool("unlinked") {
		return item{}, fmt.Errorf("%s is unlinked; there is no repository to snapshot", repo.Title())
	}
	snap, err := Snapshot(repo)
	if err != nil {
		return item{}, err
	}
	if def := repo.Str("defines"); def != "" && len(tagList) == 0 {
		tagList = []string{def}
	}
	return item{title: fmt.Sprintf("%s @ %s", vault.Title(repo), snap.Commit[:7]), ext: ".md", data: snap.Content, origin: "repository", locator: repo.ID() + "@" + snap.Commit, tags: tagList}, nil
}

// writeSource writes one item's original and its source document, under a free title, and
// returns the title and the document's path. taken holds the ids and titles of this
// capture so far.
func writeSource(tx *vault.Tx, idx *vault.Index, it item, sum string, taken map[string]bool, now time.Time) (string, string, error) {
	v := idx.V
	id := doc.NewID(schema.DocPrefix, func(s string) bool { return idx.ByID(s) != nil || taken[s] })
	taken[id] = true
	if it.title == "" {
		// A name of only characters a title cannot hold, such as "###.txt".
		it.title = id
	}
	held := func(title string) bool {
		if vault.ReservedTitle(title) {
			return true
		}
		for _, p := range idx.TitleHolders(title) {
			if p != it.ingest {
				return true
			}
		}
		return taken["t:"+strings.ToLower(title)] || v.Occupied(vault.DocPath(title))
	}
	title := it.title
	for i := 2; held(title); i++ {
		title = fmt.Sprintf("%s (%d)", it.title, i)
	}
	taken["t:"+strings.ToLower(title)] = true
	file := id + it.ext
	if err := tx.Write(path.Join(vault.Originals, file), it.data); err != nil {
		return "", "", err
	}
	if j := it.journal; j != nil {
		for rel, content := range j.Also {
			if err := tx.Write(rel, []byte(content)); err != nil {
				return "", "", err
			}
		}
	}
	rel := vault.DocPath(title)
	if err := tx.Write(rel, []byte(doc.Render(sourceFields(it, id, file, sum, now), ""))); err != nil {
		return "", "", err
	}
	return title, rel, nil
}

// sourceFields are the frontmatter of a new source. A journal edition is a primary source,
// and records its volume, its date, and the hash of its notes.
func sourceFields(it item, id, file, sum string, now time.Time) []doc.Field {
	stamp := vault.Stamp(now)
	media := Media(file)
	authority := "unknown"
	if it.journal != nil {
		authority = "primary"
	}
	fields := []doc.Field{
		{Key: "id", Value: id},
		{Key: "type", Value: "source"},
		{Key: "description", Value: "Captured, not yet ingested."},
		{Key: "tags", Value: doc.NonNil(it.tags)},
		{Key: "aliases", Value: []string{}},
		{Key: "created", Value: stamp},
		{Key: "updated", Value: stamp},
		{Key: "refreshed", Value: stamp},
		{Key: "authority", Value: authority},
		{Key: "status", Value: "pending"},
		{Key: "file", Value: doc.Link(file)},
		{Key: "media", Value: media},
		{Key: "sha256", Value: sum},
		{Key: "origin", Value: it.origin},
		{Key: "locator", Value: it.locator},
		{Key: "measure", Value: measure(media, it.data).String()},
		{Key: "captured", Value: stamp},
	}
	if j := it.journal; j != nil {
		fields = append(fields, doc.Field{Key: "volume", Value: j.Volume}, doc.Field{Key: "edition", Value: j.Date}, doc.Field{Key: "journal_hash", Value: j.Hash})
	}
	return fields
}

func bySHA(idx *vault.Index, sum string) *doc.Doc {
	for _, s := range idx.Of("source") {
		if s.Str("sha256") == sum {
			return s
		}
	}
	return nil
}

// Measure is how large a document is, in the unit a reader splits it by.
type Measure struct {
	Pages int
	Lines int
}

func (m Measure) String() string {
	switch {
	case m.Pages > 0:
		return fmt.Sprintf("%d pages", m.Pages)
	case m.Lines > 0:
		return fmt.Sprintf("%d lines", m.Lines)
	}
	return ""
}

func measure(media string, data []byte) Measure {
	switch media {
	case "pdf":
		return Measure{Pages: PDFPages(data)}
	case "markdown", "text":
		return Measure{Lines: doc.LineCount(string(data))}
	}
	return Measure{}
}

// pagesOf reads the page count a measure string holds.
func pagesOf(measure string) int {
	if f := strings.Fields(measure); len(f) == 2 && f[1] == "pages" {
		n, _ := strconv.Atoi(f[0])
		return n
	}
	return 0
}

// repoHead is a repository's head, for a snapshot.
func repoHead(root string) (string, error) {
	g := gitx.Repo{Dir: root}
	if !g.HasHead() {
		return "", fmt.Errorf("%s has no commit to snapshot", root)
	}
	return g.Head()
}
