package migrate

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/sessions"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// chords is the folder of the chord canvases in 8.x.
const chords = "chords"

// legacyDocuments is where 8.x and 9.0 kept the documents.
const legacyDocuments = "wiki/documents"

// Fields that served threads, by type.
var dropped = map[string][]string{
	"vault":   {"wikify"},
	"source":  {"from"},
	"topic":   {"from"},
	"session": {"threads", "specs", "work", "checked", "events"},
	"change":  {"work"},
}

// build9 computes the step from 8.x to 9.0, which the folders of 9.0 frame: the thread
// documents of wiki/documents and the canvases of chords/ move to threads/, with the
// paths they name updated, and the documents that stay lose the fields and sections that
// served threads.
func build9(v *vault.Vault, report *Report) (*plan, error) {
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	p := &plan{}
	for _, d := range idx.Notes {
		if path.Dir(d.Path) == legacyDocuments && slices.Contains(schema.ArchivedTypes, d.Type()) {
			p.moves = append(p.moves, Move{From: d.Path, To: vault.Threads + "/" + path.Base(d.Path)})
		}
	}
	var left []string
	err = filepath.WalkDir(v.Abs(chords), func(abs string, e fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		rel := v.Rel(abs)
		switch {
		case e.IsDir() || e.Name() == ".DS_Store":
		case strings.HasSuffix(strings.ToLower(e.Name()), ".canvas"):
			p.moves = append(p.moves, Move{From: rel, To: vault.Threads + strings.TrimPrefix(rel, chords)})
		default:
			left = append(left, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(left) > 0 {
		report.Warnings = append(report.Warnings, fmt.Sprintf("%s/ keeps the files that are no canvas, and the folder stays: %s", chords, strings.Join(left, ", ")))
	}
	sort.Slice(p.moves, func(i, j int) bool { return p.moves[i].From < p.moves[j].From })
	if err := claim(v, p.moves); err != nil {
		return nil, err
	}
	report.Moved = append(report.Moved, p.moves...)
	// The cards of a chord's canvas name their stubs by path, and a chord names its
	// canvas by path, so both follow the moves.
	paths := strings.NewReplacer(pathPairs(p.moves)...)
	for _, m := range p.moves {
		data, err := v.Read(m.From)
		if err != nil {
			return nil, err
		}
		if content := paths.Replace(string(data)); content != string(data) {
			p.edits = append(p.edits, edit{m.To, content})
		}
	}
	// The documents of 9.0's wiki/documents are misplaced for this release's index, which
	// reads source-core/documents; the step reads them where they lie.
	for _, d := range append(append([]*doc.Doc{}, idx.Docs...), idx.Misplaced...) {
		content := d.Content
		for _, f := range dropped[d.Type()] {
			if d.Front.Has(f) {
				content = doc.RemoveField(content, f)
			}
		}
		if t := d.Type(); t == "topic" || t == "repository" {
			front, body, _ := doc.Split(content)
			if trimmed := doc.RemoveSection(body, "Threads"); trimmed != body {
				content = doc.Join(front, trimmed)
			}
		}
		switch d.Type() {
		case "vault":
			content = doc.SetField(content, "layout", vault.LayoutKB)
		case "session":
			content = doc.ReplaceLead(content, sessions.Lead(doc.Parse(d.Path, []byte(content))))
		}
		if content != d.Content {
			p.edits = append(p.edits, edit{d.Path, content})
			report.Edited = append(report.Edited, d.Path)
		}
	}
	if v.Doc.Front.Has("wikify") && !slices.Contains(v.Doc.List("wikify"), "source") {
		report.Warnings = append(report.Warnings, "wikify left sources out, so no source was pending; 9.0 has no wikify, and every source the wiki has not absorbed is pending")
	}
	for _, s := range idx.Of("session") {
		if st := s.Str("status"); st == "running" || st == "waiting" {
			report.Warnings = append(report.Warnings, fmt.Sprintf("the session %s is %s; an agent of 8.x that runs in it still calls the thread tool, which 9.0 does not have", s.Title(), st))
		}
	}
	return p, nil
}

// pathPairs are the old and new forms of each moved path, as a link or a canvas card
// writes it: the whole path, and the path without .md.
func pathPairs(moves []Move) []string {
	var out []string
	for _, m := range moves {
		out = append(out, `"`+m.From+`"`, `"`+m.To+`"`, "[["+m.From+"|", "[["+m.To+"|", "[["+m.From+"]]", "[["+m.To+"]]")
		if strings.HasSuffix(m.From, ".md") {
			from, to := strings.TrimSuffix(m.From, ".md"), strings.TrimSuffix(m.To, ".md")
			out = append(out, "[["+from+"|", "[["+to+"|", "[["+from+"]]", "[["+to+"]]")
		}
	}
	return out
}

// changes reads the change documents of the vault, whatever its layout.
func changes(v *vault.Vault) []*doc.Doc {
	var out []*doc.Doc
	files, _ := filepath.Glob(v.Abs(vault.Changes + "/*/*.md"))
	for _, abs := range files {
		if data, err := os.ReadFile(abs); err == nil {
			if d := doc.Parse(v.Rel(abs), data); d.Type() == "change" {
				out = append(out, d)
			}
		}
	}
	return out
}
