package scope

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// The grid of the threads canvas. A group holds its cards in columns; a card that sits
// exactly on a slot of its group is the code's to place, and a card anywhere else is one
// the user moved, which keeps its place.
const (
	cardW    = 400
	cardH    = 280
	cardGap  = 40
	groupPad = 40
	columns  = 2
	groupGap = 160
	groupW   = 2*groupPad + columns*cardW + (columns-1)*cardGap

	noScope      = "none"
	noScopeLabel = "No area"
)

// stageColor is the canvas color of each open stage, near the stage's callout color.
var stageColor = map[string]string{"stub": "3", "spec": "5", "tasks": "6"}

var stageRank = map[string]int{"tasks": 0, "spec": 1, "stub": 2}

var priorityRank = map[string]int{"high": 0, "normal": 1, "low": 2, "someday": 3}

type rect struct{ x, y, w, h int }

func (r rect) contains(x, y int) bool {
	return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

type canvasFile struct {
	Nodes []map[string]any `json:"nodes"`
	Edges []map[string]any `json:"edges"`
}

// group is one scope's group on the canvas and the open threads whose home it is.
type group struct {
	id, label string
	sortKey   string
	threads   []*doc.Doc
	at        *rect
}

// Canvas is the threads canvas: a group for each scope that is the home of an open
// thread, and a card for each open thread in its home's group. A thread's home is its
// first scope; a thread with none sits in "No area". The code owns which cards and groups
// exist, the groups' labels and sizes, and the cards on the grid. The user owns the rest:
// where a group sits, a card moved off the grid, their own nodes, and every edge whose
// two ends are still on the canvas. It reports whether the content differs from existing.
func Canvas(idx *vault.Index, existing []byte) (string, bool) {
	var old canvasFile
	if len(existing) > 0 && json.Unmarshal(existing, &old) != nil {
		old = canvasFile{}
	}
	oldGroups := map[string]rect{}
	oldCards := map[string]map[string]any{}
	var userNodes []map[string]any
	for _, n := range old.Nodes {
		id, _ := n["id"].(string)
		switch {
		case strings.HasPrefix(id, "grp-"):
			oldGroups[id] = rectOf(n)
		case strings.HasPrefix(id, "thr-"):
			oldCards[id] = n
		default:
			userNodes = append(userNodes, n)
		}
	}

	groups := map[string]*group{}
	for _, d := range idx.Of("stub") {
		stage := d.Str("stage")
		if stage == "" {
			stage = "stub"
		}
		if stage == "closed" || d.FrontErr != nil || !strings.HasPrefix(d.Path, vault.Threads+"/") {
			continue
		}
		home, label, key := noScope, noScopeLabel, "￿"
		if h := idx.Home(d); h != nil {
			var titles []string
			for _, s := range Path(idx, h) {
				titles = append(titles, vault.Title(s))
			}
			home, label, key = h.ID(), strings.Join(titles, " / "), strings.ToLower(strings.Join(titles, "\x00"))
		}
		id := "grp-" + home
		g := groups[id]
		if g == nil {
			g = &group{id: id, label: label, sortKey: key}
			if r, ok := oldGroups[id]; ok {
				g.at = &r
			}
			groups[id] = g
		}
		g.threads = append(g.threads, d)
	}
	// A card on the grid keeps its place among the cards of its stage and priority; a new
	// card follows them.
	slot := map[string]rect{}
	for id, n := range oldCards {
		if !moved(n, oldGroups) {
			slot[id] = rectOf(n)
		}
	}
	order := make([]*group, 0, len(groups))
	for _, g := range groups {
		order = append(order, g)
		sort.SliceStable(g.threads, func(i, j int) bool { return cardLess(g.threads[i], g.threads[j], slot) })
	}
	sort.Slice(order, func(i, j int) bool { return order[i].sortKey < order[j].sortKey })

	// A card stays where the user put it: off the grid of the group it sits in.
	kept := map[string]map[string]any{}
	for _, g := range order {
		for _, d := range g.threads {
			if n, ok := oldCards[d.ID()]; ok && moved(n, oldGroups) {
				kept[d.ID()] = n
			}
		}
	}

	var groupNodes, cardNodes []map[string]any
	right, top, placed := 0, 0, false
	grow := func(r rect) {
		if !placed || r.x+r.w > right {
			right = r.x + r.w
		}
		if !placed || r.y < top {
			top = r.y
		}
		placed = true
	}
	for _, g := range order {
		if g.at != nil {
			grow(*g.at)
		}
	}
	for _, n := range userNodes {
		grow(rectOf(n))
	}
	for _, n := range kept {
		grow(rectOf(n))
	}
	next := 0
	if placed {
		next = right + groupGap
	}
	for _, g := range order {
		if g.at == nil {
			g.at = &rect{x: next, y: top}
			next += groupW + groupGap
		}
		slots := 0
		for _, d := range g.threads {
			if n, ok := kept[d.ID()]; ok {
				n["file"] = d.Path
				n["color"] = color(d)
				cardNodes = append(cardNodes, n)
				continue
			}
			col, row := slots%columns, slots/columns
			slots++
			cardNodes = append(cardNodes, map[string]any{
				"id":     d.ID(),
				"type":   "file",
				"file":   d.Path,
				"x":      g.at.x + groupPad + col*(cardW+cardGap),
				"y":      g.at.y + groupPad + row*(cardH+cardGap),
				"width":  cardW,
				"height": cardH,
				"color":  color(d),
			})
		}
		rows := max(1, (slots+columns-1)/columns)
		groupNodes = append(groupNodes, map[string]any{
			"id":     g.id,
			"type":   "group",
			"label":  g.label,
			"x":      g.at.x,
			"y":      g.at.y,
			"width":  groupW,
			"height": 2*groupPad + rows*cardH + (rows-1)*cardGap,
		})
	}

	nodes := append(append(append([]map[string]any{}, groupNodes...), cardNodes...), userNodes...)
	out := canvasFile{Nodes: nodes, Edges: []map[string]any{}}
	ids := map[string]bool{}
	for _, n := range out.Nodes {
		id, _ := n["id"].(string)
		ids[id] = true
	}
	for _, e := range old.Edges {
		from, _ := e["fromNode"].(string)
		to, _ := e["toNode"].(string)
		if ids[from] && ids[to] {
			out.Edges = append(out.Edges, e)
		}
	}
	data, _ := json.MarshalIndent(out, "", "\t")
	content := string(data) + "\n"
	return content, !sameJSON(existing, data)
}

// moved reports whether a card sits off the grid: outside every group, between the
// slots of the group it sits in, or resized.
func moved(n map[string]any, groups map[string]rect) bool {
	r := rectOf(n)
	if r.w != cardW || r.h != cardH {
		return true
	}
	for _, g := range groups {
		if !g.contains(r.x, r.y) {
			continue
		}
		dx, dy := r.x-g.x-groupPad, r.y-g.y-groupPad
		return dx < 0 || dy < 0 || dx%(cardW+cardGap) != 0 || dy%(cardH+cardGap) != 0 || dx/(cardW+cardGap) >= columns
	}
	return true
}

// cardLess orders a group's cards: the furthest stage first, then by priority, then the
// cards already on the grid in their order, then the new ones by title. It reads nothing
// that changes while a thread only runs, so cards stay put.
func cardLess(a, b *doc.Doc, slot map[string]rect) bool {
	if sa, sb := stageRank[a.Str("stage")], stageRank[b.Str("stage")]; sa != sb {
		return sa < sb
	}
	pa, ok := priorityRank[a.Str("priority")]
	if !ok {
		pa = 1
	}
	pb, ok := priorityRank[b.Str("priority")]
	if !ok {
		pb = 1
	}
	if pa != pb {
		return pa < pb
	}
	sa, oka := slot[a.ID()]
	sb, okb := slot[b.ID()]
	if oka != okb {
		return oka
	}
	if oka && sa.y != sb.y {
		return sa.y < sb.y
	}
	if oka && sa.x != sb.x {
		return sa.x < sb.x
	}
	return strings.ToLower(a.Title()) < strings.ToLower(b.Title())
}

func color(d *doc.Doc) string {
	if c, ok := stageColor[d.Str("stage")]; ok {
		return c
	}
	return stageColor["stub"]
}

func rectOf(n map[string]any) rect {
	num := func(k string) int {
		switch v := n[k].(type) {
		case float64:
			return int(v)
		case int:
			return v
		}
		return 0
	}
	return rect{x: num("x"), y: num("y"), w: num("width"), h: num("height")}
}

// sameJSON reports whether two JSON texts hold the same data. Obsidian rewrites a canvas
// in its own layout, and that is no change.
func sameJSON(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}
