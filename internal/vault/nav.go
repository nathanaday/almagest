package vault

import (
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/links"
)

// NavNode is what the rule of Up reads of one document.
type NavNode struct {
	ID   string
	Type string
	// HasUp is true when the up field holds a value, even one that names no document.
	HasUp bool
	// Up are the ids its up field names.
	Up      []string
	Tags    []string
	Defines string
}

// UpEdges gives each node the nodes directly above it. A topic or repository with a value
// in up takes the topics and repositories it names. Any other document takes the
// document that defines each tag it holds exactly. No node is above itself.
func UpEdges(nodes []NavNode) map[string][]string {
	byID := map[string]NavNode{}
	page := map[string]string{}
	for _, n := range nodes {
		byID[n.ID] = n
		if def := strings.ToLower(n.Defines); def != "" && page[def] == "" {
			page[def] = n.ID
		}
	}
	out := map[string][]string{}
	for _, n := range nodes {
		var ups []string
		add := func(id string) {
			if id != "" && id != n.ID && !slices.Contains(ups, id) {
				ups = append(ups, id)
			}
		}
		if n.HasUp && takesUp(n.Type) {
			for _, id := range n.Up {
				if takesUp(byID[id].Type) {
					add(id)
				}
			}
		} else {
			for _, t := range n.Tags {
				add(page[strings.ToLower(strings.TrimPrefix(strings.TrimSpace(t), "#"))])
			}
		}
		if len(ups) > 0 {
			out[n.ID] = ups
		}
	}
	return out
}

// takesUp reports whether a type has an up field, and may be named in one.
func takesUp(typ string) bool { return typ == "topic" || typ == "repository" }

// Cycles are the groups of nodes that reach themselves by their edges, each sorted, in
// order: the strongly connected components with more than one node, or with an edge to
// itself.
func Cycles(nodes []string, edges map[string][]string) [][]string {
	index := map[string]int{}
	low := map[string]int{}
	on := map[string]bool{}
	var stack []string
	var out [][]string
	next := 0
	var visit func(v string)
	visit = func(v string) {
		index[v], low[v] = next, next
		next++
		stack = append(stack, v)
		on[v] = true
		for _, w := range edges[v] {
			if _, seen := index[w]; !seen {
				visit(w)
				low[v] = min(low[v], low[w])
			} else if on[w] {
				low[v] = min(low[v], index[w])
			}
		}
		if low[v] != index[v] {
			return
		}
		var group []string
		for {
			w := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			on[w] = false
			group = append(group, w)
			if w == v {
				break
			}
		}
		if len(group) > 1 || slices.Contains(edges[v], v) {
			sort.Strings(group)
			out = append(out, group)
		}
	}
	sorted := slices.Clone(nodes)
	sort.Strings(sorted)
	for _, v := range sorted {
		if _, seen := index[v]; !seen {
			visit(v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

type nav struct {
	once  sync.Once
	up    map[string][]string
	below map[string][]string
}

// NavNodeOf reads a document for the rule of Up. Its up links resolve by title or path,
// as the navigator's Base and Obsidian match them; an id, an alias, or plain text names
// nothing here.
func (idx *Index) NavNodeOf(d *doc.Doc) NavNode {
	n := NavNode{ID: d.ID(), Type: d.Type(), Tags: d.List("tags"), Defines: d.Str("defines")}
	for _, v := range d.List("up") {
		if strings.TrimSpace(v) == "" {
			continue
		}
		n.HasUp = true
		if title, _ := idx.Misnamed(v); title != "" {
			continue
		}
		if target := idx.Linked(v); target != nil {
			n.Up = append(n.Up, target.ID())
		}
	}
	return n
}

// Misnamed is the title of the document a frontmatter value names in a way Obsidian does
// not link, and how: "by its id", "by an alias", or "as plain text" for a value that is no
// [[link]]. It is "" for a link that a file's title or path holds, or that names nothing.
func (idx *Index) Misnamed(link string) (title, how string) {
	t := doc.LinkTarget(link)
	if t == "" {
		return "", ""
	}
	if !doc.IsLink(link) {
		if d := idx.Linked(t); d != nil {
			return Title(d), "as plain text"
		}
		if d, err := idx.Resolve(t); err == nil {
			return Title(d), "as plain text"
		}
		return "", ""
	}
	if len(idx.LinkPaths(t)) > 0 {
		return "", ""
	}
	if d := idx.byID[t]; d != nil {
		return Title(d), "by its id"
	}
	for _, d := range idx.byAlias[links.Key(t)] {
		return Title(d), "by an alias"
	}
	return "", ""
}

func (idx *Index) readNav() {
	var nodes []NavNode
	for _, d := range idx.Documents() {
		nodes = append(nodes, idx.NavNodeOf(d))
	}
	idx.nav.up = UpEdges(nodes)
	idx.nav.below = map[string][]string{}
	for _, n := range nodes {
		for _, u := range idx.nav.up[n.ID] {
			idx.nav.below[u] = append(idx.nav.below[u], n.ID)
		}
	}
}

func (idx *Index) docs(ids []string) []*doc.Doc {
	out := make([]*doc.Doc, 0, len(ids))
	for _, id := range ids {
		if d := idx.byID[id]; d != nil {
			out = append(out, d)
		}
	}
	return out
}

// Up are the documents directly above d: the topics and repositories its up names, or,
// when its up is empty, the page of each tag its tags hold exactly.
func (idx *Index) Up(d *doc.Doc) []*doc.Doc {
	idx.nav.once.Do(idx.readNav)
	return idx.docs(idx.nav.up[d.ID()])
}

// Below are the documents directly below d, in path order.
func (idx *Index) Below(d *doc.Doc) []*doc.Doc {
	idx.nav.once.Do(idx.readNav)
	return idx.docs(idx.nav.below[d.ID()])
}

// Under counts the documents below d at any depth, each once.
func (idx *Index) Under(d *doc.Doc) int {
	idx.nav.once.Do(idx.readNav)
	seen := map[string]bool{d.ID(): true}
	queue := []string{d.ID()}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, b := range idx.nav.below[id] {
			if !seen[b] {
				seen[b] = true
				queue = append(queue, b)
			}
		}
	}
	return len(seen) - 1
}

// UpCycles are the groups of documents that reach themselves by Up.
func (idx *Index) UpCycles() [][]*doc.Doc {
	idx.nav.once.Do(idx.readNav)
	var ids []string
	for _, d := range idx.Documents() {
		ids = append(ids, d.ID())
	}
	var out [][]*doc.Doc
	for _, group := range Cycles(ids, idx.nav.up) {
		out = append(out, idx.docs(group))
	}
	return out
}
