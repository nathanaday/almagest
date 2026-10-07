package source

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/schema"
	"github.com/nathanaday/almagest/internal/vault"
)

// Chunk sizes: the most one careful read takes.
const (
	PagesPerChunk = 20
	LinesPerChunk = 800
)

// Chunk is one part of a document that one worker reads.
type Chunk struct {
	Doc     string `json:"doc"`
	Index   int    `json:"index"`
	Count   int    `json:"count"`
	Locator string `json:"locator"`
	// Size is in pages for a PDF, in lines otherwise.
	Size int `json:"size"`
	// From and To are the first and last page or line, 1-based.
	From int `json:"from"`
	To   int `json:"to"`
}

// TextBlob is one chunk ready for a worker.
type TextBlob struct {
	Doc       string   `json:"doc"`
	Type      string   `json:"type"`
	Kind      string   `json:"kind,omitempty"`
	Title     string   `json:"title"`
	Tags      []string `json:"tags"`
	Authority string   `json:"authority,omitempty"`
	Chunk     Chunk    `json:"chunk"`
	Content   string   `json:"content,omitempty"`
	// File is the captured file a worker reads with its host's Read, for a PDF or an
	// image; Pages is the range to read.
	File  string `json:"file,omitempty"`
	Pages string `json:"pages,omitempty"`
}

// material is what a document's chunks cut: a PDF's pages, or lines of text.
type material struct {
	kind  string // pdf, image, markdown, text
	lines []string
	pages int
	file  string // the captured file, relative to the vault
}

func materialOf(v *vault.Vault, d *doc.Doc) (*material, error) {
	if d.Type() == "source" {
		file := doc.LinkTarget(d.Str("file"))
		if file == "" || file == "." || file == ".." || strings.ContainsAny(file, `/\`) {
			return nil, fmt.Errorf("%s: its file field %q names no file of %s/; a source's file is a plain file name there, as capture writes it", vault.Title(d), file, vault.Originals)
		}
		assets, err := os.OpenRoot(v.Abs(vault.Originals))
		if err != nil {
			return nil, err
		}
		defer assets.Close()
		// The root refuses a link that leads out of tool/source-core/originals.
		if _, err := assets.Stat(file); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%s: the captured file %s leads out of %s/ (%v)", vault.Title(d), file, vault.Originals, err)
		}
		rel := path.Join(vault.Originals, file)
		m := &material{kind: Media(file), file: rel}
		switch m.kind {
		case "pdf":
			m.pages = pagesOf(d.Str("measure"))
			return m, nil
		case "markdown", "text":
		default:
			m.kind = "other"
			return m, nil
		}
		data, err := readIn(assets, file)
		if err != nil {
			return nil, fmt.Errorf("%s: the captured file %s is missing", vault.Title(d), rel)
		}
		m.lines = strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		return m, nil
	}
	var code []string
	if t := schema.Get(d.Type()); t != nil {
		code = t.CodeSections
	}
	body := strings.TrimRight(doc.StripSections(doc.StripLead(d.Body), code), "\n")
	return &material{kind: "markdown", lines: strings.Split(strings.TrimLeft(body, "\n"), "\n")}, nil
}

// Chunks splits a document for reading: a PDF 20 pages at a time, markdown by whole
// sections joined up to 800 lines, text 800 lines at a time cut at a blank line, and
// anything shorter than 800 lines whole.
func Chunks(idx *vault.Index, key string) ([]Chunk, error) {
	d, err := idx.Resolve(key)
	if err != nil {
		return nil, err
	}
	m, err := materialOf(idx.V, d)
	if err != nil {
		return nil, err
	}
	spans := split(m)
	out := make([]Chunk, len(spans))
	for i, s := range spans {
		out[i] = Chunk{Doc: d.ID(), Index: i + 1, Count: len(spans), Locator: s.locator, Size: s.to - s.from + 1, From: s.from, To: s.to}
		if s.from == 0 {
			out[i].Size = 0
		}
	}
	return out, nil
}

type span struct {
	from, to int
	locator  string
}

func split(m *material) []span {
	switch m.kind {
	case "pdf":
		if m.pages <= 0 {
			return []span{{locator: "whole"}}
		}
		var out []span
		for from := 1; from <= m.pages; from += PagesPerChunk {
			to := min(from+PagesPerChunk-1, m.pages)
			out = append(out, span{from, to, fmt.Sprintf("pages %d-%d", from, to)})
		}
		if len(out) == 1 {
			out[0].locator = "whole"
		}
		return out
	case "image", "other":
		return []span{{locator: "whole"}}
	}
	n := len(m.lines)
	if n <= LinesPerChunk {
		return []span{{1, n, "whole"}}
	}
	if m.kind == "markdown" {
		return sections(m.lines)
	}
	return textSpans(m.lines, 1, n)
}

// textSpans cuts lines from..to into runs of at most 800, each ending at a blank line
// near the limit when one is there.
func textSpans(lines []string, from, to int) []span {
	var out []span
	for from <= to {
		end := min(from+LinesPerChunk-1, to)
		if end < to {
			for i := end; i > end-LinesPerChunk/5 && i > from; i-- {
				if strings.TrimSpace(lines[i-1]) == "" {
					end = i
					break
				}
			}
		}
		out = append(out, span{from, end, fmt.Sprintf("lines %d-%d", from, end)})
		from = end + 1
	}
	return out
}

// sections joins a markdown file's sections into runs of at most 800 lines, splitting a
// longer section at its subheadings, and naming the sections each run holds.
func sections(lines []string) []span {
	body := strings.Join(lines, "\n")
	hs := doc.Headings(body)
	type sec struct {
		from, to int
		title    string
		level    int
	}
	var cut func(from, to, level int, title string) []sec
	cut = func(from, to, level int, title string) []sec {
		if to-from+1 <= LinesPerChunk {
			return []sec{{from, to, title, level}}
		}
		// Split at the next level of heading inside the span.
		var inner []doc.Heading
		next := 0
		for _, h := range hs {
			line := h.Line + 1
			if line > from && line <= to && h.Level > level && (next == 0 || h.Level < next) {
				next = h.Level
			}
		}
		for _, h := range hs {
			line := h.Line + 1
			if line > from && line <= to && h.Level == next {
				inner = append(inner, h)
			}
		}
		if len(inner) == 0 {
			var out []sec
			for _, s := range textSpans(lines, from, to) {
				out = append(out, sec{s.from, s.to, title, level})
			}
			return out
		}
		var out []sec
		if first := inner[0].Line; first+1 > from {
			out = append(out, sec{from, first, title, level})
		}
		for i, h := range inner {
			end := to
			if i+1 < len(inner) {
				end = inner[i+1].Line
			}
			out = append(out, cut(h.Line+1, end, h.Level, h.Title)...)
		}
		return out
	}
	top := 0
	for _, h := range hs {
		if top == 0 || h.Level < top {
			top = h.Level
		}
	}
	var secs []sec
	if top == 0 {
		secs = cut(1, len(lines), 0, "")
	} else {
		start := 1
		title := ""
		for _, h := range hs {
			if h.Level != top {
				continue
			}
			if h.Line+1 > start {
				secs = append(secs, cut(start, h.Line, top, title)...)
			}
			start, title = h.Line+1, h.Title
		}
		secs = append(secs, cut(start, len(lines), top, title)...)
	}
	var out []span
	cur := span{}
	var names []string
	flush := func() {
		if cur.from == 0 {
			return
		}
		loc := fmt.Sprintf("lines %d-%d", cur.from, cur.to)
		if len(names) > 0 {
			loc += " (§ " + strings.Join(names, ", § ") + ")"
		}
		cur.locator = loc
		out = append(out, cur)
		cur, names = span{}, nil
	}
	for _, s := range secs {
		if cur.from != 0 && s.to-cur.from+1 > LinesPerChunk {
			flush()
		}
		if cur.from == 0 {
			cur.from = s.from
		}
		cur.to = s.to
		if s.title != "" && (len(names) == 0 || names[len(names)-1] != s.title) {
			names = append(names, s.title)
		}
	}
	flush()
	return out
}

// Read gives one chunk of a document. A PDF or an image is named, not read: the worker
// reads the file with its host's Read, which reads PDFs well.
func Read(idx *vault.Index, key string, index int) (*TextBlob, error) {
	d, err := idx.Resolve(key)
	if err != nil {
		return nil, err
	}
	chunks, err := Chunks(idx, d.ID())
	if err != nil {
		return nil, err
	}
	if index < 1 || index > len(chunks) {
		return nil, fmt.Errorf("%s has chunks 1 to %d, not %d", vault.Title(d), len(chunks), index)
	}
	c := chunks[index-1]
	m, err := materialOf(idx.V, d)
	if err != nil {
		return nil, err
	}
	blob := &TextBlob{Doc: d.ID(), Type: d.Type(), Kind: d.Str("kind"), Title: vault.Title(d), Tags: doc.NonNil(d.List("tags")), Authority: d.Str("authority"), Chunk: c}
	switch m.kind {
	case "pdf", "image", "other":
		blob.File = idx.V.Abs(m.file)
		if m.kind == "pdf" && c.From > 0 {
			blob.Pages = fmt.Sprintf("%d-%d", c.From, c.To)
		}
		return blob, nil
	}
	if c.From > 0 && c.To >= c.From {
		blob.Content = strings.Join(m.lines[c.From-1:c.To], "\n")
	}
	return blob, nil
}

// readIn reads a file through a root, which refuses a path or a link that leads out of
// it.
func readIn(r *os.Root, rel string) ([]byte, error) {
	f, err := r.Open(rel)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}
