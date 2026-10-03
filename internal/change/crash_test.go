package change_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/change"
	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/thread"
)

// crashPlan proposes a change that modifies one topic and creates another.
func crashPlan(t *testing.T, tv *testvault.T) (*change.Preview, string) {
	t.Helper()
	id := tv.Doc("topic", "Motion scoring", map[string]any{"kind": "concept"}, "## Definition\n\nOld.\n")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Crash", Writes: []change.Write{{Op: "modify", ID: id, Body: str("## Definition\n\nNew.\n")}, {Op: "create", Type: "topic", Kind: "entity", Title: "Radar", Fields: map[string]any{"description": "A sensor."}}}})
	return pv, id
}

// nextWrite runs a write that starts with recovery.
func nextWrite(t *testing.T, tv *testvault.T) {
	t.Helper()
	if _, err := thread.Stub(tv.V, thread.StubIn{Text: "After the crash.", Title: "After"}, thread.Opts{Now: tv.Tick(time.Minute)}); err != nil {
		t.Fatal(err)
	}
}

func TestAnApplyWhoseCommitFailsStaysProposed(t *testing.T) {
	tv := testvault.New(t)
	pv, _ := crashPlan(t, tv)
	// With the proposal committed, the write starts clean and the lock bites at its commit.
	tv.Commit()
	proposed := tv.Read(pv.Ref.Path)
	topic := tv.Read("wiki/documents/Motion scoring.md")
	lock := filepath.Join(tv.V.Root, ".git", "index.lock")
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := change.Apply(tv.V, pv.Ref.ID, tv.Tick(time.Minute), nil)
	os.Remove(lock)
	if err == nil || !strings.Contains(err.Error(), "the vault is back as it was") {
		t.Fatalf("apply with the index locked: %v", err)
	}
	if tv.Read(pv.Ref.Path) != proposed || tv.Read("wiki/documents/Motion scoring.md") != topic || tv.V.Exists("wiki/documents/Radar.md") {
		t.Fatal("the failed apply left files changed")
	}
	apply(t, tv, pv.Ref.ID)
}

// A crash after the last write and before the commit: the document says applied and
// still holds paths, and no commit exists. Recovery puts the change back.
func TestACrashBeforeTheCommitIsPutBack(t *testing.T) {
	tv := testvault.New(t)
	pv, _ := crashPlan(t, tv)
	content := tv.Read(pv.Ref.Path)
	content = doc.SetField(doc.SetField(content, "status", "applied"), "paths", []string{"wiki/documents/Motion scoring.md", "wiki/documents/Radar.md"})
	tv.Write(pv.Ref.Path, content)
	tv.Write("wiki/documents/Motion scoring.md", "half written")
	tv.Write("wiki/documents/Radar.md", "half written")
	nextWrite(t, tv)
	if !strings.Contains(tv.Read("wiki/documents/Motion scoring.md"), "Old.") || tv.V.Exists("wiki/documents/Radar.md") {
		t.Fatal("recovery did not put the documents back")
	}
	got := tv.Read(pv.Ref.Path)
	if !strings.Contains(got, "status: proposed") || strings.Contains(got, "paths:") {
		t.Fatalf("the change is not proposed again:\n%s", got)
	}
	apply(t, tv, pv.Ref.ID)
}

// A crash after the commit and before the document's last write: the commit holds the
// applied document, and the file still holds paths. Recovery finishes the apply, and undo
// can reverse it.
func TestACrashAfterTheCommitIsFinished(t *testing.T) {
	tv := testvault.New(t)
	pv, _ := crashPlan(t, tv)
	apply(t, tv, pv.Ref.ID)
	applied := tv.Read(pv.Ref.Path)
	inFlight := doc.SetField(applied, "paths", []string{"wiki/documents/Motion scoring.md", "wiki/documents/Radar.md"})
	tv.Write(pv.Ref.Path, inFlight)
	nextWrite(t, tv)
	if got := tv.Read(pv.Ref.Path); got != applied {
		t.Fatalf("recovery did not finish the change:\n%s", got)
	}
	if !strings.Contains(tv.Read("wiki/documents/Motion scoring.md"), "New.") || !tv.V.Exists("wiki/documents/Radar.md") {
		t.Fatal("recovery took back the applied documents")
	}
	if _, err := change.Undo(tv.V, pv.Ref.ID, tv.Tick(time.Minute)); err != nil {
		t.Fatalf("undo after the finished apply: %v", err)
	}
}

func TestAnAppliedChangeHoldsNoPaths(t *testing.T) {
	tv := testvault.New(t)
	pv, _ := crashPlan(t, tv)
	apply(t, tv, pv.Ref.ID)
	if strings.Contains(tv.Read(pv.Ref.Path), "paths:") {
		t.Fatal("the applied document on disk keeps paths")
	}
	head, err := tv.V.Git().ShowFile("HEAD", pv.Ref.Path)
	if err != nil || strings.Contains(string(head), "paths:") || !strings.Contains(string(head), "status: applied") {
		t.Fatalf("the committed document:\n%s %v", head, err)
	}
	tv.Clean()
}
