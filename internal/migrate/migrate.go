// Package migrate moves a vault of an earlier layout to the layout of 10.0 in one commit,
// in up to two steps. The step from 8.x (v9.go) moves the thread documents and the chord
// canvases to threads/, and takes the fields and sections that served threads out of the
// documents that stay. The step from 9.0 (v10.go) renames the folders: wiki/ becomes
// source-core/, inbox/ becomes ingest/, and views/ becomes wiki-view/, and the paths that
// links and Bases name follow. An 8.x vault takes both steps. A vault older than 8.0
// migrates with 8.1.1 first. Plan computes the move and writes nothing; Run writes it.
package migrate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/derive"
	"github.com/nathanaday/atlas-obsidian/internal/lint"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
	"github.com/nathanaday/atlas-obsidian/internal/views"
)

// Trailer marks the migration's commit.
const Trailer = "Atlas-Migrate: 10.0"

// Move is one file the migration moves as it is.
type Move struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Report is what a migration does.
type Report struct {
	Vault string `json:"vault"`
	// From is the layout the vault had: 8.x or 9.0.
	From string `json:"from"`
	// Moved are the files the migration moved: the thread documents and chord canvases
	// of 8.x to threads/, and every file of the renamed folders.
	Moved []Move `json:"moved"`
	// Edited are the files whose text the migration changed: a field or a section of the
	// threads left, or a path the file names followed a move. Each is named by its path
	// after the moves.
	Edited []string `json:"edited"`
	// Removed are the notes of the old views/, which code writes again in wiki-view/.
	Removed  []string `json:"removed"`
	Warnings []string `json:"warnings"`
	Commit   string   `json:"commit,omitempty"`
	Plugin   string   `json:"plugin,omitempty"`
	// Strays are the notes of the user's that the migration or its views step found in
	// a views folder and moved to ingest/.
	Strays   []vault.Moved `json:"strays,omitempty"`
	Problems int           `json:"problems"`
}

// edit is a file's content after the migration, at its path after the moves.
type edit struct {
	path    string
	content string
}

// plan is one step of the migration, in memory.
type plan struct {
	moves   []Move
	edits   []edit
	removes []string
}

func newReport(v *vault.Vault) *Report {
	return &Report{Vault: v.Name(), Moved: []Move{}, Edited: []string{}, Removed: []string{}, Warnings: []string{}}
}

// check refuses a vault the migration does not take: one of the current layout, one
// older than 8.0, and one with a change waiting for an answer.
func check(v *vault.Vault) error {
	switch layout := v.LayoutVersion(); {
	case layout >= vault.Layout:
		return errors.New("this vault has the 10.0 layout already; there is nothing to migrate")
	case layout < vault.LayoutThreads:
		return errors.New("this vault has the layout of 7.x or earlier; migrate it with Atlas 8.1.1 first (the tag threads-final of atlas-obsidian), then with this release")
	}
	var waiting []string
	for _, c := range changes(v) {
		if s := c.Str("status"); s == "proposed" || s == "applying" {
			waiting = append(waiting, vault.Title(c))
		}
	}
	if len(waiting) > 0 {
		return fmt.Errorf("apply or reject these changes first, since the migration rewrites the documents they read: %s", strings.Join(waiting, ", "))
	}
	return nil
}

// Plan computes the migration of a vault and writes nothing. For an 8.x vault it lists
// the first step only, since the second reads what the first moves; the report says so.
func Plan(v *vault.Vault) (*Report, error) {
	if fresh, err := vault.Open(v.Root); err == nil {
		v = fresh
	}
	if err := check(v); err != nil {
		return nil, err
	}
	report := newReport(v)
	if v.LayoutVersion() == vault.LayoutThreads {
		report.From = "8.x"
		if _, err := build9(v, report); err != nil {
			return nil, err
		}
		report.Warnings = append(report.Warnings, "this lists the first step, from 8.x to 9.0; the same run then renames the folders of 10.0 (wiki/ to source-core/, inbox/ to ingest/, views/ to wiki-view/)")
		return report, nil
	}
	report.From = "9.0"
	if _, err := build10(v, report); err != nil {
		return nil, err
	}
	return report, nil
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
	if err := check(fresh); err != nil {
		return nil, err
	}
	report := newReport(fresh)
	report.From = "9.0"
	if fresh.LayoutVersion() == vault.LayoutThreads {
		report.From = "8.x"
		p, err := build9(fresh, report)
		if err != nil {
			return nil, err
		}
		if err := p.execute(tx); err != nil {
			return nil, err
		}
		prune(fresh.Abs(chords))
		if fresh, err = vault.Open(v.Root); err != nil {
			return nil, err
		}
	}
	p, err := build10(fresh, report)
	if err != nil {
		return nil, err
	}
	if err := p.execute(tx); err != nil {
		return nil, err
	}
	for _, dir := range []string{legacyWiki, legacyInbox, legacyViews} {
		prune(fresh.Abs(dir))
	}
	if fresh, err = vault.Open(v.Root); err != nil {
		return nil, err
	}
	if err := fresh.Git().Unexclude("/" + legacyViews + "/"); err != nil {
		return nil, err
	}
	if err := fresh.EnsureFolders(); err != nil {
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
	if _, err := vault.ObsidianSettings(fresh); err != nil {
		return nil, err
	}
	// An older plugin cannot read the new layout, so the vault gets the one this binary carries.
	if fresh.InstalledPluginVersion() != "" {
		wrote, err := vault.InstallPlugin(fresh)
		if err != nil {
			return nil, err
		}
		if len(wrote) > 0 {
			report.Plugin = vault.PluginVersion()
		}
	}
	tx.Settle(machine...)
	if err := vault.UpgradeBases(fresh, tx.Write); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
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
	sha, err := tx.CommitAll("layout: migrate to 10.0\n\n" + Trailer + " from " + report.From)
	if err != nil {
		return nil, err
	}
	report.Commit = sha
	if idx, err = vault.Load(fresh); err != nil {
		return report, err
	}
	if _, strays, err := views.Write(idx, now); err == nil {
		report.Strays = append(report.Strays, strays...)
	}
	fresh.SyncSettings(nil)
	if f, err := lint.Run(idx, lint.Options{Quick: true, Now: now}); err == nil {
		report.Problems = f.Counts[lint.Error]
	}
	return report, nil
}

// execute writes one step: every move, then every edit at its path after the moves, then
// every remove. A path that is taken refuses the step before anything moves.
func (p *plan) execute(tx *vault.Tx) error {
	for _, m := range p.moves {
		if err := tx.Move(m.From, m.To); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	for _, e := range p.edits {
		if err := tx.Write(e.path, []byte(e.content)); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	for _, r := range p.removes {
		if err := tx.Remove(r); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// claim refuses moves whose targets exist, or that share a target.
func claim(v *vault.Vault, moves []Move) error {
	to := map[string]string{}
	for _, m := range moves {
		key := strings.ToLower(m.To)
		if other, ok := to[key]; ok {
			return fmt.Errorf("%s and %s would both move to %s; rename one, then migrate", other, m.From, m.To)
		}
		to[key] = m.From
		if v.Exists(m.To) {
			return fmt.Errorf("%s would move to %s, which exists; move that file away, then migrate", m.From, m.To)
		}
	}
	return nil
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

// beforeCommit is a test hook that runs right before the migration's commit.
var beforeCommit func()
