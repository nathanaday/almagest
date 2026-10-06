// Package migrate moves a vault of the 8.x layout to the layout of 9.0 in one commit. The
// thread documents of wiki/documents and the chord canvases move to threads/, with the
// paths they name updated to the new places, and the documents that stay lose the fields
// and sections that served threads. A vault
// older than 8.0 migrates with 8.1.1 first. Plan computes the move and writes nothing;
// Run writes it.
package migrate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/derive"
	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/lint"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/sessions"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
	"github.com/nathanaday/atlas-obsidian/internal/views"
)

// Trailer marks the migration's commit.
const Trailer = "Atlas-Migrate: 9.0"

// Chords is the folder of the chord canvases in 8.x.
const Chords = "chords"

// Move is one file the migration moves as it is.
type Move struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Report is what a migration does.
type Report struct {
	Vault string `json:"vault"`
	// From is the layout the vault had.
	From string `json:"from"`
	// Moved are the thread documents and chord canvases, now in threads/.
	Moved []Move `json:"moved"`
	// Edited are the documents that lost a field or a section of the threads.
	Edited   []string `json:"edited"`
	Warnings []string `json:"warnings"`
	Commit   string   `json:"commit,omitempty"`
	Plugin   string   `json:"plugin,omitempty"`
	// Strays are the notes of the user's found in views/, which the views step moved to
	// inbox/.
	Strays   []vault.Moved `json:"strays,omitempty"`
	Problems int           `json:"problems"`
}

// edit is a file's content once the thread fields and sections leave it, or once the
// paths it names follow the moves.
type edit struct {
	path    string
	content string
}

type plan struct {
	report *Report
	moves  []Move
	// rewrites are the moved files whose content names a moved path; each path is the
	// file's place after its move.
	rewrites []edit
	edits    []edit
}

// Fields that served threads, by type.
var dropped = map[string][]string{
	"vault":   {"wikify"},
	"source":  {"from"},
	"topic":   {"from"},
	"session": {"threads", "specs", "work", "checked", "events"},
	"change":  {"work"},
}

// Plan computes the migration of a vault and writes nothing.
func Plan(v *vault.Vault) (*Report, error) {
	if fresh, err := vault.Open(v.Root); err == nil {
		v = fresh
	}
	p, err := build(v)
	if err != nil {
		return nil, err
	}
	return p.report, nil
}

func build(v *vault.Vault) (*plan, error) {
	switch layout := v.LayoutVersion(); {
	case layout >= vault.Layout:
		return nil, errors.New("this vault has the 9.0 layout already; there is nothing to migrate")
	case layout < vault.LayoutThreads:
		return nil, errors.New("this vault has the layout of 7.x or earlier; migrate it with Atlas 8.1.1 first (the tag threads-final of atlas-obsidian), then with this release")
	}
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	var waiting []string
	for _, c := range idx.Of("change") {
		if s := c.Str("status"); s == "proposed" || s == "applying" {
			waiting = append(waiting, vault.Title(c))
		}
	}
	if len(waiting) > 0 {
		return nil, fmt.Errorf("apply or reject these changes first, since the migration rewrites the documents they read: %s", strings.Join(waiting, ", "))
	}
	p := &plan{report: &Report{Vault: v.Name(), From: "8.x", Moved: []Move{}, Edited: []string{}, Warnings: []string{}}}
	for _, d := range idx.Notes {
		if path.Dir(d.Path) == vault.Documents && slices.Contains(schema.ArchivedTypes, d.Type()) {
			p.moves = append(p.moves, Move{From: d.Path, To: vault.Threads + "/" + path.Base(d.Path)})
		}
	}
	var left []string
	err = filepath.WalkDir(v.Abs(Chords), func(abs string, e fs.DirEntry, err error) error {
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
			p.moves = append(p.moves, Move{From: rel, To: vault.Threads + strings.TrimPrefix(rel, Chords)})
		default:
			left = append(left, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(left) > 0 {
		p.report.Warnings = append(p.report.Warnings, fmt.Sprintf("%s/ keeps the files that are no canvas, and the folder stays: %s", Chords, strings.Join(left, ", ")))
	}
	sort.Slice(p.moves, func(i, j int) bool { return p.moves[i].From < p.moves[j].From })
	to := map[string]string{}
	for _, m := range p.moves {
		key := strings.ToLower(m.To)
		if other, ok := to[key]; ok {
			return nil, fmt.Errorf("%s and %s would both move to %s; rename one, then migrate", other, m.From, m.To)
		}
		to[key] = m.From
		if v.Exists(m.To) {
			return nil, fmt.Errorf("%s would move to %s, which exists; move that file away, then migrate", m.From, m.To)
		}
	}
	p.report.Moved = p.moves
	// The cards of a chord's canvas name their stubs by path, and a chord names its
	// canvas by path, so both follow the moves.
	paths := strings.NewReplacer(pathPairs(p.moves)...)
	for _, m := range p.moves {
		data, err := v.Read(m.From)
		if err != nil {
			return nil, err
		}
		if content := paths.Replace(string(data)); content != string(data) {
			p.rewrites = append(p.rewrites, edit{m.To, content})
		}
	}
	for _, d := range idx.Docs {
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
			content = doc.SetField(content, "layout", vault.Layout)
		case "session":
			content = doc.ReplaceLead(content, sessions.Lead(doc.Parse(d.Path, []byte(content))))
		}
		if content != d.Content {
			p.edits = append(p.edits, edit{d.Path, content})
			p.report.Edited = append(p.report.Edited, d.Path)
		}
	}
	if v.Doc.Front.Has("wikify") && !slices.Contains(v.Doc.List("wikify"), "source") {
		p.report.Warnings = append(p.report.Warnings, "wikify left sources out, so no source was pending; 9.0 has no wikify, and every source the wiki has not absorbed is pending")
	}
	for _, s := range idx.Of("session") {
		if st := s.Str("status"); st == "running" || st == "waiting" {
			p.report.Warnings = append(p.report.Warnings, fmt.Sprintf("the session %s is %s; an agent of 8.x that runs in it still calls the thread tool, which 9.0 does not have", s.Title(), st))
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

// Run migrates a vault in one commit, then writes the views.
func Run(v *vault.Vault, now time.Time) (_ *Report, err error) {
	tx, err := vault.BeginAsIs(v, func() error { return vault.Recover(v) })
	if err != nil {
		return nil, err
	}
	defer tx.End(&err)
	fresh, err := vault.Open(v.Root)
	if err != nil {
		return nil, err
	}
	p, err := build(fresh)
	if err != nil {
		return nil, err
	}
	for _, m := range p.moves {
		if err := tx.Move(m.From, m.To); err != nil {
			return nil, fmt.Errorf("migrate: %w", err)
		}
	}
	for _, e := range append(p.rewrites, p.edits...) {
		if err := tx.Write(e.path, []byte(e.content)); err != nil {
			return nil, fmt.Errorf("migrate: %w", err)
		}
	}
	if err := vault.UpgradeBases(fresh, tx.Write); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	pruneChords(fresh.Abs(Chords))
	if fresh, err = vault.Open(v.Root); err != nil {
		return nil, err
	}
	// The files the next steps write are kept too, so a failed commit puts them back.
	machine := []string{vault.AppJSON}
	for _, f := range vault.PluginFiles {
		machine = append(machine, path.Join(vault.PluginDir, f))
	}
	if err := tx.Keep(machine...); err != nil {
		return nil, err
	}
	// An older plugin cannot read the new layout, so the vault gets the one this binary carries.
	if fresh.InstalledPluginVersion() != "" {
		wrote, err := vault.InstallPlugin(fresh)
		if err != nil {
			return nil, err
		}
		if len(wrote) > 0 {
			p.report.Plugin = vault.PluginVersion()
		}
	}
	tx.Settle(machine...)
	idx, err := vault.Load(fresh)
	if err != nil {
		return nil, err
	}
	if _, err := derive.Sync(idx, vault.NewGuard(idx, tx).Write); err != nil {
		return nil, err
	}
	if beforeCommit != nil {
		beforeCommit()
	}
	sha, err := tx.CommitAll("layout: migrate to 9.0\n\n" + Trailer + " from " + p.report.From)
	if err != nil {
		return nil, err
	}
	p.report.Commit = sha
	if idx, err = vault.Load(fresh); err != nil {
		return p.report, err
	}
	if _, strays, err := views.Write(idx, now); err == nil {
		p.report.Strays = strays
	}
	fresh.SyncSettings(nil)
	if f, err := lint.Run(idx, lint.Options{Quick: true, Now: now}); err == nil {
		p.report.Problems = f.Counts[lint.Error]
	}
	return p.report, nil
}

// pruneChords removes the folders of chords/ that the moves left empty, and chords/ itself
// when it holds nothing but .DS_Store files.
func pruneChords(root string) {
	var dirs []string
	empty := true
	filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
		switch {
		case err != nil:
		case e.IsDir():
			dirs = append(dirs, p)
		case e.Name() != ".DS_Store":
			empty = false
		}
		return nil
	})
	if empty {
		os.RemoveAll(root)
		return
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		os.Remove(dirs[i])
	}
}

// beforeCommit is a test hook that runs right before the migration's commit.
var beforeCommit func()
