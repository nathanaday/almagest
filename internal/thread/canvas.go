package thread

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// A chord's canvas shows its threads as cards and their order as arrows: an arrow from A
// to B says B comes after A. Code writes the cards and the arrows from the stubs, and
// keeps where the user put each card. When the user redraws the arrows, the canvas
// differs from the stubs until the user saves the order (CanvasSave) or takes the stubs'
// order back (CanvasWrite). The chord's canvas field holds the hash of the graph code
// wrote last, so a sync tells the user's edit from its own.

// Sizes of a card and the gaps between cards, in canvas units.
const (
	CardWidth  = 440
	CardHeight = 340
	CardGapX   = 140
	CardGapY   = 60
)

// CanvasPath is where a chord's canvas lives.
func CanvasPath(chord *doc.Doc) string { return vault.Chords + "/" + chord.Title() + ".canvas" }

// Card colors, from Obsidian's six presets: 1 red, 2 orange, 3 yellow, 4 green, 5 cyan,
// 6 purple. "" is the default gray.
const (
	ColorDone     = "4" // verified or closed
	ColorReady    = "5" // can start now
	ColorBlocked  = "3" // blocked
	ColorStarted  = "6" // work under way
	ColorChecking = "2" // every task done, not verified yet
)

// Legend is what each card color means, in the order Obsidian's canvas bar shows them.
var Legend = []struct{ Color, Label string }{
	{ColorDone, "Verified"},
	{ColorReady, "Ready"},
	{ColorStarted, "Started"},
	{ColorChecking, "To verify"},
	{ColorBlocked, "Blocked"},
	{"", "Waiting"},
}

// CardColor is the color of a thread's card. A thread that waits on another, a dropped
// thread, and a resolved one stay gray.
func (b *Board) CardColor(s *doc.Doc) string {
	switch status := b.Status(s); {
	case status == Verified || status == Closed:
		return ColorDone
	case Ended(status):
		return ""
	case b.Blocked(s) != "":
		return ColorBlocked
	case status == Unverified:
		return ColorChecking
	case status == Started:
		return ColorStarted
	case b.ReadyToStart(s):
		return ColorReady
	}
	return ""
}

// graph is the threads of a canvas or of a chord, and the order between them.
type graph struct {
	members map[string]bool
	// edges holds "from>to": to comes after from.
	edges map[string]bool
}

func newGraph() *graph { return &graph{members: map[string]bool{}, edges: map[string]bool{}} }

func (g *graph) hash() string {
	var keys []string
	for m := range g.members {
		keys = append(keys, m)
	}
	for e := range g.edges {
		keys = append(keys, e)
	}
	sort.Strings(keys)
	return shortHash(strings.Join(keys, "\n"))
}

// chordGraph is a chord's order as its stubs hold it.
func (b *Board) chordGraph(chord *doc.Doc) *graph {
	g := newGraph()
	for _, s := range b.members[chord.ID()] {
		g.members[s.ID()] = true
	}
	for _, s := range b.members[chord.ID()] {
		for _, a := range b.After(s) {
			if g.members[a.ID()] {
				g.edges[a.ID()+">"+s.ID()] = true
			}
		}
	}
	return g
}

// canvas is a canvas file as generic JSON, so every key Obsidian or the user wrote stays.
type canvas struct {
	top   map[string]any
	nodes []map[string]any
	edges []map[string]any
}

func parseCanvas(data []byte) (*canvas, error) {
	c := &canvas{top: map[string]any{}}
	if len(bytes.TrimSpace(data)) == 0 {
		return c, nil
	}
	if err := json.Unmarshal(data, &c.top); err != nil {
		return nil, err
	}
	for _, key := range []string{"nodes", "edges"} {
		list, _ := c.top[key].([]any)
		for _, x := range list {
			if m, ok := x.(map[string]any); ok {
				if key == "nodes" {
					c.nodes = append(c.nodes, m)
				} else {
					c.edges = append(c.edges, m)
				}
			}
		}
	}
	return c, nil
}

func (c *canvas) render() []byte {
	top := map[string]any{}
	for k, v := range c.top {
		top[k] = v
	}
	nodes := make([]any, len(c.nodes))
	for i, n := range c.nodes {
		nodes[i] = n
	}
	edges := make([]any, len(c.edges))
	for i, e := range c.edges {
		edges[i] = e
	}
	top["nodes"], top["edges"] = nodes, edges
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "\t")
	enc.Encode(top)
	return buf.Bytes()
}

func str(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func num(m map[string]any, key string) float64 {
	f, _ := m[key].(float64)
	return f
}

// stubOf is the stub a canvas node shows, or nil: a file node whose file is a stub, or
// whose id is a stub's when the file moved.
func (b *Board) stubOf(node map[string]any) *doc.Doc {
	if str(node, "type") != "file" {
		return nil
	}
	if d := b.Idx.ByPath(str(node, "file")); d != nil && d.Type() == "stub" && b.Idx.ByID(d.ID()) == d {
		return d
	}
	if d := b.Idx.ByID(str(node, "id")); d != nil && d.Type() == "stub" {
		return d
	}
	return nil
}

// canvasGraph is the order a canvas shows, and the stub of each of its nodes by node id.
func (b *Board) canvasGraph(c *canvas) (*graph, map[string]*doc.Doc) {
	g := newGraph()
	byNode := map[string]*doc.Doc{}
	for _, n := range c.nodes {
		if s := b.stubOf(n); s != nil {
			byNode[str(n, "id")] = s
			g.members[s.ID()] = true
		}
	}
	for _, e := range c.edges {
		from, to := byNode[str(e, "fromNode")], byNode[str(e, "toNode")]
		if from != nil && to != nil && from.ID() != to.ID() {
			g.edges[from.ID()+">"+to.ID()] = true
		}
	}
	return g, byNode
}

func (b *Board) readCanvas(chord *doc.Doc) (*canvas, bool, error) {
	data, err := b.Idx.V.Read(CanvasPath(chord))
	if err != nil {
		return &canvas{top: map[string]any{}}, false, nil
	}
	c, err := parseCanvas(data)
	if err != nil {
		return nil, true, fmt.Errorf("%s is no canvas: %w", CanvasPath(chord), err)
	}
	return c, true, nil
}

// paint sets a stub's node to the stub's file and its status color.
func (b *Board) paint(node map[string]any, s *doc.Doc) {
	node["file"] = s.Path
	if color := b.CardColor(s); color == "" {
		delete(node, "color")
	} else {
		node["color"] = color
	}
}

// syncCanvas brings a chord's canvas up to date and records the hash of its graph. When
// the user changed the cards or the arrows since code wrote them, it leaves the graph
// alone and refreshes only the files and the colors, unless ForceCanvas names the chord.
func (b *Board) syncCanvas(chord *doc.Doc, write WriteFunc) (string, bool, error) {
	force, tidy := b.ForceCanvas[chord.ID()], b.TidyCanvas[chord.ID()]
	rel := CanvasPath(chord)
	c, exists, err := b.readCanvas(chord)
	if err != nil {
		// A canvas that does not parse is the user's to repair; the chord still syncs.
		b.canvas[chord.ID()] = chord.Str("canvas")
		return rel, false, nil
	}
	want := b.chordGraph(chord)
	if !exists && len(want.members) == 0 {
		return rel, false, nil
	}
	have, byNode := b.canvasGraph(c)
	before := c.render()
	ours := !exists || force || tidy || have.hash() == chord.Str("canvas") || have.hash() == want.hash()
	if !ours {
		on := map[string]bool{}
		for _, n := range c.nodes {
			if s := byNode[str(n, "id")]; s != nil {
				b.paint(n, s)
				on[s.ID()] = true
			}
		}
		// A thread that joined in this write gets its card; the user's arrows stay.
		for _, s := range b.Members(chord) {
			if b.Joined[s.ID()] && !on[s.ID()] {
				n := map[string]any{"id": s.ID(), "type": "file"}
				b.paint(n, s)
				c.nodes = append(c.nodes, n)
				place(c.nodes, n, b.Rank(s))
			}
		}
		b.canvas[chord.ID()] = chord.Str("canvas")
	} else {
		b.layout(chord, c, byNode, tidy)
		b.canvas[chord.ID()] = want.hash()
	}
	dropLegend(c)
	after := c.render()
	if exists && sameJSON(before, after) {
		return rel, false, nil
	}
	wrote, err := write(rel, after)
	return rel, wrote, err
}

func sameJSON(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}

// layout makes a canvas show the chord's order: one node per thread, one arrow per after
// link between threads of the chord. A node that exists keeps its place and size, unless
// tidy places every card again. Nodes and arrows that are not about stubs stay.
func (b *Board) layout(chord *doc.Doc, c *canvas, byNode map[string]*doc.Doc, tidy bool) {
	members := b.Members(chord)
	isMember := map[string]bool{}
	for _, s := range members {
		isMember[s.ID()] = true
	}
	nodeOf := map[string]map[string]any{} // stub id → its node
	var nodes []map[string]any
	for _, n := range c.nodes {
		s := byNode[str(n, "id")]
		switch {
		case s == nil:
			nodes = append(nodes, n)
		case isMember[s.ID()] && nodeOf[s.ID()] == nil:
			nodeOf[s.ID()] = n
			nodes = append(nodes, n)
		}
	}
	if tidy {
		for _, n := range nodeOf {
			delete(n, "x")
			delete(n, "y")
		}
	}
	for _, s := range members {
		n := nodeOf[s.ID()]
		if n == nil {
			n = map[string]any{"id": s.ID(), "type": "file"}
			nodeOf[s.ID()] = n
			nodes = append(nodes, n)
		}
		b.paint(n, s)
		if _, placed := n["x"]; placed {
			continue
		}
		place(nodes, n, b.Rank(s))
		if tidy {
			n["width"], n["height"] = float64(CardWidth), float64(CardHeight)
		}
	}
	c.nodes = nodes
	nodeID := func(stub string) string { return str(nodeOf[stub], "id") }
	want := b.chordGraph(chord).edges
	var edges []map[string]any
	seen := map[string]bool{}
	for _, e := range c.edges {
		from, to := byNode[str(e, "fromNode")], byNode[str(e, "toNode")]
		if from == nil || to == nil {
			// An arrow that touches a node of the user's is the user's.
			if (from == nil || isMember[from.ID()]) && (to == nil || isMember[to.ID()]) {
				edges = append(edges, e)
			}
			continue
		}
		key := from.ID() + ">" + to.ID()
		if want[key] && !seen[key] {
			seen[key] = true
			e["fromNode"], e["toNode"] = nodeID(from.ID()), nodeID(to.ID())
			edges = append(edges, e)
		}
	}
	var missing []string
	for key := range want {
		if !seen[key] {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	for _, key := range missing {
		from, to, _ := strings.Cut(key, ">")
		edges = append(edges, map[string]any{"id": "after-" + from + "-" + to, "fromNode": nodeID(from), "fromSide": "right", "toNode": nodeID(to), "toSide": "left"})
	}
	c.edges = edges
}

// place puts a card with no place in its rank's column, at the first free spot from the
// top, and gives it the card's size when it has none.
func place(nodes []map[string]any, n map[string]any, rank int) {
	taken := func(x, y float64) bool {
		for _, o := range nodes {
			if _, placed := o["x"]; !placed {
				continue
			}
			if x < num(o, "x")+num(o, "width") && num(o, "x") < x+CardWidth && y < num(o, "y")+num(o, "height") && num(o, "y") < y+CardHeight {
				return true
			}
		}
		return false
	}
	x := float64(rank * (CardWidth + CardGapX))
	y := 0.0
	for taken(x, y) {
		y += CardHeight + CardGapY
	}
	n["x"], n["y"] = x, y
	if _, sized := n["width"]; !sized {
		n["width"], n["height"] = float64(CardWidth), float64(CardHeight)
	}
}

// LegendPrefix marks the legend groups that 8.0.2 to 8.1.0 put on a canvas. The legend
// is in Obsidian's canvas bar now; a write removes the groups.
const LegendPrefix = "atlas-legend-"

func isLegend(n map[string]any) bool { return strings.HasPrefix(str(n, "id"), LegendPrefix) }

// dropLegend removes the legend groups of an older canvas, and the arrows that touch one.
func dropLegend(c *canvas) {
	legend := map[string]bool{}
	nodes := c.nodes[:0]
	for _, n := range c.nodes {
		if isLegend(n) {
			legend[str(n, "id")] = true
		} else {
			nodes = append(nodes, n)
		}
	}
	c.nodes = nodes
	if len(legend) == 0 {
		return
	}
	edges := c.edges[:0]
	for _, e := range c.edges {
		if !legend[str(e, "fromNode")] && !legend[str(e, "toNode")] {
			edges = append(edges, e)
		}
	}
	c.edges = edges
}

// CanvasState says whether a chord's canvas shows the order its stubs hold.
type CanvasState struct {
	Chord   vault.Ref `json:"chord"`
	Path    string    `json:"path"`
	Exists  bool      `json:"exists"`
	Differs bool      `json:"differs"`
	// Threads are the titles of the threads whose place saving the canvas would change.
	Threads []string `json:"threads"`
	// Changes are what saving the canvas would change, one line each.
	Changes []string `json:"changes"`
}

// CanvasStatus compares a chord's canvas with its stubs.
func CanvasStatus(idx *vault.Index, key string) (*CanvasState, error) {
	chord, err := idx.ResolveType(key, "chord")
	if err != nil {
		return nil, err
	}
	b := Load(idx)
	out := &CanvasState{Chord: b.Ref(chord), Path: CanvasPath(chord), Threads: []string{}, Changes: []string{}}
	c, exists, err := b.readCanvas(chord)
	if err != nil {
		return nil, err
	}
	out.Exists = exists
	if !exists {
		return out, nil
	}
	have, _ := b.canvasGraph(c)
	want := b.chordGraph(chord)
	out.Changes = b.graphChanges(want, have)
	out.Threads = b.moved(want, have)
	out.Differs = len(out.Changes) > 0
	return out, nil
}

// moved are the titles of the threads whose place differs between two orders: a thread
// that joins or leaves, and a thread whose arrows in differ.
func (b *Board) moved(from, to *graph) []string {
	ids := map[string]bool{}
	for m := range to.members {
		if !from.members[m] {
			ids[m] = true
		}
	}
	for m := range from.members {
		if !to.members[m] {
			ids[m] = true
		}
	}
	diff := func(x, y *graph) {
		for e := range x.edges {
			a, c, _ := strings.Cut(e, ">")
			if !y.edges[e] && to.members[a] && to.members[c] {
				ids[c] = true
			}
		}
	}
	diff(to, from)
	diff(from, to)
	out := []string{}
	for id := range ids {
		if d := b.Idx.ByID(id); d != nil {
			out = append(out, d.Title())
		}
	}
	sort.Strings(out)
	return out
}

// graphChanges lists what turns the order from into the order to.
func (b *Board) graphChanges(from, to *graph) []string {
	name := func(id string) string {
		if d := b.Idx.ByID(id); d != nil {
			return d.Title()
		}
		return id
	}
	var out []string
	for m := range to.members {
		if !from.members[m] {
			out = append(out, "add the thread "+name(m))
		}
	}
	for m := range from.members {
		if !to.members[m] {
			out = append(out, "remove the thread "+name(m))
		}
	}
	edge := func(key string) string {
		a, c, _ := strings.Cut(key, ">")
		return name(c) + " after " + name(a)
	}
	for e := range to.edges {
		if !from.edges[e] {
			out = append(out, "put "+edge(e))
		}
	}
	for e := range from.edges {
		if !to.edges[e] {
			a, c, _ := strings.Cut(e, ">")
			if to.members[a] && to.members[c] {
				out = append(out, "no longer put "+edge(e))
			}
		}
	}
	sort.Strings(out)
	return out
}
