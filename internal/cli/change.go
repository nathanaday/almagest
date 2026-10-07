package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/nathanaday/almagest/internal/change"
	"github.com/nathanaday/almagest/internal/vault"
)

// changeCmd is almagest change: propose, show, apply, reject, undo, and the work documents.
func (c *CLI) changeCmd(argv []string) error {
	a := parse(argv)
	v, err := c.open(a)
	if err != nil {
		return err
	}
	now := c.Now()
	var pv *change.Preview
	switch a.arg(0) {
	case "propose":
		var plan change.Plan
		if err := c.readJSON(a.arg(1), &plan); err != nil {
			return fmt.Errorf("the plan: %w", err)
		}
		if id := a.get("id"); id != "" {
			plan.ID = id
		}
		pv, err = change.Propose(v, plan, now)
	case "start":
		pv, err = change.Start(v, change.StartIn{Title: a.get("title"), Kind: a.get("kind"), Files: a.list("file")}, now)
	case "progress":
		pv, err = change.Progress(v, a.arg(1), strings.Join(a.pos[min(2, len(a.pos)):], " "), now)
	case "show", "":
		idx, lerr := vault.Load(v)
		if lerr != nil {
			return lerr
		}
		pv, err = change.Show(idx, a.arg(1))
	case "apply":
		pv, err = change.Apply(v, a.arg(1), now, nil)
	case "reject":
		pv, err = change.Reject(v, a.arg(1), a.get("reason"), now)
	case "undo":
		pv, err = change.Undo(v, a.arg(1), now)
	default:
		return fmt.Errorf("change takes propose, start, progress, show, apply, reject, or undo, not %q", a.arg(0))
	}
	if err != nil {
		return err
	}
	if a.arg(0) != "show" && a.arg(0) != "" {
		c.views(v, now)
	}
	return c.emit(a, pv, func(w io.Writer) { printPreview(w, pv) })
}

func printPreview(w io.Writer, pv *change.Preview) {
	fmt.Fprintf(w, "%s · %s · %s\n", pv.Ref.Title, pv.Status, pv.Counts.String())
	fmt.Fprintf(w, "  %s\n", pv.Ref.Path)
	for _, wl := range pv.Writes {
		line := fmt.Sprintf("  %-7s %s", wl.Op, wl.Title)
		if wl.Kind != "" {
			line += " (" + wl.Kind + ")"
		}
		if wl.Lines != "" {
			line += "  " + wl.Lines
		}
		if wl.Note != "" {
			line += "  · " + wl.Note
		}
		fmt.Fprintln(w, line)
	}
	for _, r := range pv.Rewrites {
		fmt.Fprintf(w, "  rewrite %s\n", r.Title)
	}
	if len(pv.NewTags) > 0 {
		fmt.Fprintf(w, "  new tags: %s\n", strings.Join(pv.NewTags, ", "))
	}
	for _, warn := range pv.Warnings {
		fmt.Fprintf(w, "  warning: %s\n", warn)
	}
	if pv.Reason != "" {
		fmt.Fprintf(w, "  reason: %s\n", pv.Reason)
	}
	if pv.Commit != "" {
		fmt.Fprintf(w, "  commit %s\n", short(pv.Commit))
	}
}
