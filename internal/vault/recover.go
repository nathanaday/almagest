package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/gitx"
)

// Applying is the status of a change document while its apply writes.
const Applying = "applying"

// ChangeTrailer is the trailer of an apply's commit, which names the change's id.
const ChangeTrailer = "Atlas-Change"

// Recover finds each change a crash left in flight: a change document that still holds
// paths. When the apply's commit exists, the apply landed and only the document's last
// write was lost, so it takes the document from HEAD. Otherwise it puts back every path
// the apply may have written, and sets the change to proposed. It skips a path that is no
// local document: the field is frontmatter, which a pull or a shell can write. The caller
// holds the lock.
func Recover(v *Vault) error {
	files, _ := filepath.Glob(v.Abs(Changes + "/*/*.md"))
	g := v.Git()
	for _, abs := range files {
		data, err := os.ReadFile(abs)
		if err != nil || !strings.Contains(string(data), "\npaths:") {
			continue
		}
		d := doc.Parse(v.Rel(abs), data)
		if !d.Front.Has("paths") || d.ID() == "" {
			continue
		}
		if sha, err := g.FindTrailer(ChangeTrailer, d.ID()); err == nil && sha != "" && g.Has(sha+":"+d.Path) {
			// The apply landed: its commit holds the document as applied. The file differs
			// from it by paths alone, unless someone edited it after the crash; such an
			// edit goes into git first. The document comes from the apply's own commit, so
			// a recovery that a crash stops here does the same again.
			landed, _ := g.ShowFile(sha, d.Path)
			if doc.RemoveField(d.Content, "paths") != string(landed) {
				if _, err := commitFound(v, d, []string{d.Path}); err != nil {
					return fmt.Errorf("recover %s: %w", Title(d), err)
				}
			}
			if err := g.RestoreFrom(sha, d.Path); err != nil {
				return fmt.Errorf("recover %s: %w", Title(d), err)
			}
			if err := g.Unstage(d.Path); err != nil {
				return fmt.Errorf("recover %s: %w", Title(d), err)
			}
			continue
		}
		var local []string
		for _, p := range d.List("paths") {
			if v.Local(p) || chordCanvas(v, p) {
				local = append(local, p)
			}
		}
		// The revision to put the paths back from is HEAD as the crash left it. It goes
		// into the document before anything else, so a recovery that a crash stops
		// partway restores from the same revision, not from its own recovery commit.
		base := d.Str("recovering")
		if base == "" {
			base = recoveringNone
			if head, err := g.Head(); err == nil && head != "" {
				base = head
			}
			d.Content = doc.SetField(d.Content, "recovering", base)
			if err := v.Write(d.Path, []byte(d.Content)); err != nil {
				return fmt.Errorf("recover %s: %w", Title(d), err)
			}
		} else if !validBase(g, base) {
			// The field is frontmatter, which a hand or a pull can change. Recovery wrote
			// a full commit id from the history; anything else, such as HEAD or a short
			// id, could put back nothing, or delete what it lacks.
			return fmt.Errorf("recover %s: its recovering field %q is not the full id of a commit in the vault's history; set it to the full id of the commit before the apply (git log shows it), then try again. Removing the field recovers from HEAD, which puts nothing back once a recovery commit has landed", d.Path, base)
		}
		from := base
		if from == recoveringNone {
			from = ""
		}
		// What the crash left, and any edit made since, goes into git before the paths
		// go back, so no text is lost.
		if _, err := commitFound(v, d, local); err != nil {
			return fmt.Errorf("recover %s: %w", Title(d), err)
		}
		for _, p := range local {
			if err := g.RestoreFrom(from, p); err != nil {
				return fmt.Errorf("recover %s: %w", Title(d), err)
			}
			v.Prune(p)
		}
		// The checkout staged the reversal, and a crash inside a commit leaves the change
		// document staged as applied; both wait in the work tree for the next write's
		// snapshot instead, so the user's own next commit takes neither.
		if err := g.Unstage(append(local, d.Path)...); err != nil {
			return fmt.Errorf("recover %s: %w", Title(d), err)
		}
		content := doc.RemoveField(doc.RemoveField(doc.SetField(d.Content, "status", "proposed"), "paths"), "recovering")
		content = doc.ReplaceLead(content, doc.Callout("change", "Proposed", "Recovered after a crash. Review the documents below, then say yes in the chat, or press Apply."))
		if err := v.Write(d.Path, []byte(content)); err != nil {
			return err
		}
	}
	return nil
}

// BeginWrite starts a write that ends in one commit: it refuses a vault of an older
// layout, takes the lock, recovers a change a crash left applying, and commits a dirty
// tree as a snapshot. Every write tool starts with it.
func BeginWrite(v *Vault) (*Tx, error) {
	if err := v.CheckLayout(); err != nil {
		return nil, err
	}
	return Begin(v, func() error { return Recover(v) })
}

// recoveringNone records a recovery in a repository that had no commit before the crash.
const recoveringNone = "none"

// RecoveredTrailer is the trailer of the commit that keeps what a crash left, which names
// the change's id.
const RecoveredTrailer = "Atlas-Recovered"

// commitFound commits each path whose bytes differ from HEAD, as found, and returns the
// revision to put the paths back from: the commit before that one, or HEAD when nothing
// differed.
func commitFound(v *Vault, d *doc.Doc, paths []string) (string, error) {
	g := v.Git()
	if !g.HasHead() {
		return "", nil
	}
	var differ []string
	for _, p := range paths {
		disk, diskErr := os.ReadFile(v.Abs(p))
		head, headErr := g.ShowFile("HEAD", p)
		switch {
		case diskErr != nil && headErr != nil:
		case diskErr != nil || headErr != nil || string(disk) != string(head):
			differ = append(differ, p)
		}
	}
	if len(differ) == 0 {
		return "HEAD", nil
	}
	before, err := g.Head()
	if err != nil {
		return "", err
	}
	if err := g.Add(differ...); err != nil {
		return "", err
	}
	noun := "files"
	if len(differ) == 1 {
		noun = "file"
	}
	// Only these paths: a crash inside a commit can leave other entries staged, such as
	// the change document as applied, which belong to no recovery commit.
	if _, err := g.CommitOnly(fmt.Sprintf("recovery: %d %s as found after a crash\n\n%s: %s", len(differ), noun, RecoveredTrailer, d.ID()), differ...); err != nil {
		return "", err
	}
	return before, nil
}

// chordCanvas reports whether a listed path is a chord's canvas, which an apply's derived
// sync writes and recovery puts back with the documents.
func chordCanvas(v *Vault, p string) bool {
	return strings.HasPrefix(p, Chords+"/") && strings.HasSuffix(p, ".canvas") && !strings.Contains(strings.TrimPrefix(p, Chords+"/"), "/") && v.Contain(p) == nil
}

// validBase reports whether a recovering value can be the base recovery wrote: none only
// in a repository with no commit, else the full id of a commit in the history.
func validBase(g gitx.Repo, base string) bool {
	if base == recoveringNone {
		head, err := g.Head()
		return err != nil || head == ""
	}
	return g.InHistory(base)
}
