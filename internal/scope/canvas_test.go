package scope_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/change"
	"github.com/nathanaday/atlas-obsidian/internal/core"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/threads"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

type node struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	File   string `json:"file,omitempty"`
	Label  string `json:"label,omitempty"`
	Text   string `json:"text,omitempty"`
	Color  string `json:"color,omitempty"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type edge struct {
	ID       string `json:"id"`
	FromNode string `json:"fromNode"`
	ToNode   string `json:"toNode"`
}

type board struct {
	Nodes []node `json:"nodes"`
	Edges []edge `json:"edges"`
}

func readBoard(t *testing.T, tv *testvault.T) board {
	t.Helper()
	var b board
	if err := json.Unmarshal([]byte(tv.Read(vault.ThreadsCanvas)), &b); err != nil {
		t.Fatal(err)
	}
	return b
}

func writeBoard(t *testing.T, tv *testvault.T, b board) {
	t.Helper()
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	tv.Write(vault.ThreadsCanvas, string(data))
}

func (b board) node(id string) *node {
	for i := range b.Nodes {
		if b.Nodes[i].ID == id {
			return &b.Nodes[i]
		}
	}
	return nil
}

// inside is the group whose box holds the node's corner.
func (b board) inside(n *node) string {
	for _, g := range b.Nodes {
		if g.Type == "group" && n.X >= g.X && n.X < g.X+g.Width && n.Y >= g.Y && n.Y < g.Y+g.Height {
			return g.Label
		}
	}
	return ""
}

func (b board) groups() []string {
	var out []string
	for _, n := range b.Nodes {
		if n.Type == "group" {
			out = append(out, n.Label)
		}
	}
	return out
}

func open(t *testing.T, tv *testvault.T, title, priority string, scope ...string) string {
	t.Helper()
	r, err := threads.Open(tv.V, threads.OpenIn{Text: title, Title: title, Scope: scope, Priority: priority}, tv.Tick(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	return r.View.Stub.ID
}

func scopes(t *testing.T, tv *testvault.T) {
	t.Helper()
	repo := tv.Repo("p3-edge", nil)
	tv.Page("area", "work", nil, "")
	tv.Page("area", "p3", map[string]any{"parent": "[[work]]"}, "")
	tv.Page("repository", "p3-edge", map[string]any{"parent": "[[p3]]", "path": repo}, "")
	tv.Page("area", "ml", nil, "")
	if _, err := core.Sync(tv.V, testvault.Now); err != nil {
		t.Fatal(err)
	}
	tv.Commit()
}

func TestThreadsCanvasGroupsOpenThreadsByTheirFirstScope(t *testing.T) {
	tv := testvault.New(t)
	if got := tv.Read(vault.ThreadsCanvas); got != vault.EmptyCanvas {
		t.Fatalf("init writes an empty canvas:\n%s", got)
	}
	scopes(t, tv)
	low := open(t, tv, "Tune the tracker", "low", "p3-edge")
	high := open(t, tv, "Score boxes by motion", "high", "p3-edge")
	both := open(t, tv, "Train a motion model", "", "ml", "p3")
	loose := open(t, tv, "Tidy the notes", "")
	dead := open(t, tv, "Drop the old scorer", "", "p3-edge")
	if _, err := threads.File(tv.V, dead, "receipt", "## Delivered\n\nNothing.", "killed", tv.Tick(time.Minute)); err != nil {
		t.Fatal(err)
	}

	b := readBoard(t, tv)
	if got := strings.Join(b.groups(), " | "); got != "ml | work / p3 / p3-edge | No area" {
		t.Fatalf("groups: %s", got)
	}
	for id, want := range map[string]string{low: "work / p3 / p3-edge", high: "work / p3 / p3-edge", both: "ml", loose: "No area"} {
		n := b.node(id)
		if n == nil || n.Type != "file" || !strings.HasPrefix(n.File, "threads/") || n.Color != "3" {
			t.Fatalf("card %s: %+v", id, n)
		}
		if got := b.inside(n); got != want {
			t.Errorf("card %s sits in %q, want %q", n.File, got, want)
		}
	}
	if b.node(dead) != nil {
		t.Error("a closed thread has no card")
	}
	if h, l := b.node(high), b.node(low); h.Y != l.Y || h.X > l.X {
		t.Errorf("the higher priority comes first: %+v %+v", h, l)
	}
	if got := tv.Read(threadPath(t, tv, both)); !strings.Contains(got, `chain: ["[[ml]]", "[[work]]", "[[p3]]"]`) {
		t.Fatalf("a thread's chain holds each scope and the areas above it:\n%s", got)
	}
	tv.Clean()
}

func threadPath(t *testing.T, tv *testvault.T, id string) string {
	t.Helper()
	d := tv.Index().ByID(id)
	if d == nil {
		t.Fatalf("no document %s", id)
	}
	return d.Path
}

func TestThreadsCanvasKeepsTheUsersMovesAndEdges(t *testing.T) {
	tv := testvault.New(t)
	scopes(t, tv)
	a := open(t, tv, "Score boxes by motion", "high", "p3-edge")
	c := open(t, tv, "Tune the tracker", "", "p3-edge")
	d := open(t, tv, "Train a motion model", "", "ml")

	// In Obsidian the user drags the ml group with its card, moves one card off the grid,
	// adds a note, and wires two cards.
	b := readBoard(t, tv)
	for i := range b.Nodes {
		n := &b.Nodes[i]
		if n.Label == "ml" || n.ID == d {
			n.X += 500
			n.Y += 300
		}
		if n.ID == c {
			n.X += 13
			n.Y += 700
		}
	}
	b.Nodes = append(b.Nodes, node{ID: "5f2a9c0e1b3d4a6f", Type: "text", Text: "Order of work", X: -600, Y: 0, Width: 250, Height: 60})
	b.Edges = []edge{{ID: "e1", FromNode: a, ToNode: c}, {ID: "e2", FromNode: c, ToNode: d}}
	writeBoard(t, tv, b)
	tv.Commit()
	moved := *b.node(c)
	mlCard := *b.node(d)

	e := open(t, tv, "Label the clips", "", "ml")
	b = readBoard(t, tv)
	if got := *b.node(c); got.X != moved.X || got.Y != moved.Y {
		t.Errorf("a card the user moved stays: %+v, want %+v", got, moved)
	}
	if got := *b.node(d); got.X != mlCard.X || got.Y != mlCard.Y {
		t.Errorf("a card that moved with its group stays on its slot: %+v, want %+v", got, mlCard)
	}
	if got := b.inside(b.node(e)); got != "ml" {
		t.Errorf("a new card joins its group where the user put it: %q", got)
	}
	if b.node("5f2a9c0e1b3d4a6f") == nil || len(b.Edges) != 2 {
		t.Fatalf("the user's note and edges stay: %+v", b.Edges)
	}
	if b.node(a).X == moved.X {
		t.Error("the code's cards stay on the grid")
	}

	// Obsidian saves the canvas in its own layout; that is no change.
	data, _ := json.MarshalIndent(b, "", "  ")
	tv.Write(vault.ThreadsCanvas, string(data))
	tv.Commit()
	unlock, _ := tv.V.Lock()
	wrote, err := threads.SyncVault(tv.V)
	unlock()
	if err != nil || len(wrote) != 0 {
		t.Fatalf("sync rewrites an unchanged canvas: %v %v", wrote, err)
	}

	// A closed thread takes its card and its edges with it.
	if _, err := threads.File(tv.V, c, "receipt", "## Delivered\n\nDone.", "killed", tv.Tick(time.Minute)); err != nil {
		t.Fatal(err)
	}
	b = readBoard(t, tv)
	if b.node(c) != nil || len(b.Edges) != 0 {
		t.Fatalf("closing drops the card and its edges: %+v", b.Edges)
	}
}

func TestARenamedAreaRelabelsItsGroupInTheSameCommit(t *testing.T) {
	tv := testvault.New(t)
	scopes(t, tv)
	open(t, tv, "Score boxes by motion", "", "p3-edge")
	pv, err := change.Propose(tv.V, change.Plan{Title: "Rename p3", Writes: []change.Write{{Op: "rename", ID: tv.ID("p3"), Title: "p3 product"}}}, tv.Tick(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := change.Apply(tv.V, pv.Ref.ID, tv.Tick(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(readBoard(t, tv).groups(), " | "); got != "work / p3 product / p3-edge" {
		t.Fatalf("groups: %s", got)
	}
	tv.Clean()
}
