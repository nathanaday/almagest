package cli

import (
	"fmt"
	"io"
	"strconv"

	"github.com/nathanaday/almagest/internal/source"
)

// sourceCmd is almagest source: capture, and the chunks and pages of a source.
func (c *CLI) sourceCmd(argv []string) error {
	a := parse(argv, "new-tags")
	switch a.arg(0) {
	case "capture":
		v, err := c.open(a)
		if err != nil {
			return err
		}
		req := source.Request{Ingest: a.list("ingest"), Title: a.get("title"), Repository: a.get("repository"), Tags: a.list("tag"), Locator: a.get("locator"), NewTags: a.has("new-tags")}
		if a.has("text") {
			text, err := c.readText(a.get("text"))
			if err != nil {
				return err
			}
			req.Text = text
		}
		res, err := source.Capture(v, req, c.Now())
		if err != nil {
			return err
		}
		c.views(v, c.Now())
		return c.emit(a, res, func(w io.Writer) {
			for _, cp := range res.Captured {
				if cp.Duplicate != "" {
					fmt.Fprintf(w, "already captured: %s (%s)\n", cp.Ref.Title, cp.Duplicate)
					continue
				}
				fmt.Fprintf(w, "captured: %s (%s) · %s · %d chunks\n", cp.Ref.Title, cp.Ref.ID, cp.Measure, len(cp.Chunks))
			}
		})
	case "chunks":
		idx, err := c.index(a)
		if err != nil {
			return err
		}
		chunks, err := source.Chunks(idx, a.arg(1))
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"chunks": chunks}, func(w io.Writer) {
			for _, ch := range chunks {
				fmt.Fprintf(w, "%d/%d  %s\n", ch.Index, ch.Count, ch.Locator)
			}
		})
	case "read":
		idx, err := c.index(a)
		if err != nil {
			return err
		}
		n, _ := strconv.Atoi(a.arg(2))
		blob, err := source.Read(idx, a.arg(1), n)
		if err != nil {
			return err
		}
		return c.emit(a, map[string]any{"blob": blob}, func(w io.Writer) {
			if blob.File != "" {
				fmt.Fprintf(w, "Read %s pages %s\n", blob.File, blob.Pages)
				return
			}
			fmt.Fprintln(w, blob.Content)
		})
	}
	return fmt.Errorf("source takes capture, chunks, or read, not %q", a.arg(0))
}
