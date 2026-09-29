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
var scoped = []string{"area", "repository", "concept", "entity", "policy", "source", "stub"}

// Chain is the scopes a page belongs to, from the top of the graph down: for a
// knowledge page its scope and every area above it; for an area or a repository the
// areas above it; for a thread each of its scopes and the areas above each, the first
// scope's path first. A Base that asks for "chain contains this scope" finds a scope's
// pages and threads, and every one below it.
func ChainOf(idx *vault.Index, d *doc.Doc) []string {
	var starts []*doc.Doc
	switch d.Type() {
	case "area", "repository":
		if p := idx.Parent(d); p != nil {
			starts = []*doc.Doc{p}
		}
	case "stub":
		for _, id := range idx.ScopeIDs(d) {
			starts = append(starts, idx.ByID(id))
		}
	default:
		if ids := idx.ScopeIDs(d); len(ids) > 0 {
			starts = []*doc.Doc{idx.ByID(ids[0])}
		}
	}
	out := []string{}
	seen := map[string]bool{}
	for _, start := range starts {
		for _, s := range Path(idx, start) {
			if !seen[s.ID()] {
				seen[s.ID()] = true
				out = append(out, doc.Link(vault.Title(s)))
			}
		}
	}
	return out
}

// Path is a scope page and the areas above it, from the top of the graph down.
func Path(idx *vault.Index, d *doc.Doc) []*doc.Doc {
	up := append([]*doc.Doc{d}, idx.Ancestors(d)...)
	out := make([]*doc.Doc, 0, len(up))
	for i := len(up) - 1; i >= 0; i-- {
		out = append(out, up[i])
	}
	return out
}

// Derive is each scoped page's and each thread's content with its chain current and, for
// an area or a repository, its lead callout; Atlas.md's content with the map of the
// graph; and the threads canvas. It returns only the contents that differ from the files.
func Derive(idx *vault.Index) map[string]string {
	out := map[string]string{}
	for _, d := range idx.Of(scoped...) {
		home := vault.Wiki
		if d.Type() == "stub" {
			home = vault.Threads
		}
		if !strings.HasPrefix(d.Path, home+"/") || d.FrontErr != nil {
			continue
		}
		content := d.Content
		if field := placedField(d); field != "" && !idx.Legacy() {
			if link := idx.ScopeLink(d); link != "" || d.Front.Has(field) {
				content = doc.SetField(content, field, link)
			}
		}
		if d.Type() == "stub" {
			if scopes := homeFirst(idx, d); scopes != nil {
				content = doc.SetField(content, "scope", scopes)
			}
		}
		content = doc.SetField(content, "chain", ChainOf(idx, d))
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
	existing, _ := idx.V.Read(vault.ThreadsCanvas)
	if content, changed := Canvas(idx, existing); changed {
		out[vault.ThreadsCanvas] = content
	}
	return out
}

// homeFirst is a stub's scope list with its home, the scope its folder lies in, first; nil
// when the thread lies at the top of threads/ or its list already starts with the home.
func homeFirst(idx *vault.Index, d *doc.Doc) []string {
	home := idx.Home(d)
	if home == nil {
		return nil
	}
	list := d.List("scope")
	if len(list) > 0 {
		if s := idx.Linked(list[0]); s != nil && s.ID() == home.ID() {
			return nil
		}
	}
	out := []string{doc.Link(vault.Title(home))}
	for _, v := range list {
		if s := idx.Linked(v); s == nil || s.ID() != home.ID() {
			out = append(out, v)
		}
	}
	return out
}

// placedField is the field of a wiki page that its folder decides: a scope page's parent,
// a knowledge page's scope; "" for a thread.
func placedField(d *doc.Doc) string {
	switch d.Type() {
	case "area", "repository":
		return "parent"
	case "stub":
		return ""
	}
	return "scope"
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
// holds the page that shows it, grouped by type, and every open thread whose chain holds
// it. Inline, it needs no file of its own.
var scopeView = []string{
	"```base",
	"filters:",
	"  and:",
	`    - 'file.ext == "md"'`,
	"    - 'chain.contains(this.file.asLink())'",
	"formulas:",
	`  stage_order: 'if(stage == "tasks", "1 · tasks", if(stage == "spec", "2 · spec", "3 · stub"))'`,
	"views:",
	"  - type: table",
	"    name: In this scope",
	"    filters:",
	"      and:",
	`        - file.inFolder("wiki")`,
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
	"  - type: table",
	"    name: Threads",
	"    filters:",
	"      and:",
	`        - 'type == "stub"'`,
	`        - 'stage != "closed"'`,
	"    groupBy:",
	"      property: formula.stage_order",
	"      direction: ASC",
	"    order:",
	"      - file.name",
	"      - priority",
	"      - tasks",
	"      - active",
	"      - scope",
	"      - updated",
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
