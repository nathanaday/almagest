// Package source brings outside documents into the vault and reads any document in
// chunks. Capture copies a file into wiki/assets/, never to be edited again, and writes
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

	"github.com/nathanaday/atlas-obsidian/internal/derive"
	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/tags"
	"github.com/nathanaday/atlas-obsidian/internal/thread"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// MaxFileSize bounds a file capture takes.
const MaxFileSize = 200 << 20

// Request is what to capture: files in the inbox, pasted text, or a repository.
type Request struct {
	Inbox      []string `json:"inbox,omitempty" jsonschema:"names of files waiting in inbox/"`
	Text       string   `json:"text,omitempty" jsonschema:"text pasted in the conversation, or a passage to keep"`
	Title      string   `json:"title,omitempty" jsonschema:"the title of pasted text"`
	Locator    string   `json:"locator,omitempty" jsonschema:"for text: where it came from, such as this session's document or a URL"`
	Repository string   `json:"repository,omitempty" jsonschema:"a repository document (id or title): capture a snapshot of it at its head"`
	Tags       []string `json:"tags,omitempty" jsonschema:"the categories of the sources"`
	Resolves   string   `json:"resolves,omitempty" jsonschema:"an open stub that asked for this source; it closes as resolved"`
	NewTags    bool     `json:"new_tags,omitempty" jsonschema:"allow a tag no document holds, in tagging: known; set it only after the user agreed"`
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
	Captured []Captured  `json:"captured"`
	Events   []vault.Ref `json:"events"`
	Commit   string      `json:"commit,omitempty"`
}

// item is one file to capture.
type item struct {
	title   string
	ext     string
	data    []byte
	origin  string
	locator string
	inbox   string
	tags    []string
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

// Capture brings the requested documents into the vault as one commit.
func Capture(v *vault.Vault, req Request, o thread.Opts) (*Result, error) {
	n := 0
	if len(req.Inbox) > 0 {
		n++
	}
	if strings.TrimSpace(req.Text) != "" {
		n++
	}
	if req.Repository != "" {
		n++
	}
	if n != 1 {
		return nil, errors.New("capture takes one of inbox, text, or repository")
	}
	now := o.Now.Truncate(time.Second)
	tx, err := vault.BeginWrite(v)
	if err != nil {
		return nil, err
	}
	defer tx.Close()
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	tagList, err := tags.NormalizeAll(req.Tags)
	if err != nil {
		return nil, err
	}
	for _, t := range tagList {
		if v.Tagging() == "known" && !req.NewTags && !idx.TagExists(t) {
			return nil, fmt.Errorf("the tag %q is new, and this vault uses known tags; use a tag that exists, or ask the user and call again with new_tags: true", t)
		}
	}
	var stub *doc.Doc
	if req.Resolves != "" {
		if stub, err = idx.ResolveType(req.Resolves, "stub"); err != nil {
			return nil, fmt.Errorf("resolves: %w", err)
		}
		if s := thread.Load(idx).Status(stub); s != thread.StatusStub {
			return nil, fmt.Errorf("resolves: %s is %s; only a stub with no spec resolves into a source", stub.Title(), s)
		}
	}
	var items []item
	switch {
	case len(req.Inbox) > 0:
		for _, name := range req.Inbox {
			rel, err := v.InboxFile(name)
			if err != nil {
				return nil, err
			}
			st, err := os.Stat(v.Abs(rel))
			if err != nil {
				return nil, err
			}
			if st.Size() > MaxFileSize {
				return nil, fmt.Errorf("inbox: %s is %d MB; capture takes files up to 200 MB", name, st.Size()>>20)
			}
			data, err := v.Read(rel)
			if err != nil {
				return nil, err
			}
			base := path.Base(rel)
			ext := strings.ToLower(path.Ext(base))
			// The name is the user's file's, so a long one is cut, not refused; room is left
			// for the " (n)" that tells two captures of one name apart.
			title := doc.CutTitle(doc.CleanTitle(strings.TrimSuffix(base, path.Ext(base))), doc.MaxTitleBytes-len(" (99)"))
			items = append(items, item{title: title, ext: ext, data: data, origin: "inbox", locator: base, inbox: rel, tags: tagList})
		}
	case strings.TrimSpace(req.Text) != "":
		title := doc.CleanTitle(req.Title)
		if title == "" {
			return nil, errors.New("captured text needs a title")
		}
		if err := doc.CheckTitle(title); err != nil {
			return nil, err
		}
		origin := "pasted"
		if strings.HasPrefix(req.Locator, "http://") || strings.HasPrefix(req.Locator, "https://") {
			origin = "url"
		}
		items = append(items, item{title: title, ext: ".md", data: []byte(strings.TrimSpace(req.Text) + "\n"), origin: origin, locator: req.Locator, tags: tagList})
	default:
		repo, err := idx.ResolveType(req.Repository, "repository")
		if err != nil {
			return nil, err
		}
		if repo.Front.Bool("unlinked") {
			return nil, fmt.Errorf("%s is unlinked; there is no repository to snapshot", repo.Title())
		}
		snap, err := Snapshot(repo)
		if err != nil {
			return nil, err
		}
		tg := tagList
		if def := repo.Str("defines"); def != "" && len(tg) == 0 {
			tg = []string{def}
		}
		items = append(items, item{title: fmt.Sprintf("%s @ %s", vault.Title(repo), snap.Commit[:7]), ext: ".md", data: snap.Content, origin: "repository", locator: repo.ID() + "@" + snap.Commit, tags: tg})
	}
	out := &Result{Captured: []Captured{}, Events: []vault.Ref{}}
	var titles, created []string
	taken := map[string]bool{}
	for _, it := range items {
		sum := doc.FileHash(it.data)
		if dup := bySHA(idx, sum); dup != nil {
			out.Captured = append(out.Captured, Captured{Ref: idx.Ref(dup), SHA256: sum, Measure: dup.Str("measure"), Duplicate: dup.ID()})
			if it.inbox != "" {
				if err := tx.Remove(it.inbox); err != nil {
					return nil, err
				}
			}
			continue
		}
		id := doc.NewID(schema.DocPrefix, func(s string) bool { return idx.ByID(s) != nil || taken[s] })
		taken[id] = true
		if it.title == "" {
			// A name of only characters a title cannot hold, such as "###.txt".
			it.title = id
		}
		title := it.title
		held := func(title string) bool {
			if vault.ReservedTitle(title) {
				return true
			}
			for _, p := range idx.TitleHolders(title) {
				if p != it.inbox {
					return true
				}
			}
			return taken["t:"+strings.ToLower(title)] || v.Occupied(vault.DocPath(title))
		}
		for i := 2; held(title); i++ {
			title = fmt.Sprintf("%s (%d)", it.title, i)
		}
		taken["t:"+strings.ToLower(title)] = true
		file := id + it.ext
		media := Media(file)
		m := measure(media, it.data)
		if err := tx.Write(path.Join(vault.Assets, file), it.data); err != nil {
			return nil, err
		}
		stamp := vault.Stamp(now)
		fields := []doc.Field{
			{Key: "id", Value: id},
			{Key: "type", Value: "source"},
			{Key: "description", Value: "Captured, not yet ingested."},
			{Key: "tags", Value: nonNil(it.tags)},
			{Key: "aliases", Value: []string{}},
			{Key: "created", Value: stamp},
			{Key: "updated", Value: stamp},
			{Key: "refreshed", Value: stamp},
			{Key: "authority", Value: "unknown"},
			{Key: "status", Value: "pending"},
			{Key: "file", Value: doc.Link(file)},
			{Key: "media", Value: media},
			{Key: "sha256", Value: sum},
			{Key: "origin", Value: it.origin},
			{Key: "locator", Value: it.locator},
			{Key: "measure", Value: m.String()},
			{Key: "captured", Value: stamp},
		}
		if stub != nil {
			fields = append(fields, doc.Field{Key: "from", Value: doc.Link(stub.Title())})
		}
		rel := vault.DocPath(title)
		if err := tx.Write(rel, []byte(doc.Render(fields, ""))); err != nil {
			return nil, err
		}
		if it.inbox != "" {
			if err := tx.Remove(it.inbox); err != nil {
				return nil, err
			}
		}
		titles = append(titles, title)
		created = append(created, rel)
	}
	var eventID string
	if stub != nil && len(titles) > 0 {
		idx2, err := vault.Load(v)
		if err != nil {
			return nil, err
		}
		if eventID, err = thread.ResolveInTx(tx, idx2, idx2.ByID(stub.ID()), titles, thread.Opts{Now: now, By: o.By}); err != nil {
			return nil, err
		}
	}
	if idx2, err := vault.Load(v); err == nil {
		if _, err := derive.Sync(idx2, vault.NewGuard(idx2, tx).Write); err != nil {
			return nil, err
		}
		if _, err := thread.Load(idx2).SyncWith(vault.NewGuard(idx2, tx)); err != nil {
			return nil, err
		}
	}
	subject := "capture: " + strings.Join(titles, ", ")
	if len(titles) == 0 {
		subject = "capture: duplicates only"
	}
	sha, err := tx.Commit(subject)
	if err != nil {
		return nil, err
	}
	out.Commit = sha
	idx, err = vault.Load(v)
	if err != nil {
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
	if e := idx.ByID(eventID); e != nil {
		out.Events = append(out.Events, idx.Ref(e))
	}
	return out, nil
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
		return Measure{Lines: lineCount(string(data))}
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

func lineCount(s string) int {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// repoHead is a repository's head, for a snapshot.
func repoHead(root string) (string, error) {
	g := gitx.Repo{Dir: root}
	if !g.HasHead() {
		return "", fmt.Errorf("%s has no commit to snapshot", root)
	}
	return g.Head()
}

func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}
