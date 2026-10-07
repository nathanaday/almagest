package cli

import (
	"fmt"
	"io"

	"github.com/nathanaday/almagest/internal/journal"
	"github.com/nathanaday/almagest/internal/vault"
)

// journalCmd is almagest journal: the volumes, and the user's publish.
func (c *CLI) journalCmd(argv []string) error {
	a := parse(argv)
	v, err := c.open(a)
	if err != nil {
		return err
	}
	switch a.arg(0) {
	case "", "list":
		idx, err := vault.Load(v)
		if err != nil {
			return err
		}
		vols := journal.Volumes(idx)
		return c.emit(a, map[string]any{"journals": vols}, func(w io.Writer) {
			if len(vols) == 0 {
				fmt.Fprintf(w, "No journal yet: a volume is a folder directly under %s/.\n", vault.Journals)
			}
			for _, vol := range vols {
				line := fmt.Sprintf("%s (%s) · %s", vol.Name, vol.Volume, count(vol.Notes, "note", "notes"))
				if vol.Edition != "" {
					line += " · latest: " + vol.Edition
				}
				if vol.Changed {
					line += " · changed since"
				}
				fmt.Fprintln(w, line)
			}
		})
	case "publish":
		now := c.Now()
		p, err := journal.Publish(v, a.arg(1), now)
		if err != nil {
			return err
		}
		c.views(v, now)
		return c.emit(a, map[string]any{"published": p}, func(w io.Writer) {
			fmt.Fprintf(w, "Published %s (%s), %s. It waits for the wiki as a pending source.\n", p.Source.Title, p.Source.ID, short(p.Commit))
		})
	}
	return fmt.Errorf("journal takes list or publish, not %q", a.arg(0))
}
