package change_test

import (
	"os"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/change"
	"github.com/nathanaday/atlas-obsidian/internal/core"
	"github.com/nathanaday/atlas-obsidian/internal/lint"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
)

// course is a vault with one area, a sub-area, and pages in both, synced and committed.
func course(t *testing.T) *testvault.T {
	t.Helper()
	tv := testvault.New(t)
	tv.Page("area", "CS566", nil, "")
	tv.Page("area", "Transformers", map[string]any{"parent": "[[CS566]]"}, "")
	tv.Page("concept", "Backprop", map[string]any{"scope": "[[CS566]]", "sources": []string{"[[Attention]]"}}, "")
	tv.Page("concept", "Attention", map[string]any{"scope": "[[Transformers]]", "sources": []string{"[[Backprop]]"}}, "See [[Backprop]].\n")
	tv.Write("wiki/CS566/Lecture notes.md", "my own note, in the area's folder\n")
	tv.Write("wiki/CS566/Transformers/diagram.png", "png")
	if _, err := core.Sync(tv.V, testvault.Now); err != nil {
		t.Fatal(err)
	}
	tv.Commit()
	return tv
}

func TestAPageGoesInTheFolderOfItsScope(t *testing.T) {
	tv := course(t)
	pv := propose(t, tv, change.Plan{Title: "Add pages", Writes: []change.Write{
		{Op: "create", Type: "entity", Title: "PyTorch", Fields: map[string]any{"scope": "Transformers", "description": "A library.", "kind": "tool"}},
		{Op: "create", Type: "area", Title: "Vision", Fields: map[string]any{"parent": "CS566", "description": "Vision models."}},
		{Op: "create", Type: "concept", Title: "ViT", Fields: map[string]any{"scope": "Vision", "description": "A vision transformer."}},
	}})
	want := []string{"wiki/CS566/Transformers/entities/PyTorch.md", "wiki/CS566/Vision/Vision.md", "wiki/CS566/Vision/concepts/ViT.md"}
	for i, w := range pv.Writes {
		if w.Path != want[i] {
			t.Errorf("write %d lands at %s, not %s", i, w.Path, want[i])
		}
	}
	apply(t, tv, pv.Ref.ID)
	for _, p := range want {
		if !tv.V.Exists(p) {
			t.Errorf("%s is missing", p)
		}
	}
	tv.Clean()
}

func TestAScopeChangeMovesThePage(t *testing.T) {
	tv := course(t)
	pv := propose(t, tv, change.Plan{Title: "Move backprop", Writes: []change.Write{{Op: "modify", ID: "Backprop", Fields: map[string]any{"scope": "Transformers"}}}})
	if w := pv.Writes[0]; w.Path != "wiki/CS566/Transformers/concepts/Backprop.md" || !strings.Contains(w.Note, "moves from wiki/CS566/concepts/Backprop.md") {
		t.Fatalf("preview %+v", w)
	}
	apply(t, tv, pv.Ref.ID)
	if tv.V.Exists("wiki/CS566/concepts") {
		t.Fatal("the emptied type folder goes")
	}
	page := tv.Read("wiki/CS566/Transformers/concepts/Backprop.md")
	if !strings.Contains(page, `scope: "[[Transformers]]"`) || !strings.Contains(page, `chain: ["[[CS566]]", "[[Transformers]]"]`) {
		t.Fatalf("page:\n%s", page)
	}
	tv.Clean()
}

func TestARenamedAreaMovesItsFolderAndUndoPutsItBack(t *testing.T) {
	tv := course(t)
	pv := propose(t, tv, change.Plan{Title: "Rename the course", Writes: []change.Write{{Op: "rename", ID: "CS566", Title: "CS566 Deep Learning"}}})
	if len(pv.Folders) != 1 || pv.Folders[0].From != "wiki/CS566" || pv.Folders[0].To != "wiki/CS566 Deep Learning" || pv.Folders[0].Files != 5 {
		t.Fatalf("folders %+v", pv.Folders)
	}
	if !strings.Contains(tv.Read(pv.Ref.Path), "### folder moves\n\n- `wiki/CS566` → `wiki/CS566 Deep Learning`: 5 files") {
		t.Fatalf("the change document lists the move:\n%s", tv.Read(pv.Ref.Path))
	}
	apply(t, tv, pv.Ref.ID)
	for _, p := range []string{
		"wiki/CS566 Deep Learning/CS566 Deep Learning.md",
		"wiki/CS566 Deep Learning/concepts/Backprop.md",
		"wiki/CS566 Deep Learning/Transformers/Transformers.md",
		"wiki/CS566 Deep Learning/Transformers/concepts/Attention.md",
		"wiki/CS566 Deep Learning/Transformers/diagram.png",
		"wiki/CS566 Deep Learning/Lecture notes.md",
	} {
		if !tv.V.Exists(p) {
			t.Errorf("%s is missing", p)
		}
	}
	if tv.V.Exists("wiki/CS566") {
		t.Fatal("the old folder goes")
	}
	if got := tv.Read("wiki/CS566 Deep Learning/Transformers/Transformers.md"); !strings.Contains(got, `parent: "[[CS566 Deep Learning]]"`) {
		t.Fatalf("the sub-area's parent follows:\n%s", got)
	}
	tv.Clean()
	if _, err := change.Undo(tv.V, pv.Ref.ID, testvault.Now); err != nil {
		t.Fatal(err)
	}
	if !tv.V.Exists("wiki/CS566/Transformers/diagram.png") || !tv.V.Exists("wiki/CS566/CS566.md") || tv.V.Exists("wiki/CS566 Deep Learning") {
		t.Fatal("undo puts every file back and leaves no empty folder")
	}
	tv.Clean()
}

func TestARemovedAreaEmptiesIntoItsParent(t *testing.T) {
	tv := course(t)
	pv := propose(t, tv, change.Plan{Title: "Fold transformers in", Writes: []change.Write{{Op: "remove", ID: "Transformers", Redirect: "CS566"}}})
	apply(t, tv, pv.Ref.ID)
	if tv.V.Exists("wiki/CS566/Transformers") || !tv.V.Exists("wiki/CS566/diagram.png") {
		t.Fatal("the folder empties into the parent's")
	}
	if got := tv.Read("wiki/CS566/concepts/Attention.md"); !strings.Contains(got, `scope: "[[CS566]]"`) {
		t.Fatalf("attention belongs to the course:\n%s", got)
	}
	tv.Clean()
}

func TestAMoveThatLoopsOrCollidesIsRefused(t *testing.T) {
	tv := course(t)
	_, err := change.Propose(tv.V, change.Plan{Title: "Loop", Writes: []change.Write{{Op: "modify", ID: "CS566", Fields: map[string]any{"parent": "Transformers"}}}}, testvault.Now)
	if err == nil || !strings.Contains(err.Error(), "loop") {
		t.Fatalf("a loop: %v", err)
	}
	tv.Write("wiki/Transformers/diagram.png", "in the way")
	tv.Commit()
	_, err = change.Propose(tv.V, change.Plan{Title: "Up", Writes: []change.Write{{Op: "modify", ID: "Transformers", Fields: map[string]any{"parent": nil}}}}, testvault.Now)
	if err == nil || !strings.Contains(err.Error(), "wiki/Transformers/diagram.png, which holds a file") {
		t.Fatalf("a collision: %v", err)
	}
	_, err = change.Propose(tv.V, change.Plan{Title: "Reserved", Writes: []change.Write{{Op: "create", Type: "area", Title: "Concepts", Fields: map[string]any{"description": "x"}}}}, testvault.Now)
	if err == nil || !strings.Contains(err.Error(), "name of a type folder") {
		t.Fatalf("a reserved title: %v", err)
	}
}

func TestAHandMoveIsAScopeChange(t *testing.T) {
	tv := course(t)
	if err := os.Rename(tv.V.Abs("wiki/CS566/concepts/Backprop.md"), tv.V.Abs("wiki/CS566/Transformers/concepts/Backprop.md")); err != nil {
		t.Fatal(err)
	}
	idx := tv.Index()
	d, _ := idx.Resolve("Backprop")
	if ids := idx.ScopeIDs(d); len(ids) != 1 || ids[0] != tv.ID("Transformers") {
		t.Fatalf("the folder gives the scope before any sync: %v", ids)
	}
	// A change on the page keeps it where the user put it.
	pv := propose(t, tv, change.Plan{Title: "Edit backprop", Writes: []change.Write{{Op: "modify", ID: "Backprop", Body: str("## Definition\n\nChain rule.\n")}}})
	if pv.Writes[0].Path != "wiki/CS566/Transformers/concepts/Backprop.md" {
		t.Fatalf("the page stays: %+v", pv.Writes[0])
	}
	apply(t, tv, pv.Ref.ID)
	if got := tv.Read("wiki/CS566/Transformers/concepts/Backprop.md"); !strings.Contains(got, `scope: "[[Transformers]]"`) {
		t.Fatalf("the field follows the folder:\n%s", got)
	}
	if f, _ := lint.Run(tv.Index(), lint.Options{Now: testvault.Now}); f.Counts[lint.Error] != 0 {
		t.Fatalf("lint: %+v", f.Findings)
	}
	tv.Clean()
}
