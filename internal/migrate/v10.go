package migrate

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
	"github.com/nathanaday/atlas-obsidian/internal/views"
)

// The folders of 9.0 that 10.0 renames.
const (
	legacyWiki   = "wiki"
	legacyInbox  = "inbox"
	legacyViews  = "views"
	legacyAssets = "wiki/assets"
)

// renames map each 9.0 folder to its 10.0 place, the longest first, so a file takes the
// most specific one.
var renames = []struct{ from, to string }{
	{legacyDocuments, vault.Documents},
	{legacyAssets, vault.Originals},
	{legacyViews + "/tags", vault.WikiView + "/" + views.NavFolder},
	{legacyWiki, vault.Core},
	{legacyInbox, vault.Ingest},
	{legacyViews, vault.WikiView},
}

// renamed is a 9.0 path at its 10.0 place, and whether a rename applies.
func renamed(rel string) (string, bool) {
	for _, r := range renames {
		if rel == r.from || strings.HasPrefix(rel, r.from+"/") {
			return r.to + strings.TrimPrefix(rel, r.from), true
		}
	}
	return rel, false
}

// pathRef matches a 9.0 folder where a path stands as a path: right after [[, ![[, ](,
// or a quote, with its slash, or alone between quotes (a Base's inFolder).
var pathRef = regexp.MustCompile(`(\[\[|\]\(|"|')(wiki/documents|wiki/assets|wiki|inbox|views/tags|views)(/|"|')`)

// rewritePaths points the path references of one text at the 10.0 folders. With quoted
// false, only links count: a quoted path in a note's prose is a record of what was.
func rewritePaths(s string, quoted bool) string {
	return pathRef.ReplaceAllStringFunc(s, func(m string) string {
		parts := pathRef.FindStringSubmatch(m)
		open, dir, close := parts[1], parts[2], parts[3]
		link := open == "[[" || open == "]("
		switch {
		case !quoted && !link:
			return m
		case close != "/" && (link || close != open || (dir != legacyDocuments && dir != legacyAssets)):
			return m
		}
		to, _ := renamed(dir)
		return open + to + close
	})
}

// rewriteRefs points a file's path references at the 10.0 folders. A canvas and a Base
// name paths in quotes; a note names them in links, and in quotes only inside a base
// block. A note's code, inline or fenced, stays as written.
func rewriteRefs(ext, s string) string {
	if ext != ".md" {
		return rewritePaths(s, true)
	}
	lines := strings.Split(s, "\n")
	fence := ""
	for i, l := range lines {
		t := strings.TrimSpace(l)
		switch {
		case fence == "" && (strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")):
			fence = t
			continue
		case fence != "" && (t == "```" || t == "~~~" || strings.HasPrefix(t, fence[:3]) && strings.Trim(t, fence[:1]) == ""):
			fence = ""
			continue
		case fence != "":
			if strings.TrimLeft(fence, "`~ ") == "base" {
				lines[i] = rewritePaths(l, true)
			}
			continue
		}
		spans := strings.Split(l, "`")
		for j := 0; j < len(spans); j += 2 {
			spans[j] = rewritePaths(spans[j], false)
		}
		lines[i] = strings.Join(spans, "`")
	}
	return strings.Join(lines, "\n")
}

// build10 computes the step from 9.0 to 10.0: every file of wiki/, inbox/, and views/
// moves to its 10.0 folder, except the notes code wrote in views/, which go; the path
// references of every note, canvas, and Base but the captured originals follow; a
// source's origin inbox becomes ingest; and Atlas.md takes the layout of 10.0.
func build10(v *vault.Vault, report *Report) (*plan, error) {
	p := &plan{}
	taken := map[string]bool{}
	for _, dir := range []string{legacyWiki, legacyInbox} {
		err := walkFiles(v, dir, func(rel string) error {
			to, _ := renamed(rel)
			p.moves = append(p.moves, Move{From: rel, To: to})
			taken[strings.ToLower(to)] = true
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	err := walkFiles(v, legacyViews, func(rel string) error {
		if strings.HasSuffix(rel, ".md") {
			if data, err := v.Read(rel); err == nil && strings.HasPrefix(string(data), views.Notice) {
				p.removes = append(p.removes, rel)
				report.Removed = append(report.Removed, rel)
				return nil
			}
		}
		to := freeName(v, vault.Ingest+"/"+path.Base(rel), taken)
		p.moves = append(p.moves, Move{From: rel, To: to})
		report.Strays = append(report.Strays, vault.Moved{From: rel, To: to})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(p.moves, func(i, j int) bool { return p.moves[i].From < p.moves[j].From })
	if err := claim(v, p.moves); err != nil {
		return nil, err
	}
	report.Moved = append(report.Moved, p.moves...)
	dest := map[string]string{}
	for _, m := range p.moves {
		dest[m.From] = m.To
	}
	gone := map[string]bool{}
	for _, r := range p.removes {
		gone[r] = true
	}
	err = filepath.WalkDir(v.Root, func(abs string, e fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel := v.Rel(abs)
		if e.IsDir() {
			if abs != v.Root && (strings.HasPrefix(e.Name(), ".") || rel == vault.Trash || rel == vault.WikiView || e.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(path.Ext(rel))
		if gone[rel] || (ext != ".md" && ext != ".canvas" && ext != ".base") || rel == legacyAssets || strings.HasPrefix(rel, legacyAssets+"/") {
			// A captured original stays as it was captured; its hash names it.
			return nil
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		content := rewriteRefs(ext, string(data))
		if ext == ".md" {
			content = retype(rel, content)
		}
		if content == string(data) {
			return nil
		}
		at := rel
		if to, ok := dest[rel]; ok {
			at = to
		}
		p.edits = append(p.edits, edit{at, content})
		report.Edited = append(report.Edited, at)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return p, nil
}

// retype gives a note the fields of 10.0: the vault document its layout, and a source
// captured from the inbox the origin ingest.
func retype(rel, content string) string {
	d := doc.Parse(rel, []byte(content))
	switch {
	case rel == vault.Marker && d.Type() == "vault":
		return doc.SetField(content, "layout", vault.Layout)
	case d.Type() == "source" && d.Str("origin") == legacyInbox:
		return doc.SetField(content, "origin", "ingest")
	}
	return content
}

// walkFiles calls fn with the vault path of every file under dir, but .DS_Store.
func walkFiles(v *vault.Vault, dir string, fn func(rel string) error) error {
	return filepath.WalkDir(v.Abs(dir), func(abs string, e fs.DirEntry, err error) error {
		switch {
		case err != nil:
			if os.IsNotExist(err) {
				return nil
			}
			return err
		case e.IsDir() || e.Name() == ".DS_Store":
			return nil
		}
		return fn(v.Rel(abs))
	})
}

// freeName is rel, or rel with a number, so that no file and no other move holds it.
func freeName(v *vault.Vault, rel string, taken map[string]bool) string {
	ext := path.Ext(rel)
	base := strings.TrimSuffix(rel, ext)
	out := rel
	for i := 2; v.Exists(out) || taken[strings.ToLower(out)]; i++ {
		out = fmt.Sprintf("%s (%d)%s", base, i, ext)
	}
	taken[strings.ToLower(out)] = true
	return out
}
