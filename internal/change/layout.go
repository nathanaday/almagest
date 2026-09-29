package change

import (
	"path"
	"sort"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/links"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Move is a file that moves and is not written: a file in the folder of a scope the
// change renames, gives a new parent, or removes. For a folder, From and To are folders.
type Move struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Files int    `json:"files,omitempty"`
}

// scopeNode is a scope page as the change leaves it.
type scopeNode struct {
	title    string
	parent   string // id, or "" for the vault
	removed  bool
	redirect string // id of the page a removed scope's links go to
	folder   string // the folder the page makes a scope now; "" for a new page or one outside its folder
}

// layout is where the change puts every file of the wiki.
type layout struct {
	c     *check
	nodes map[string]*scopeNode
	dirs  map[string]string
	busy  map[string]bool
}

// place sets where each page of the change ends up, and lists every other file that moves
// with the folder of a scope: a scope's page and everything in its folder move when the
// change renames it or gives it a new parent, and move to the parent's folder, or to the
// redirect's, when the change removes it. A knowledge page moves when its scope changes.
func (c *check) place(ops []*op) ([]Move, []Move) {
	l := &layout{c: c, nodes: map[string]*scopeNode{}, dirs: map[string]string{}, busy: map[string]bool{}}
	final := map[string]string{}  // id → the page's content as the change leaves it
	titles := map[string]string{} // id → the page's title as the change leaves it
	for _, o := range ops {
		switch o.Kind {
		case "create", "modify":
			final[o.ID] = o.Content
		}
		if _, ok := titles[o.ID]; !ok {
			titles[o.ID] = o.Title
		}
		if o.Kind == "rename" {
			titles[o.ID] = o.NewTitle
		}
	}
	for _, d := range c.idx.Of("area", "repository") {
		n := &scopeNode{title: vault.Title(d), folder: c.idx.Folder(d)}
		if p := c.idx.Parent(d); p != nil {
			n.parent = p.ID()
		}
		if t, ok := titles[d.ID()]; ok {
			n.title = t
		}
		if content, ok := final[d.ID()]; ok {
			n.parent = c.scopeID(doc.Parse(d.Path, []byte(content)).Str("parent"))
		}
		l.nodes[d.ID()] = n
	}
	for _, o := range ops {
		switch {
		case o.Kind == "create" && (o.Type == "area" || o.Type == "repository"):
			l.nodes[o.ID] = &scopeNode{title: o.Title, parent: c.scopeID(doc.Parse(o.Path, []byte(o.Content)).Str("parent"))}
		case o.Kind == "remove" && l.nodes[o.ID] != nil:
			n := l.nodes[o.ID]
			n.removed = true
			if r := l.nodes[o.Redirect]; r != nil && !r.removed {
				n.redirect = o.Redirect
			}
		}
	}
	for _, o := range ops {
		if (o.Kind == "create" || o.Kind == "rename") && (o.Type == "area" || o.Type == "repository") && vault.ReservedTitle(titles[o.ID]) {
			c.refuse("%s: %q is the name of a type folder; an area or a repository takes another title", o.label(), titles[o.ID])
		}
	}

	// Where each page of the change ends up.
	byPath := map[string]bool{}
	for _, o := range ops {
		if o.Kind == "remove" {
			byPath[o.Path] = true
			continue
		}
		var to string
		switch {
		case o.Type == "area" || o.Type == "repository":
			to = l.dir(o.ID, o) + "/" + titles[o.ID] + ".md"
		case o.Kind == "create":
			to = vault.Route(l.scopeDir(c.scopeID(doc.Parse(o.Path, []byte(o.Content)).Str("scope")), o), o.Type, o.Title)
		default:
			d := c.idx.ByID(o.ID)
			old := ""
			if ids := c.idx.ScopeIDs(d); len(ids) > 0 {
				old = ids[0]
			}
			now := old
			if content, ok := final[o.ID]; ok {
				now = c.scopeID(doc.Parse(d.Path, []byte(content)).Str("scope"))
			}
			if now != old {
				to = vault.Route(l.scopeDir(now, o), o.Type, titles[o.ID])
			} else {
				to = path.Dir(l.relocate(d.Path)) + "/" + titles[o.ID] + ".md"
			}
		}
		if o.Kind == "create" {
			o.Path = to
		} else if to != o.Path {
			o.NewPath = to
		}
		byPath[o.Path] = true
	}

	// Every other file of the wiki moves with its scope's folder.
	var moves []Move
	files := make([]string, 0, len(c.idx.Docs)+len(c.idx.Notes)+len(c.idx.Files))
	for _, d := range c.idx.Docs {
		files = append(files, d.Path)
	}
	for _, d := range c.idx.Notes {
		files = append(files, d.Path)
	}
	files = append(files, c.idx.Files...)
	for _, p := range files {
		if !(strings.HasPrefix(p, vault.Wiki+"/") || strings.HasPrefix(p, vault.Threads+"/")) || byPath[p] {
			continue
		}
		if to := l.relocate(p); to != p {
			moves = append(moves, Move{From: p, To: to})
		}
	}
	sort.Slice(moves, func(i, j int) bool { return moves[i].From < moves[j].From })
	l.collisions(ops, moves)
	return moves, l.folderMoves(moves)
}

// scopeID is the id of the scope page a field's link names once the change applies, or
// "" for the vault.
func (c *check) scopeID(value string) string {
	t := doc.LinkTarget(value)
	if t == "" {
		return ""
	}
	if o := c.claimed[links.Key(t)]; o != nil {
		return o.ID
	}
	if d := c.idx.ByID(t); d != nil {
		return d.ID()
	}
	if d, err := c.idx.Resolve(t); err == nil {
		return d.ID()
	}
	return ""
}

// scopeDir is the folder a page goes in for a scope, refusing a scope whose page lies
// outside its folder and that the change does not move.
func (l *layout) scopeDir(id string, o *op) string {
	if id == "" {
		return vault.Wiki
	}
	if n := l.nodes[id]; n != nil && n.folder == "" && !l.touched(id) {
		if d := l.c.idx.ByID(id); d != nil {
			l.c.refuse("%s: the page of %s lies at %s, outside a folder of its own; move it to …/%s/%s.md first (lint lists it)", o.label(), n.title, d.Path, n.title, n.title)
		}
	}
	return l.dir(id, o)
}

// touched reports whether the change writes a page.
func (l *layout) touched(id string) bool {
	return len(l.c.byID[id]) > 0
}

// dir is the folder a scope makes once the change applies. A removed scope's folder
// empties into its redirect's, or its parent's.
func (l *layout) dir(id string, o *op) string {
	if d, ok := l.dirs[id]; ok {
		return d
	}
	n := l.nodes[id]
	if n == nil {
		return vault.Wiki
	}
	if l.busy[id] {
		l.c.refuse("%s: the parents of %s loop", o.label(), n.title)
		return vault.Wiki
	}
	l.busy[id] = true
	var d string
	switch {
	case n.removed && n.redirect != "":
		d = l.dir(n.redirect, o)
	case n.removed:
		d = l.dirOrWiki(n.parent, o)
	default:
		d = l.dirOrWiki(n.parent, o) + "/" + n.title
	}
	l.busy[id] = false
	l.dirs[id] = d
	return d
}

func (l *layout) dirOrWiki(id string, o *op) string {
	if id == "" {
		return vault.Wiki
	}
	return l.dir(id, o)
}

// relocate is where a file lands when the folder of its scope moves. A thread's files move
// with the scope's mirror under threads/.
func (l *layout) relocate(p string) string {
	s := l.c.idx.Container(p)
	if s == nil {
		return p
	}
	from := l.c.idx.Folder(s)
	to := l.dir(s.ID(), &op{Kind: "move", Title: p})
	if to == from {
		return p
	}
	if strings.HasPrefix(p, vault.Threads+"/") {
		from, to = vault.Mirror(from), vault.Mirror(to)
	}
	return to + strings.TrimPrefix(p, from)
}

// collisions refuses two files on one path, and a file that lands on a file that stays.
func (l *layout) collisions(ops []*op, moves []Move) {
	removed := map[string]bool{}
	for _, o := range ops {
		if o.Kind == "remove" {
			removed[strings.ToLower(o.Path)] = true
		}
	}
	taken := map[string]string{}
	claim := func(to, from string) {
		key := strings.ToLower(to)
		if other, ok := taken[key]; ok && other != from {
			l.c.refuse("%s and %s would both land at %s", orNew(other), orNew(from), to)
			return
		}
		taken[key] = from
		if l.c.idx.V.Exists(to) && !strings.EqualFold(to, from) && !removed[key] {
			l.c.refuse("%s would land at %s, which holds a file", orNew(from), to)
		}
	}
	for _, o := range ops {
		switch {
		case o.Kind == "create":
			claim(o.Path, "")
		case o.Kind != "remove" && o.NewPath != "":
			claim(o.NewPath, o.Path)
		}
	}
	for _, m := range moves {
		claim(m.To, m.From)
	}
}

func orNew(from string) string {
	if from == "" {
		return "a new page"
	}
	return from
}

// folderMoves are the scope folders that move, with the files each carries.
func (l *layout) folderMoves(moves []Move) []Move {
	var out []Move
	for _, dir := range sortedFolders(l.c.idx) {
		s := l.c.idx.ScopeFolders()[dir]
		to := l.dir(s.ID(), &op{Kind: "move", Title: dir})
		if to == dir {
			continue
		}
		if p := l.c.idx.Container(dir); p != nil {
			from, at := l.c.idx.Folder(p), l.dir(p.ID(), &op{Kind: "move", Title: dir})
			if at != from && to == at+strings.TrimPrefix(dir, from) {
				continue // it moves with its parent's folder, which the parent's line shows
			}
		}
		n := 0
		for _, m := range moves {
			if strings.HasPrefix(m.From, dir+"/") || strings.HasPrefix(m.From, vault.Mirror(dir)+"/") {
				n++
			}
		}
		out = append(out, Move{From: dir, To: to, Files: n})
	}
	return out
}

func sortedFolders(idx *vault.Index) []string {
	var out []string
	for dir := range idx.ScopeFolders() {
		out = append(out, dir)
	}
	sort.Strings(out)
	return out
}
