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

// Recover finds each change a crash left applying, puts back every path it may have
// written, and sets it to proposed. It skips a path that is no local document: the field
// is frontmatter, which a pull or a shell can write. The caller holds the lock.
func Recover(v *Vault) error {
	files, _ := filepath.Glob(v.Abs(Changes + "/*/*.md"))
	g := v.Git()
	for _, abs := range files {
		data, err := os.ReadFile(abs)
		if err != nil || !strings.Contains(string(data), "status: "+Applying) {
			continue
		}
		d := doc.Parse(v.Rel(abs), data)
		if d.Str("status") != Applying {
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
