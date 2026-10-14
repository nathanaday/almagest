package change

import (
	"sort"
	"strings"

	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/links"
	"github.com/nathanaday/almagest/internal/schema"
	"github.com/nathanaday/almagest/internal/vault"
)

// upLinks checks the up fields of the vault as the change leaves it: no document above
// itself, no up that names a document the change removes, and no cycle that the change
// closes. A cycle the vault held before is lint's to report, so it blocks no unrelated
// change.
func (c *check) upLinks(out *planned) {
	final := map[string]*doc.Doc{}
	checked := map[string]bool{}
	var order []string
	for _, d := range c.idx.Documents() {
		if !c.removed(d.ID()) {
			final[d.ID()] = d
			order = append(order, d.ID())
		}
	}
	for _, o := range out.Ops {
		if !o.writesContent() {
			continue
		}
		if _, ok := final[o.ID]; !ok {
			order = append(order, o.ID)
		}
		final[o.ID] = doc.Parse(o.finalPath(), []byte(o.Content))
		if !o.Rewrite {
			checked[o.ID] = true
		}
	}
	title := func(id string) string {
		if d := c.idx.ByID(id); d != nil {
			return c.finalTitle(d)
		}
		return final[id].Title()
	}
	var after []vault.NavNode
	for _, id := range order {
		d := final[id]
		n := vault.NavNode{ID: id, Type: d.Type(), Tags: d.List("tags"), Defines: d.Str("defines")}
		for _, v := range d.List("up") {
			if strings.TrimSpace(v) == "" {
				continue
			}
			n.HasUp = true
			target := c.idOf(v)
			switch {
			case target == id:
				c.refuse("%s: up names %s itself; drop that link", title(id), title(id))
			case target == "" && !checked[id]:
				if gone := c.removedBy(v); gone != "" && c.redirected(v) {
					c.refuse("%s: up names %s, which this change removes, and its redirect is no topic or repository; redirect to a topic or repository, or modify the up of %s", title(id), gone, title(id))
				} else if gone != "" {
					c.refuse("%s: up names %s, which this change removes; give the remove a redirect, or modify the up of %s", title(id), gone, title(id))
				}
			}
			if target != "" {
				n.Up = append(n.Up, target)
			}
		}
		after = append(after, n)
	}
	var before []vault.NavNode
	var ids []string
	for _, d := range c.idx.Documents() {
		before = append(before, c.idx.NavNodeOf(d))
		ids = append(ids, d.ID())
	}
	old := map[string]bool{}
	for _, group := range vault.Cycles(ids, vault.UpEdges(before)) {
		old[strings.Join(group, ",")] = true
	}
	for _, group := range vault.Cycles(order, vault.UpEdges(after)) {
		if old[strings.Join(group, ",")] {
			continue
		}
		names := make([]string, len(group))
		for i, id := range group {
			names[i] = title(id)
		}
		sort.Strings(names)
		c.refuse("up: %s reach themselves through their up links (a tag page counts for a document whose up is empty); drop or change one of those links so that no document sits under itself", strings.Join(names, ", "))
	}
}

// idOf is the id of the topic or repository a link names once the change applies, by
// title, path, or id, as the navigator matches it; "" when it names none.
func (c *check) idOf(value string) string {
	t := doc.LinkTarget(value)
	if t == "" {
		return ""
	}
	if o := c.claimed[links.Key(t)]; o != nil {
		return o.ID
	}
	if d := c.idx.ByID(t); d != nil {
		if c.removed(d.ID()) {
			return ""
		}
		return d.ID()
	}
	if c.gone[links.Key(t)] {
		return ""
	}
	if d := c.idx.Linked(t); d != nil && !c.removed(d.ID()) {
		return d.ID()
	}
	return ""
}

// named is the document a link names by title, path, id, or alias, as the vault stands
// before the change; nil when it names none.
func (c *check) named(value string) *doc.Doc {
	t := doc.LinkTarget(value)
	if t == "" {
		return nil
	}
	if d := c.idx.Linked(t); d != nil {
		return d
	}
	if d, err := c.idx.Resolve(t); err == nil {
		return d
	}
	return nil
}

// removedBy is the title of the document a link names that this change removes, or "".
func (c *check) removedBy(value string) string {
	if d := c.named(value); d != nil && c.removed(d.ID()) {
		return vault.Title(d)
	}
	return ""
}

// renaming is the rename in this change that frees a link's title, or nil.
func (c *check) renaming(value string) *op {
	key := links.Key(doc.LinkTarget(value))
	for _, ops := range c.byID {
		for _, o := range ops {
			if o.Kind == OpRename && o.NewTitle != "" && links.Key(o.Title) == key {
				return o
			}
		}
	}
	return nil
}

// links checks every link of the writes as the change leaves them, after the rewrites,
// so a link a rename or a redirect moves counts at its new place. A link to a removed
// document, or one by id or alias, gets a refusal of its own; any other, the field's
// check. Propose checks the plan's own writes; apply, every write of the document.
func (c *check) links(out *planned, rewrites bool) {
	for _, o := range out.Ops {
		if !o.writesContent() || o.Rewrite && !rewrites {
			continue
		}
		d := doc.Parse(o.finalPath(), []byte(o.Content))
		t := schema.Get(o.Type)
		if d.FrontErr != nil || t == nil {
			continue
		}
		for _, f := range t.Fields {
			if f.Kind != schema.Link && f.Kind != schema.Links {
				continue
			}
			for _, v := range d.List(f.Name) {
				named := c.named(v)
				title, by := c.idx.Misnamed(v)
				switch {
				case named != nil && c.removed(named.ID()) && c.claimed[links.Key(doc.LinkTarget(v))] == nil:
					switch {
					case title != "":
						c.refuse("%s: %s: %s names %s by %s, and this change removes %s; change the link", o.label(), f.Name, v, title, by, title)
					case c.redirected(v):
						c.refuse("%s: %s: %s names %s, which this change removes, and its redirect is no %s; redirect to a %s, or change the link", o.label(), f.Name, v, vault.Title(named), strings.Join(f.Targets, " or "), strings.Join(f.Targets, " or "))
					default:
						c.refuse("%s: %s: %s names %s, which this change removes; give the remove a redirect, or change the link", o.label(), f.Name, v, vault.Title(named))
					}
				case title != "" && c.claimed[links.Key(doc.LinkTarget(v))] == nil:
					final := c.finalTitle(named)
					c.refuse("%s: %s: %s names %s by %s; Obsidian resolves a link by title, so link [[%s]]", o.label(), f.Name, v, final, by, final)
				default:
					for _, p := range t.CheckLink(f, v, c) {
						c.refuse("%s: %s", o.label(), p)
					}
				}
			}
		}
	}
}

// redirected reports whether a link names a document that this change removes with a
// redirect, so the link rewrite moves the link.
func (c *check) redirected(value string) bool {
	d := c.named(value)
	if d == nil {
		return false
	}
	for _, o := range c.byID[d.ID()] {
		if o.Kind == OpRemove && o.Redirect != "" {
			return true
		}
	}
	return false
}
