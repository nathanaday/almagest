package scope

import (
	"sort"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Writer writes a file only when its content differs and reports whether it wrote.
type Writer func(rel string, content []byte) (bool, error)

// scoped are the types that carry a chain.
var scoped = []string{"area", "repository", "concept", "entity", "policy", "source"}

// Chain is the scopes a page belongs to, from the top of the graph down: for a
// knowledge page its scope and every area above it; for an area or a repository the
// areas above it. A Base that asks for "chain contains this scope" finds a scope's pages
// and every page below it.
func ChainOf(idx *vault.Index, d *doc.Doc) []string {
	var start *doc.Doc
	var out []string
	switch d.Type() {
	case "area", "repository":
		start = idx.Parent(d)
	default:
		if ids := idx.ScopeIDs(d); len(ids) > 0 {
			start = idx.ByID(ids[0])
		}
	}
	if start == nil {
		return []string{}
	}
	list := append([]*doc.Doc{start}, idx.Ancestors(start)...)
	for i := len(list) - 1; i >= 0; i-- {
		out = append(out, doc.Link(vault.Title(list[i])))
	}
	return out
}

// Derive is each scoped page's content with its chain current and, for an area or a
// repository, its lead callout; and Atlas.md's content with the map of the graph. It
// returns only the contents that differ from the files.
func Derive(idx *vault.Index) map[string]string {
	out := map[string]string{}
	for _, d := range idx.Of(scoped...) {
		if !strings.HasPrefix(d.Path, vault.Wiki+"/") || d.FrontErr != nil {
			continue
		}
		content := doc.SetField(d.Content, "chain", ChainOf(idx, d))
		if d.Type() == "area" || d.Type() == "repository" {
			content = doc.ReplaceLead(content, scopeLead(idx, d))
		}
		if content != d.Content {
			out[d.Path] = content
		}
	}
	if atlas := idx.ByPath(vault.Marker); atlas != nil {
		content := doc.ReplaceLead(atlas.Content, mapLead(idx))
		if content != atlas.Content {
			out[atlas.Path] = content
		}
	}
	return out
}

// Heal writes what Derive finds, and returns the paths it wrote.
func Heal(idx *vault.Index, write Writer) ([]string, error) {
	changed := Derive(idx)
	paths := make([]string, 0, len(changed))
	for p := range changed {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var wrote []string
	for _, p := range paths {
		ok, err := write(p, []byte(changed[p]))
		if err != nil {
			return wrote, err
		}
		if ok {
			wrote = append(wrote, p)
		}
	}
	return wrote, nil
}

// scopeView is the Base a scope page's lead callout holds: every wiki page whose chain
// holds the page that shows it, grouped by type. Inline, it needs no file of its own.
var scopeView = []string{
	"```base",
	"filters:",
	"  and:",
	`    - file.inFolder("wiki")`,
	`    - 'file.ext == "md"'`,
	"    - 'chain.contains(this.file.asLink())'",
	"views:",
	"  - type: table",
	"    name: In this scope",
	"    groupBy:",
	"      property: type",
	"      direction: ASC",
	"    order:",
	"      - file.name",
	"      - description",
	"      - scope",
	"      - status",
	"    sort:",
	"      - property: file.name",
	"        direction: ASC",
	"```",
}

// scopeLead is the lead callout of an area or a repository: the path from the vault
// down to it, and the view of every page in it and below it.
func scopeLead(idx *vault.Index, d *doc.Doc) string {
	crumbs := []string{"[[Atlas|" + idx.V.Name() + "]]"}
	crumbs = append(crumbs, ChainOf(idx, d)...)
	crumbs = append(crumbs, "**"+vault.Title(d)+"**")
	lines := append([]string{strings.Join(crumbs, " → "), ""}, scopeView...)
	return doc.Callout(d.Type(), vault.Title(d), lines...)
}

// mapLead is Atlas.md's lead callout: the areas and repositories as a tree.
func mapLead(idx *vault.Index) string {
	children := map[string][]*doc.Doc{}
	for _, d := range idx.Of("area", "repository") {
		parent := ""
		if p := idx.Parent(d); p != nil {
			parent = p.ID()
		}
		children[parent] = append(children[parent], d)
	}
	for _, list := range children {
		sort.Slice(list, func(i, j int) bool {
			if list[i].Type() != list[j].Type() {
				return list[i].Type() == "area"
			}
			return strings.ToLower(vault.Title(list[i])) < strings.ToLower(vault.Title(list[j]))
		})
	}
	var lines []string
	seen := map[string]bool{}
	var walk func(parent string, depth int)
	walk = func(parent string, depth int) {
		for _, d := range children[parent] {
			if seen[d.ID()] {
				continue
			}
			seen[d.ID()] = true
			lines = append(lines, strings.Repeat("    ", depth)+"- "+doc.Link(vault.Title(d)))
			walk(d.ID(), depth+1)
		}
	}
	walk("", 0)
	if len(lines) == 0 {
		lines = []string{"No area or repository yet. The repo-link skill links the first repository."}
	}
	return doc.Callout("atlas", idx.V.Name(), lines...)
}
