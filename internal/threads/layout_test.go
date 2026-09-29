package threads_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/change"
	"github.com/nathanaday/atlas-obsidian/internal/core"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/threads"
)

// areas is a vault with an area and a sub-area, synced and committed.
func areas(t *testing.T) *testvault.T {
	t.Helper()
	tv := testvault.New(t)
	tv.Page("area", "ML", nil, "")
	tv.Page("area", "CS566", map[string]any{"parent": "[[ML]]"}, "")
	if _, err := core.Sync(tv.V, testvault.Now); err != nil {
		t.Fatal(err)
	}
	tv.Commit()
	return tv
}

func scopes(s ...string) *[]string { return &s }

func TestAThreadIsFiledUnderItsFirstScope(t *testing.T) {
	tv := areas(t)
	r := ok(t)(threads.Open(tv.V, threads.OpenIn{Text: "Train the model.", Title: "Train it", Scope: []string{"CS566", "ML"}}, tv.Tick(time.Minute)))
	if r.View.Stub.Path != "threads/ML/CS566/Train it/Train it.md" {
		t.Fatalf("filed at %s", r.View.Stub.Path)
	}
	loose := ok(t)(threads.Open(tv.V, threads.OpenIn{Text: "Some idea.", Title: "Some idea"}, tv.Tick(time.Minute)))
	if loose.View.Stub.Path != "threads/Some idea/Some idea.md" {
		t.Fatalf("a thread with no scope stays at the top: %s", loose.View.Stub.Path)
	}
	canvas := tv.Read("threads/Threads.canvas")
	if !strings.Contains(canvas, `"label": "ML / CS566"`) || !strings.Contains(canvas, `"label": "No area"`) {
		t.Fatalf("canvas groups by home:\n%s", canvas)
	}
	tv.Clean()
}

func TestSetScopeFilesTheThreadLater(t *testing.T) {
	tv := areas(t)
	ok(t)(threads.Open(tv.V, threads.OpenIn{Text: "Some idea.", Title: "Some idea"}, tv.Tick(time.Minute)))
	tv.Write("threads/Some idea/sketch.png", "png")
	tv.Commit()
	r := ok(t)(threads.Set(tv.V, "Some idea", threads.SetIn{Scope: scopes("CS566")}, tv.Tick(time.Minute)))
	if r.View.Stub.Path != "threads/ML/CS566/Some idea/Some idea.md" || !tv.V.Exists("threads/ML/CS566/Some idea/sketch.png") {
		t.Fatalf("the folder moves with the user's files: %s", r.View.Stub.Path)
	}
	if tv.V.Exists("threads/Some idea") {
		t.Fatal("the old folder goes")
	}
	r = ok(t)(threads.Set(tv.V, "Some idea", threads.SetIn{Title: str("Better idea")}, tv.Tick(time.Minute)))
	if r.View.Stub.Path != "threads/ML/CS566/Better idea/Better idea.md" || !tv.V.Exists("threads/ML/CS566/Better idea/sketch.png") {
		t.Fatalf("a rename keeps the home: %s", r.View.Stub.Path)
	}
	r = ok(t)(threads.Set(tv.V, "Better idea", threads.SetIn{Scope: scopes()}, tv.Tick(time.Minute)))
	if r.View.Stub.Path != "threads/Better idea/Better idea.md" || tv.V.Exists("threads/ML") {
		t.Fatalf("no scope files it at the top again: %s", r.View.Stub.Path)
	}
	tv.Clean()
}

func TestAThreadFollowsItsAreaAndAHandMove(t *testing.T) {
	tv := areas(t)
	ok(t)(threads.Open(tv.V, threads.OpenIn{Text: "Train the model.", Title: "Train it", Scope: []string{"CS566"}}, tv.Tick(time.Minute)))
	pv, err := change.Propose(tv.V, change.Plan{Title: "Rename", Writes: []change.Write{{Op: "rename", ID: "CS566", Title: "Deep Learning"}}}, tv.Tick(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if pv.Folders[0].Files != 1 {
		t.Fatalf("the move counts the thread's files: %+v", pv.Folders)
	}
	if _, err := change.Apply(tv.V, pv.Ref.ID, tv.Tick(time.Minute), nil); err != nil {
		t.Fatal(err)
	}
	stub := tv.Read("threads/ML/Deep Learning/Train it/Train it.md")
	if !strings.Contains(stub, `scope: ["[[Deep Learning]]"]`) {
		t.Fatalf("the thread moves with its area:\n%s", stub)
	}
	tv.Clean()

	// The user drags the thread to the area above.
	if err := os.Rename(tv.V.Abs("threads/ML/Deep Learning/Train it"), tv.V.Abs("threads/ML/Train it")); err != nil {
		t.Fatal(err)
	}
	if _, err := core.Sync(tv.V, testvault.Now); err != nil {
		t.Fatal(err)
	}
	if got := tv.Read("threads/ML/Train it/Train it.md"); !strings.Contains(got, `scope: ["[[ML]]", "[[Deep Learning]]"]`) {
		t.Fatalf("the folder is the home, and the other scopes stay:\n%s", got)
	}

	// An area renamed in a shell leaves its mirror behind; sync files the thread again.
	tv.Commit()
	ok(t)(threads.Set(tv.V, "Train it", threads.SetIn{Scope: scopes("Deep Learning")}, tv.Tick(time.Minute)))
	os.Rename(tv.V.Abs("wiki/ML/Deep Learning"), tv.V.Abs("wiki/ML/DL"))
	os.Rename(tv.V.Abs("wiki/ML/DL/Deep Learning.md"), tv.V.Abs("wiki/ML/DL/DL.md"))
	tv.Write("threads/ML/Deep Learning/Train it/Train it.md", strings.Replace(tv.Read("threads/ML/Deep Learning/Train it/Train it.md"), "[[Deep Learning]]", "[[DL]]", 1))
	if _, err := core.Sync(tv.V, testvault.Now); err != nil {
		t.Fatal(err)
	}
	if !tv.V.Exists("threads/ML/DL/Train it/Train it.md") || tv.V.Exists("threads/ML/Deep Learning") {
		t.Fatal("sync refiles a thread whose folder stands for no scope")
	}
}
func str(s string) *string { return &s }

func TestADropOnAFolderIsEnough(t *testing.T) {
	tv := areas(t)
	ok(t)(threads.Open(tv.V, threads.OpenIn{Text: "Some idea.", Title: "Some idea"}, tv.Tick(time.Minute)))
	if !tv.V.Exists("threads/ML/CS566") {
		t.Fatal("each scope has a folder under threads/ to drop a thread on")
	}
	tv.Page("concept", "Dropped", nil, "")
	os.Rename(tv.V.Abs("wiki/concepts/Dropped.md"), tv.V.Abs("wiki/ML/CS566/Dropped.md"))
	os.Rename(tv.V.Abs("threads/Some idea"), tv.V.Abs("threads/ML/CS566/Some idea"))
	if _, err := core.Sync(tv.V, testvault.Now); err != nil {
		t.Fatal(err)
	}
	if got := tv.Read("wiki/ML/CS566/concepts/Dropped.md"); !strings.Contains(got, `scope: "[[CS566]]"`) || !strings.Contains(got, `chain: ["[[ML]]", "[[CS566]]"]`) {
		t.Fatalf("a page dropped on an area goes in its type's folder:\n%s", got)
	}
	if got := tv.Read("threads/ML/CS566/Some idea/Some idea.md"); !strings.Contains(got, `scope: ["[[CS566]]"]`) {
		t.Fatalf("a thread dropped on an area's folder is filed there:\n%s", got)
	}
}
