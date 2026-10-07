package cli

import (
	"fmt"
	"io"

	"github.com/nathanaday/almagest/internal/vault"
	"github.com/nathanaday/almagest/internal/wikify"
)

// wikifyCmd is almagest wikify: start a copy, and mark it.
func (c *CLI) wikifyCmd(argv []string) error {
	a := parse(argv)
	v, err := c.open(a)
	if err != nil {
		return err
	}
	switch a.arg(0) {
	case "start":
		cp, err := wikify.Start(v, a.arg(1))
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"copy": cp}, func(w io.Writer) { fmt.Fprintf(w, "Copied to %s.\n", cp) })
	case "mark":
		var marks []wikify.Mark
		if err := c.readJSON(a.arg(2), &marks); err != nil {
			return fmt.Errorf("the marks: %w", err)
		}
		idx, err := vault.Load(v)
		if err != nil {
			return err
		}
		m, err := wikify.Place(idx, a.arg(1), marks)
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"marked": m}, func(w io.Writer) {
			fmt.Fprintf(w, "Marked %s in %s.\n", count(len(m.Placed), "phrase", "phrases"), m.Note)
			for _, p := range m.Missing {
				fmt.Fprintf(w, "  not found: %s\n", p)
			}
		})
	}
	return fmt.Errorf("wikify takes start or mark, not %q", a.arg(0))
}
