package threads_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/sessions"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/threads"
)

func ok(t *testing.T) func(*threads.Result, error) *threads.Result {
	return func(r *threads.Result, err error) *threads.Result {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
}

func TestAThreadFromStubToReceipt(t *testing.T) {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", nil)
	tv.Page("repository", "p3-edge", map[string]any{"path": repo}, "")
	tv.Write("inbox/alarms.md", "fix the vehicle false alarms\n")
	tv.Commit()

	r := ok(t)(threads.Open(tv.V, threads.OpenIn{Text: "Vehicles trip false alarms at night.", Title: "Filter vehicle false alarms", Scope: []string{"p3-edge"}, Priority: "high", Inbox: "alarms.md"}, tv.Tick(time.Minute)))
	if r.View.Stub.State["stage"] != "stub" || r.View.Next != "spec" || r.Commit == "" {
		t.Fatalf("open: %+v", r.View)
	}
	if tv.V.Exists("inbox/alarms.md") {
		t.Fatal("the inbox note leaves in the same commit")
	}
	stub := tv.Read("threads/p3-edge/Filter vehicle false alarms/Filter vehicle false alarms.md")
	for _, want := range []string{`scope: ["[[p3-edge]]"]`, "priority: high", "> [!stub] Filter vehicle false alarms", "**Stub** → Spec → Tasks → Receipt", "## Stub\n\nVehicles trip false alarms at night."} {
		if !strings.Contains(stub, want) {
			t.Errorf("stub lacks %q:\n%s", want, stub)
		}
	}
	if tv.Log()[0] != "thread: open Filter vehicle false alarms" {
		t.Fatalf("log %v", tv.Log())
	}

	r = ok(t)(threads.File(tv.V, "Filter vehicle false alarms", "spec", "## Goal\n\nNo false alarms from vehicles.\n\n## Done when\n\n- a test passes", "", tv.Tick(time.Minute)))
	if r.View.Spec == nil || r.View.Next != "tasks" {
		t.Fatalf("spec: %+v", r.View)
	}
	if _, err := threads.File(tv.V, r.View.Stub.ID, "spec", "again", "", testvault.Now); err == nil {
		t.Fatal("a thread has one spec")
	}

	r = ok(t)(threads.Tasks(tv.V, r.View.Stub.ID, []threads.TaskIn{
		{Title: "Score boxes by motion", Text: "## What\n\nScore.\n\n## Verify\n\n- go test", Repository: "p3-edge"},
		{Title: "Tune the threshold", Text: "## What\n\nTune.", Repository: "p3-edge", Depends: []string{"T1"}},
	}, tv.Tick(time.Minute)))
	if len(r.View.Tasks) != 2 || r.View.Next != "task T1" || r.View.Tasks[1].Ready {
		t.Fatalf("tasks: %+v", r.View)
	}
	t2 := tv.Read("threads/p3-edge/Filter vehicle false alarms/Filter vehicle false alarms — T2 Tune the threshold.md")
	if !strings.Contains(t2, `depends: ["[[Filter vehicle false alarms — T1 Score boxes by motion]]"]`) || !strings.Contains(t2, "## Progress") || !strings.Contains(t2, "after [[Filter vehicle false alarms — T1 Score boxes by motion|T1]]") {
		t.Fatalf("T2:\n%s", t2)
	}
	if _, err := threads.Task(tv.V, "T2", r.View.Stub.ID, threads.TaskDo{Do: "start"}, testvault.Now); err == nil || !strings.Contains(err.Error(), "call thread task T1 done first") {
		t.Fatalf("a refusal teaches: %v", err)
	}
	if _, err := threads.File(tv.V, r.View.Stub.ID, "receipt", "Delivered.", "completed", testvault.Now); err == nil || !strings.Contains(err.Error(), "T1 Score boxes by motion, T2 Tune the threshold still open") {
		t.Fatalf("a completed receipt waits: %v", err)
	}
	ok(t)(threads.Task(tv.V, "T1", r.View.Stub.ID, threads.TaskDo{Do: "done", Result: "Scored. Commit abc123. go test passes."}, tv.Tick(time.Minute)))
	r = ok(t)(threads.Task(tv.V, "T2", r.View.Stub.ID, threads.TaskDo{Do: "drop"}, tv.Tick(time.Minute)))
	if r.View.Next != "receipt" || r.View.Stub.State["tasks"] != "1/1" {
		t.Fatalf("next: %+v", r.View)
	}
	r = ok(t)(threads.File(tv.V, r.View.Stub.ID, "receipt", "## Delivered\n\nMotion scoring.", "completed", tv.Tick(time.Minute)))
	if r.View.Stub.State["stage"] != "closed" || r.View.Stub.State["outcome"] != "completed" || r.View.Next != "none" {
		t.Fatalf("closed: %+v", r.View.Stub.State)
	}
	if _, err := threads.Tasks(tv.V, r.View.Stub.ID, []threads.TaskIn{{Title: "More"}}, testvault.Now); err != threads.ErrClosed {
		t.Fatalf("a closed thread takes no document: %v", err)
	}
	r = ok(t)(threads.Reopen(tv.V, r.View.Stub.ID, tv.Tick(time.Minute)))
	if r.View.Receipt != nil || r.View.Stub.State["stage"] != "tasks" {
		t.Fatalf("reopened: %+v", r.View)
	}
	if !tv.V.Exists("threads/p3-edge/Filter vehicle false alarms/Filter vehicle false alarms — Receipt (reopened 2026-09-27).md") {
		t.Fatal("the receipt is kept, renamed")
	}
	tv.Clean()
}

func TestKilledReceiptDropsOpenTasks(t *testing.T) {
	tv := testvault.New(t)
	r := ok(t)(threads.Open(tv.V, threads.OpenIn{Text: "An idea"}, testvault.Now))
	ok(t)(threads.Tasks(tv.V, r.View.Stub.ID, []threads.TaskIn{{Title: "Try it", Text: "x"}}, testvault.Now))
	r = ok(t)(threads.File(tv.V, r.View.Stub.ID, "receipt", "## Why killed\n\nNot worth it.", "killed", testvault.Now))
	if r.View.Tasks[0].Ref.State["status"] != "dropped" {
		t.Fatalf("killed: %+v", r.View.Tasks)
	}
	if !strings.Contains(tv.Read(r.View.Receipt.Path), "> [!killed] An idea · killed") {
		t.Fatalf("receipt:\n%s", tv.Read(r.View.Receipt.Path))
	}
}

func TestRenameMovesEveryDocumentAndLink(t *testing.T) {
	tv := testvault.New(t)
	r := ok(t)(threads.Open(tv.V, threads.OpenIn{Text: "Do the thing", Title: "Old name"}, testvault.Now))
	ok(t)(threads.File(tv.V, r.View.Stub.ID, "spec", "## Goal\n\nSee [[Old name]].", "", testvault.Now))
	ok(t)(threads.Tasks(tv.V, r.View.Stub.ID, []threads.TaskIn{{Title: "A", Text: "x"}, {Title: "B", Text: "x", Depends: []string{"A"}}}, testvault.Now))
	tv.Page("concept", "Linked", nil, "From [[Old name — Spec]].\n")
	tv.Commit()
	title := "New name"
	r = ok(t)(threads.Set(tv.V, r.View.Stub.ID, threads.SetIn{Title: &title}, testvault.Now))
	if r.View.Stub.Title != "New name" || r.View.Spec.Title != "New name — Spec" || r.View.Tasks[1].Ref.Title != "New name — T2 B" {
		t.Fatalf("renamed: %+v", r.View)
	}
	if tv.V.Exists("threads/Old name") {
		t.Fatal("the old folder is gone")
	}
	b := tv.Read("threads/New name/New name — T2 B.md")
	if !strings.Contains(b, `thread: "[[New name]]"`) || !strings.Contains(b, `depends: ["[[New name — T1 A]]"]`) {
		t.Fatalf("T2:\n%s", b)
	}
	if !strings.Contains(tv.Read("wiki/concepts/Linked.md"), "[[New name — Spec]]") {
		t.Fatal("a wiki page's link follows in the rename's commit")
	}
	tv.Clean()
	if !strings.HasPrefix(tv.Log()[0], "thread: rename Old name to New name") {
		t.Fatalf("log %v", tv.Log())
	}
}

func TestActiveComesFromLiveSessions(t *testing.T) {
	tv := testvault.New(t)
	r := ok(t)(threads.Open(tv.V, threads.OpenIn{Text: "Work", Title: "Work item"}, testvault.Now))
	ok(t)(threads.Tasks(tv.V, r.View.Stub.ID, []threads.TaskIn{{Title: "First", Text: "x"}}, testvault.Now))
	unlock, _ := tv.V.Lock()
	e := sessions.Event{SessionID: "a1b2c3d4-0000", Cwd: tv.V.Root}
	if _, err := sessions.Start(tv.V, e, testvault.Now); err != nil {
		t.Fatal(err)
	}
	if _, err := sessions.Touch(tv.V, e, testvault.Now, func(c string) string {
		return sessions.AddLink(sessions.AddLink(c, "threads", "Work item"), "tasks", "Work item — T1 First")
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := threads.SyncVault(tv.V); err != nil {
		t.Fatal(err)
	}
	unlock()
	stub := tv.Read(r.View.Stub.Path)
	if !strings.Contains(stub, "active: true") || !strings.Contains(stub, "active in [[2026-09-27 1432 a1b2c3]]") {
		t.Fatalf("stub:\n%s", stub)
	}
	if _, err := threads.Task(tv.V, "T1", r.View.Stub.ID, threads.TaskDo{Do: "start"}, testvault.Now); err == nil {
		t.Fatal("a task a live session holds needs take")
	}
	if _, err := threads.Task(tv.V, "T1", r.View.Stub.ID, threads.TaskDo{Do: "start", Take: true}, testvault.Now); err != nil {
		t.Fatal(err)
	}
	session := tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md")
	if !strings.Contains(session, "> [!session] running · T1 of [[Work item]]") {
		t.Fatalf("session:\n%s", session)
	}
	unlock, _ = tv.V.Lock()
	lost, _ := sessions.MarkLost(tv.V, testvault.Now.Add(13*time.Hour), 12)
	threads.SyncVault(tv.V)
	unlock()
	if len(lost) != 1 || !strings.Contains(tv.Read(r.View.Stub.Path), "active: false") {
		t.Fatalf("a lost session lets the thread go: %v", lost)
	}
}

func TestSyncFollowsAHandEdit(t *testing.T) {
	tv := testvault.New(t)
	r := ok(t)(threads.Open(tv.V, threads.OpenIn{Text: "x", Title: "Thread"}, testvault.Now))
	r = ok(t)(threads.File(tv.V, r.View.Stub.ID, "spec", "## Goal\n\ny", "", testvault.Now))
	// The user deletes the spec in Obsidian.
	if err := tv.V.Remove(r.View.Spec.Path); err != nil {
		t.Fatal(err)
	}
	// The stub moves back a stage, and its card on the canvas takes the stub's color.
	wrote, err := threads.SyncVault(tv.V)
	if err != nil || strings.Join(wrote, " ") != r.View.Stub.Path+" threads/Threads.canvas" {
		t.Fatalf("sync %v %v", wrote, err)
	}
	stub := tv.Read(r.View.Stub.Path)
	if !strings.Contains(stub, "stage: stub") || !strings.Contains(stub, "updated: 2026-09-27") {
		t.Fatalf("the thread moves back:\n%s", stub)
	}
	if again, _ := threads.SyncVault(tv.V); len(again) != 0 {
		t.Fatal("a second sync writes nothing")
	}
	// A callout of the user's stays.
	tv.Write(r.View.Stub.Path, strings.Replace(stub, "## Stub", "> [!note] mine\n\n## Stub", 1))
	threads.SyncVault(tv.V)
	if !strings.Contains(tv.Read(r.View.Stub.Path), "> [!note] mine") {
		t.Fatal("sync keeps the user's callout")
	}
	d := doc.Parse("", []byte(tv.Read(r.View.Stub.Path)))
	if doc.LeadType(d.Body) != "stub" {
		t.Fatal("the lead stays first")
	}
}

func TestTaskSetRenamesAndRefusesLoops(t *testing.T) {
	tv := testvault.New(t)
	r := ok(t)(threads.Open(tv.V, threads.OpenIn{Text: "x", Title: "T"}, testvault.Now))
	ok(t)(threads.Tasks(tv.V, r.View.Stub.ID, []threads.TaskIn{{Title: "A", Text: "x"}, {Title: "B", Text: "x", Depends: []string{"T1"}}}, testvault.Now))
	deps := []string{"T2"}
	if _, err := threads.Task(tv.V, "T1", r.View.Stub.ID, threads.TaskDo{Do: "set", Depends: &deps}, testvault.Now); err == nil || !strings.Contains(err.Error(), "loop") {
		t.Fatalf("a loop: %v", err)
	}
	title, order := "Alpha", 5
	r = ok(t)(threads.Task(tv.V, "T1", r.View.Stub.ID, threads.TaskDo{Do: "set", Title: &title, Order: &order}, testvault.Now))
	if r.View.Tasks[1].Ref.Title != "T — T5 Alpha" {
		t.Fatalf("renamed: %+v", r.View.Tasks)
	}
	if !strings.Contains(tv.Read("threads/T/T — T2 B.md"), `depends: ["[[T — T5 Alpha]]"]`) {
		t.Fatal("the dependency follows the rename")
	}
}
