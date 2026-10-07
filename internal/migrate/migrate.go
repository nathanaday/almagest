// Package migrate moves a vault of the layout before tool/ (layout 7) to the current
// layout in one commit: sessions/, source-core/, and trash/ go into tool/, and the paths
// that links and Bases name follow. Plan computes the move and writes nothing; Run writes
// it. Only the user runs it: the guard refuses the command to an agent.
package migrate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/almagest/internal/derive"
	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/lint"
	"github.com/nathanaday/almagest/internal/vault"
	"github.com/nathanaday/almagest/internal/views"
)

// Trailer marks the migration's commit.
const Trailer = "Almagest-Migrate"

// The folders that tool/ takes, as layout 7 named them.
const (
	oldSessions = "sessions"
	oldCore     = "source-core"
	oldTrash    = "trash"
)

var moved = map[string]string{oldSessions: vault.Sessions, oldCore: vault.Core, oldTrash: vault.Trash}

// bookmarks is Obsidian's list of bookmarked files, which names them by path.
const bookmarks = ".obsidian/bookmarks.json"

// Move is one file the migration moves.
type Move struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Report is what a migration does.
type Report struct {
	Vault string `json:"vault"`
	From  int    `json:"from"`
	To    int    `json:"to"`
	Moved []Move `json:"moved"`
	// Edited are the files whose links or Base filters named a moved path, by their path
	// after the moves.
	Edited   []string `json:"edited"`
	Warnings []string `json:"warnings"`
	Commit   string   `json:"commit,omitempty"`
	Problems int      `json:"problems"`
}

// plan is the migration in memory: the moves, then the edits at their paths after them.
type plan struct {
	moves []Move
	edits map[string][]byte
}

// check refuses a vault that the migration does not take.
func check(v *vault.Vault) error {
	switch l := v.LayoutVersion(); l {
	case vault.LayoutBeforeTool:
		return nil
	case vault.Layout:
		return errors.New("this vault keeps sessions/, source-core/, and trash/ in tool/ already; there is nothing to migrate")
	default:
		return fmt.Errorf("this vault has layout %d, and the migration takes layout %d to %d: %w", l, vault.LayoutBeforeTool, vault.Layout, vault.ErrLayout)
	}
}

// Plan computes the migration of a vault and writes nothing.
func Plan(v *vault.Vault) (*Report, error) {
	if err := check(v); err != nil {
		return nil, err
	}
	r := newReport(v)
	p, err := build(v, r)
	if err != nil {
		return nil, err
	}
	r.Moved = p.moves
	return r, nil
}

// Run migrates a vault in one commit, then writes the views.
func Run(v *vault.Vault, now time.Time) (_ *Report, err error) {
	// The write's start makes the folders of the current layout, so a vault the
	// migration does not take is refused before anything touches it.
	if err := check(v); err != nil {
		return nil, err
	}
	r := newReport(v)
	if mine := userFiles(v); len(mine) > 0 {
		r.Warnings = append(r.Warnings, fmt.Sprintf("%s/ held %d %s of yours before the migration; they stay there, beside what Almagest keeps", vault.Tool, len(mine), doc.Plural(len(mine), "file", "files")))
	}
	tx, err := vault.Begin(v, func() error { return vault.Recover(v) })
	if err != nil {
		return nil, err
	}
	defer tx.End(&err)
	p, err := build(v, r)
	if err != nil {
		return nil, err
	}
	for _, m := range p.moves {
		if err := tx.Move(m.From, m.To); err != nil {
			return nil, fmt.Errorf("migrate: %w", err)
		}
	}
	for _, rel := range sortedKeys(p.edits) {
		if err := tx.Write(rel, p.edits[rel]); err != nil {
			return nil, fmt.Errorf("migrate: %w", err)
		}
	}
	r.Moved = p.moves
	for dir := range moved {
		prune(v.Abs(dir))
	}
	fresh, err := vault.Open(v.Root)
	if err != nil {
		return nil, err
	}
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
	sha, err := tx.Commit(fmt.Sprintf("layout: move sessions/, source-core/, and trash/ into %s/", vault.Tool), fmt.Sprintf("%s: %d to %d", Trailer, r.From, r.To))
	if err != nil {
		return nil, err
	}
	r.Commit = sha
	if idx, err = vault.Load(fresh); err != nil {
		return r, err
	}
	views.Write(idx, now)
	fresh.SyncSettings(nil)
	if f, err := lint.Run(idx, lint.Options{Quick: true, Now: now}); err == nil {
		r.Problems = f.Counts[lint.Error]
	}
	return r, nil
}

func newReport(v *vault.Vault) *Report {
	r := &Report{Vault: v.Name(), From: v.LayoutVersion(), To: vault.Layout, Moved: []Move{}, Edited: []string{}, Warnings: []string{}}
	if applied, _ := filepath.Glob(v.Abs(vault.Changes + "/*/*.md")); slices.ContainsFunc(applied, func(f string) bool {
		data, _ := os.ReadFile(f)
		return strings.Contains(string(data), "\nstatus: applied\n")
	}) {
		r.Warnings = append(r.Warnings, "undo takes back no change applied before the migration, since its documents moved")
	}
	return r
}

// build computes the migration: every file of sessions/, source-core/, and trash/ moves
// into tool/; the links and Base filters that name a moved path follow; Obsidian's
// attachment folder, excluded files, and bookmarks follow; and Almagest.md takes the
// current layout.
func build(v *vault.Vault, r *Report) (*plan, error) {
	p := &plan{edits: map[string][]byte{}}
	for _, dir := range sortedKeys(moved) {
		err := walkFiles(v, dir, func(rel string) error {
			p.moves = append(p.moves, Move{From: rel, To: vault.Tool + "/" + rel})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(p.moves, func(i, j int) bool { return p.moves[i].From < p.moves[j].From })
	if err := claim(v, p.moves); err != nil {
		return nil, err
	}
	dest := map[string]string{}
	for _, m := range p.moves {
		dest[m.From] = m.To
	}
	edit := func(rel string, before, after []byte) {
		if string(before) == string(after) {
			return
		}
		at := rel
		if to, ok := dest[rel]; ok {
			at = to
		}
		p.edits[at] = after
		r.Edited = append(r.Edited, at)
	}
	err := filepath.WalkDir(v.Root, func(abs string, e fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel := v.Rel(abs)
		if e.IsDir() {
			// The views are written again; a captured original, the trash, and a file
			// that waits in ingest/ stay as they were.
			if abs != v.Root && (strings.HasPrefix(e.Name(), ".") || e.Name() == "node_modules" || slices.Contains([]string{vault.WikiView, vault.Ingest, oldTrash, vault.Trash, oldCore + "/originals", vault.Originals}, rel)) {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(path.Ext(rel))
		if ext != ".md" && ext != ".canvas" && ext != ".base" {
			return nil
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		after := rewriteRefs(ext, string(data))
		if vault.IsMarker(rel) {
			after = doc.SetField(after, "layout", vault.Layout)
		}
		edit(rel, data, []byte(after))
		return nil
	})
	if err != nil {
		return nil, err
	}
	if data, err := v.Read(bookmarks); err == nil {
		edit(bookmarks, data, []byte(rewritePaths(string(data), true)))
	}
	if data, err := v.Read(vault.AppJSON); err == nil {
		after, err := appSettings(data)
		if err != nil {
			return nil, err
		}
		edit(vault.AppJSON, data, after)
	}
	sort.Strings(r.Edited)
	return p, nil
}

// pathRef matches a folder that tool/ takes where a path stands as a path: right after
// [[, ![[, ](, or a quote, with its slash, or alone between quotes (a Base's inFolder).
var pathRef = regexp.MustCompile(`(\[\[|\]\(<?(?:\./)?|"|')(source-core|sessions|trash)(/|"|')`)

// rewritePaths points the path references of one text into tool/. With quoted false, only
// links count: a quoted path in a note's prose is a record of what was.
func rewritePaths(s string, quoted bool) string {
	return pathRef.ReplaceAllStringFunc(s, func(m string) string {
		parts := pathRef.FindStringSubmatch(m)
		open, dir, close := parts[1], parts[2], parts[3]
		link := strings.HasPrefix(open, "[[") || strings.HasPrefix(open, "](")
		switch {
		case !quoted && !link:
			return m
		case close != "/" && (link || close != open):
			return m
		}
		return open + moved[dir] + close
	})
}

// rewriteRefs points a file's path references into tool/. A canvas and a Base name paths
// in quotes; a note names them in links, and in quotes only inside a base block. A note's
// code, inline or fenced, stays as written.
func rewriteRefs(ext, s string) string {
	if ext != ".md" {
		return rewritePaths(s, true)
	}
	lines := strings.Split(s, "\n")
	// The open fence: its marks (``` or a longer run, or ~~~) and its info string. Only a
	// line of at least as many of the same marks closes it, so the writes of a change
	// document, fenced in five backticks, keep the blocks they hold.
	marks, info := "", ""
	for i, l := range lines {
		t := strings.TrimSpace(l)
		run := fenceRun(t)
		switch {
		case marks == "" && run != "":
			marks, info = run, strings.TrimSpace(t[len(run):])
			continue
		case marks != "" && run != "" && run[0] == marks[0] && len(run) >= len(marks) && strings.TrimSpace(t[len(run):]) == "":
			marks, info = "", ""
			continue
		case marks != "":
			if info == "base" {
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

// fenceRun is the run of three or more backticks or tildes that opens t, or "".
func fenceRun(t string) string {
	if !strings.HasPrefix(t, "```") && !strings.HasPrefix(t, "~~~") {
		return ""
	}
	n := len(t) - len(strings.TrimLeft(t, t[:1]))
	return t[:n]
}

// appSettings points Obsidian's attachment folder and excluded files into tool/, and
// keeps every other key and the user's own choices.
func appSettings(data []byte) ([]byte, error) {
	settings := map[string]any{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, errors.New(vault.AppJSON + " is not valid JSON; fix it, then migrate")
	}
	changed := false
	if a, _ := settings["attachmentFolderPath"].(string); strings.TrimSuffix(a, "/") == oldCore+"/originals" {
		settings["attachmentFolderPath"] = vault.Originals
		changed = true
	}
	if list, ok := settings["userIgnoreFilters"].([]any); ok {
		for i, x := range list {
			if s, _ := x.(string); strings.TrimSuffix(s, "/") == oldTrash {
				list[i] = vault.Trash + "/"
				changed = true
			}
		}
	}
	if !changed {
		return data, nil
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// claim refuses moves whose targets exist.
func claim(v *vault.Vault, moves []Move) error {
	for _, m := range moves {
		if v.Exists(m.To) {
			return fmt.Errorf("%s would move to %s, which exists; move that file away, then migrate", m.From, m.To)
		}
	}
	return nil
}

// userFiles are the files in tool/ before the migration: the user's, since layout 7 has
// no tool/.
func userFiles(v *vault.Vault) []string {
	var out []string
	walkFiles(v, vault.Tool, func(rel string) error {
		out = append(out, rel)
		return nil
	})
	return out
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

// prune removes the folders under root that the moves left empty, and root itself when
// it holds nothing but .DS_Store files.
func prune(root string) {
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

func sortedKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// beforeCommit is a test hook that runs right before the migration's commit.
var beforeCommit func()
