// Package source brings outside documents into the vault and reads any document in
// chunks. Capture copies a file into wiki/sources/files/, never to be edited again, and
// writes its source page; the page stays pending until a change absorbs it.
package source

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/change"
	"github.com/nathanaday/atlas-obsidian/internal/core"
	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	graph "github.com/nathanaday/atlas-obsidian/internal/scope"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// MaxFileSize bounds a file capture takes.
const MaxFileSize = 200 << 20

// Request is what to capture: files in the inbox, pasted text, or a repository.
type Request struct {
	Inbox      []string `json:"inbox,omitempty" jsonschema:"names of files waiting in inbox/"`
	Text       string   `json:"text,omitempty" jsonschema:"text pasted in the conversation, or a passage to keep"`
	Title      string   `json:"title,omitempty" jsonschema:"the title of pasted text"`
	Locator    string   `json:"locator,omitempty" jsonschema:"for text: where it came from, such as this session's document"`
	Repository string   `json:"repository,omitempty" jsonschema:"a repository page (id or title): capture a snapshot of it at its head"`
	Scope      string   `json:"scope,omitempty" jsonschema:"the area or repository the sources belong to, by id or title"`
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
	inbox   string
	scope   string
}

// Capture brings the requested documents into the vault as one commit.
func Capture(v *vault.Vault, req Request, now time.Time) (*Result, error) {
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
	tx, err := change.Begin(v)
	if err != nil {
		return nil, err
	}
	defer tx.Close()
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	scope := ""
	if req.Scope != "" {
		s, err := idx.ResolveType(req.Scope, "area", "repository")
		if err != nil {
			return nil, fmt.Errorf("scope: %w", err)
		}
		scope = doc.Link(vault.Title(s))
	}
	var items []item
	switch {
	case len(req.Inbox) > 0:
		for _, name := range req.Inbox {
			rel := path.Join(vault.Inbox, strings.TrimPrefix(path.Clean("/"+name), "/"))
			st, err := os.Stat(v.Abs(rel))
			if err != nil || st.IsDir() {
				return nil, fmt.Errorf("inbox: %s is not a file in inbox/; vault status lists what waits there", name)
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
			items = append(items, item{title: doc.CleanTitle(strings.TrimSuffix(base, path.Ext(base))), ext: ext, data: data, origin: "inbox", locator: base, inbox: rel, scope: scope})
		}
	case strings.TrimSpace(req.Text) != "":
		title := doc.CleanTitle(req.Title)
		if title == "" {
			return nil, errors.New("captured text needs a title")
		}
		items = append(items, item{title: title, ext: ".md", data: []byte(strings.TrimSpace(req.Text) + "\n"), origin: "pasted", locator: req.Locator, scope: scope})
	default:
		repo, err := idx.ResolveType(req.Repository, "repository")
		if err != nil {
			return nil, err
		}
		snap, err := Snapshot(repo)
		if err != nil {
			return nil, err
		}
		items = append(items, item{title: fmt.Sprintf("%s @ %s", vault.Title(repo), snap.Commit[:7]), ext: ".md", data: snap.Content, origin: "repository", locator: repo.ID() + "@" + snap.Commit, scope: doc.Link(vault.Title(repo))})
	}
	out := &Result{Captured: []Captured{}}
	var titles []string
	var created []string
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
		id := doc.NewID("src", func(s string) bool { return idx.ByID(s) != nil || taken[s] })
		taken[id] = true
		title := it.title
		held := func(title string) bool {
			for _, p := range idx.TitleHolders(title) {
				if p != it.inbox {
					return true
				}
			}
			return taken["t:"+strings.ToLower(title)]
		}
		for i := 2; held(title); i++ {
			title = fmt.Sprintf("%s (%d)", it.title, i)
		}
		taken["t:"+strings.ToLower(title)] = true
		file := id + it.ext
		m := measure(core.Kind(file), it.data)
		if err := tx.Write(path.Join(vault.SourceFiles, file), it.data); err != nil {
			return nil, err
		}
		page := doc.Render([]doc.Field{
			{Key: "id", Value: id},
			{Key: "type", Value: "source"},
			{Key: "created", Value: vault.Date(now)},
			{Key: "updated", Value: vault.Date(now)},
			{Key: "scope", Value: it.scope},
			{Key: "description", Value: "Captured, not yet ingested."},
			{Key: "aliases", Value: []string{}},
			{Key: "tags", Value: []string{}},
			{Key: "sources", Value: []string{}},
			{Key: "file", Value: doc.Link(file)},
			{Key: "sha256", Value: sum},
			{Key: "origin", Value: it.origin},
			{Key: "locator", Value: it.locator},
			{Key: "measure", Value: m.String()},
			{Key: "captured", Value: vault.Date(now)},
			{Key: "authority", Value: "unknown"},
		}, "!"+doc.Link(file)+"\n")
		rel := path.Join(vault.Wiki, "sources", title+".md")
		if err := tx.Write(rel, []byte(page)); err != nil {
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
	if healed, err := vault.Load(v); err == nil {
		if _, err := graph.Heal(healed, tx.WriteIfChanged); err != nil {
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
		chunks, _ := Chunks(idx, d.ID())
		out.Captured = append(out.Captured, Captured{Ref: idx.Ref(d), SHA256: d.Str("sha256"), Measure: d.Str("measure"), Chunks: chunks})
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

func measure(kind string, data []byte) Measure {
	switch kind {
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

// repoFacts is a repository's head, for a snapshot.
func repoHead(root string) (string, error) {
	g := gitx.Repo{Dir: root}
	if !g.HasHead() {
		return "", fmt.Errorf("%s has no commit to snapshot", root)
	}
	return g.Head()
}
