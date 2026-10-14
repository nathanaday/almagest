package lint

import (
	"sort"
	"strings"

	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/vault"
)

// MaxBelow is the most topics and repositories a document holds directly below it
// before lint calls it crowded.
const MaxBelow = 30

// ups checks the up field: a source takes none, no document names itself, and no group of
// documents sits under itself. A dead link, an alias too, is the schema check's.
func (r *run) ups(docs []*doc.Doc) {
	in := map[string]bool{}
	for _, d := range docs {
		in[d.ID()] = true
		values := d.List("up")
		if len(values) == 0 {
			continue
		}
		if d.Type() == "source" {
			r.add("up", Error, d, fixFor(d)+": drop up", "a source takes no up; it sits under the page of its tag, and its topics cite it")
			continue
		}
		for _, v := range values {
			target := r.idx.Linked(v)
			if target == nil {
				target, _ = r.idx.Resolve(doc.LinkTarget(v))
			}
			if target == d {
				r.add("up", Error, d, fixFor(d)+": drop that link", "up names the document itself")
			}
		}
	}
	for _, group := range r.idx.UpCycles() {
		sort.Slice(group, func(i, j int) bool { return vault.Title(group[i]) < vault.Title(group[j]) })
		if !in[group[0].ID()] {
			continue
		}
		names := make([]string, len(group))
		for i, d := range group {
			names[i] = vault.Title(d)
		}
		r.add("up", Error, group[0], fixFor(group[0])+": drop or change one up link among them", "%s reach themselves through their up links (a tag page counts for a document whose up is empty)", strings.Join(names, ", "))
	}
}

// crowded warns about a document with too many topics and repositories directly below
// it. Sources do not count: they take no up, so no map could move them.
func (r *run) crowded(d *doc.Doc) {
	n := 0
	for _, b := range r.idx.Below(d) {
		if b.Type() == "topic" || b.Type() == "repository" {
			n++
		}
	}
	if n > MaxBelow {
		r.add("crowded", Warning, d, "wiki-map: group them under hub topics, and set their up", "%d topics and repositories sit directly below it; more than %d are hard to scan", n, MaxBelow)
	}
}
