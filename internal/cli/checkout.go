package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/nathanaday/almagest/internal/checkout"
	"github.com/nathanaday/almagest/internal/vault"
)

// checkoutCmd is almagest checkout: the candidates, make, return, and the list.
func (c *CLI) checkoutCmd(argv []string) error {
	a := parse(argv)
	v, err := c.open(a)
	if err != nil {
		return err
	}
	now := c.Now()
	switch a.arg(0) {
	case "", "list":
		list := checkout.List(v)
		return c.emit(a, map[string]any{"checkouts": list}, func(w io.Writer) {
			if len(list) == 0 {
				fmt.Fprintln(w, "No checkout yet.")
			}
			for _, e := range list {
				fmt.Fprintf(w, "%s · %s · %s · %s · %d edited\n", e.Folder, e.Status, e.Request, count(e.Documents, "document", "documents"), e.Edited)
			}
		})
	case "candidates":
		idx, err := vault.Load(v)
		if err != nil {
			return err
		}
		limit, _ := strconv.Atoi(a.get("limit"))
		cands, err := checkout.Candidates(idx, strings.Join(a.pos[1:], " "), a.list("tag"), a.list("type"), limit)
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"candidates": cands}, func(w io.Writer) {
			for _, cd := range cands {
				via := ""
				if cd.Via != "" {
					via = " · via " + cd.Via
				}
				fmt.Fprintf(w, "%6.2f  %d  %s (%s)%s\n", cd.Score, cd.Distance, cd.Ref.Title, cd.Ref.ID, via)
			}
		})
	case "make":
		var o checkout.Order
		if err := c.readJSON(a.arg(1), &o); err != nil {
			return fmt.Errorf("the order: %w", err)
		}
		m, err := checkout.Make(v, o, now)
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"made": m}, func(w io.Writer) {
			fmt.Fprintf(w, "Checked out %s into %s; its index is %s.\n", count(len(m.Copies), "document", "documents"), m.Folder, m.Index)
		})
	case "return":
		r, err := checkout.Return(v, a.arg(1), now)
		if err != nil {
			return err
		}
		c.views(v, now)
		return c.emit(a, map[string]any{"returned": r}, func(w io.Writer) {
			fmt.Fprintf(w, "Returned the checkout to %s.\n", r.Folder)
			if r.Change != nil {
				printPreview(w, r.Change)
			} else {
				fmt.Fprintln(w, "No copy was edited, so it proposes no change.")
			}
			for _, s := range r.Skipped {
				fmt.Fprintf(w, "  left out: %s\n", s)
			}
			if r.Warning != "" {
				fmt.Fprintf(w, "warning: %s\n", r.Warning)
			}
		})
	}
	return fmt.Errorf("checkout takes list, candidates, make, or return, not %q", a.arg(0))
}
