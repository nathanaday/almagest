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
			if err := g.RestoreFrom("HEAD", d.Path); err != nil {
				return fmt.Errorf("recover %s: %w", Title(d), err)
			}
			continue
		}
		for _, p := range d.List("paths") {
			if !v.Local(p) {
				continue
			}
			if err := g.RestoreFrom("HEAD", p); err != nil {
				return fmt.Errorf("recover %s: %w", Title(d), err)
			}
			v.Prune(p)
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
