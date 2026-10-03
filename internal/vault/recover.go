package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
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
		if sha, err := g.FindTrailer(ChangeTrailer, d.ID()); err == nil && sha != "" && g.Has("HEAD:"+d.Path) {
			// The apply landed. The file differs from its commit by paths alone, unless
			// someone edited it after the crash; such an edit goes into git first.
			from := "HEAD"
			head, _ := g.ShowFile("HEAD", d.Path)
			if doc.RemoveField(d.Content, "paths") != string(head) {
				if from, err = commitFound(v, d, []string{d.Path}); err != nil {
					return fmt.Errorf("recover %s: %w", Title(d), err)
				}
			}
			if err := g.RestoreFrom(from, d.Path); err != nil {
				return fmt.Errorf("recover %s: %w", Title(d), err)
			}
			if err := g.Unstage(d.Path); err != nil {
				return fmt.Errorf("recover %s: %w", Title(d), err)
			}
			continue
		}
		var local []string
		for _, p := range d.List("paths") {
			if v.Local(p) {
				local = append(local, p)
			}
		}
		// What the crash left, and any edit made since, goes into git before the paths
		// go back, so no text is lost. The paths then go back to the commit before it.
		from, err := commitFound(v, d, local)
		if err != nil {
			return fmt.Errorf("recover %s: %w", Title(d), err)
		}
		for _, p := range local {
			if err := g.RestoreFrom(from, p); err != nil {
				return fmt.Errorf("recover %s: %w", Title(d), err)
			}
			v.Prune(p)
		}
		// The checkout staged the reversal; it waits in the work tree for the next write's
		// snapshot instead, so the user's own next commit does not take it.
		if err := g.Unstage(local...); err != nil {
			return fmt.Errorf("recover %s: %w", Title(d), err)
		}
		content := doc.RemoveField(doc.SetField(d.Content, "status", "proposed"), "paths")
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
