package cli

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/nathanaday/almagest/internal/brief"
	"github.com/nathanaday/almagest/internal/lint"
	"github.com/nathanaday/almagest/internal/match"
	"github.com/nathanaday/almagest/internal/search"
)

// searchCmd is almagest search, the ranked search of the documents.
func (c *CLI) searchCmd(argv []string) error {
	a := parse(argv)
	idx, err := c.index(a)
	if err != nil {
		return err
	}
	q := search.Query{Text: strings.Join(a.pos, " "), Types: a.list("type"), Kinds: a.list("kind"), Tags: a.list("tag"), Status: a.list("status"), Repository: a.get("repository")}
	q.Limit, _ = strconv.Atoi(a.get("limit"))
	hits, err := search.Search(idx, q)
	if err != nil {
		return err
	}
	return c.emit(a, hits, func(w io.Writer) {
		for _, h := range hits.Hits {
			kind := h.Ref.Type
			if h.Ref.Kind != "" {
				kind += " " + h.Ref.Kind
			}
			fmt.Fprintf(w, "%6.2f  %-16s %s  (%s)\n", h.Score, kind, h.Ref.Title, h.Ref.ID)
			if h.Snippet != "" {
				fmt.Fprintf(w, "        %s\n", h.Snippet)
			}
		}
		fmt.Fprintf(w, "%d of %d\n", len(hits.Hits), hits.Total)
		if len(hits.Facets.Tags) > 0 {
			var list []string
			for t, n := range hits.Facets.Tags {
				list = append(list, fmt.Sprintf("%s %d", t, n))
			}
			sort.Strings(list)
			fmt.Fprintf(w, "with: %s\n", strings.Join(list, " · "))
		}
	})
}

// contextCmd is almagest context: what a task in a repository, a tag, or a path needs.
func (c *CLI) contextCmd(argv []string) error {
	a := parse(argv)
	idx, err := c.index(a)
	if err != nil {
		return err
	}
	b, err := brief.Of(idx, brief.Input{Repository: a.arg(0), Tags: a.list("tag"), Path: a.get("path")})
	if err != nil {
		return err
	}
	return c.emit(a, b, func(w io.Writer) {
		var tagList []string
		for _, t := range b.Tags {
			tagList = append(tagList, fmt.Sprintf("%s %d", t.Tag, t.Count))
		}
		fmt.Fprintf(w, "Tags: %s\n", strings.Join(tagList, " · "))
		for _, p := range b.Pages {
			fmt.Fprintf(w, "Page of %s: %s\n", p.Tag, p.Ref.Title)
		}
		var repos []string
		for _, r := range b.Repositories {
			repos = append(repos, r.Title)
		}
		if len(repos) > 0 {
			fmt.Fprintf(w, "Repositories: %s\n", strings.Join(repos, ", "))
		}
		for _, p := range b.Policies {
			fmt.Fprintf(w, "Policy (%s): %s · %s\n", orDash(p.Strength), p.Ref.Title, strings.Join(p.Via, ", "))
		}
		for _, in := range b.Instructions {
			fmt.Fprintf(w, "Instructions: %s\n", in.Path)
		}
		if r := b.Repository; r != nil {
			fmt.Fprintf(w, "Repository: %s · %s · head %s · %d dirty · %d ahead, %d behind the remote", r.Path, r.Branch, r.Head, len(r.Dirty), r.Ahead, r.BehindRemote)
			if r.Behind >= 0 {
				fmt.Fprintf(w, " · %d commits past its description", r.Behind)
			}
			fmt.Fprintln(w)
		}
	})
}

func orDash(s string) string {
	if s == "" {
		return "–"
	}
	return s
}

// matchCmd is almagest match: the documents that hold the subjects of a list.
func (c *CLI) matchCmd(argv []string) error {
	a := parse(argv, "across")
	idx, err := c.index(a)
	if err != nil {
		return err
	}
	in := match.Input{Docs: append(a.list("docs"), a.pos...), Tags: a.list("tag"), Across: a.has("across")}
	if file := a.get("items"); file != "" {
		if err := c.readJSON(file, &in.Items); err != nil {
			return fmt.Errorf("--items: %w", err)
		}
		in.Docs = nil
	}
	m, err := match.Run(idx, in)
	if err != nil {
		return err
	}
	return c.emit(a, m, nil)
}

// lintCmd is almagest lint, the health check.
func (c *CLI) lintCmd(argv []string) error {
	a := parse(argv)
	idx, err := c.index(a)
	if err != nil {
		return err
	}
	f, err := lint.Run(idx, lint.Options{Tags: append(a.list("tag"), a.pos...), Now: c.Now()})
	if err != nil {
		return err
	}
	return c.emit(a, f, func(w io.Writer) {
		if len(f.Findings) == 0 {
			fmt.Fprintf(w, "%d documents, no findings.\n", f.Checked)
			return
		}
		groups := map[string][]lint.Finding{}
		var order []string
		for _, x := range f.Findings {
			k := x.Severity + " " + x.Check
			if _, ok := groups[k]; !ok {
				order = append(order, k)
			}
			groups[k] = append(groups[k], x)
		}
		for _, k := range order {
			list := groups[k]
			fmt.Fprintf(w, "%s (%d) · fix: %s\n", k, len(list), list[0].Fix)
			for _, x := range list {
				fmt.Fprintf(w, "  %s: %s\n", x.Doc.Path, x.Message)
			}
		}
		fmt.Fprintf(w, "%d documents · %d errors · %d warnings · %d info\n", f.Checked, f.Counts[lint.Error], f.Counts[lint.Warning], f.Counts[lint.Info])
	})
}
