package change_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/almagest/internal/change"
	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/lint"
	"github.com/nathanaday/almagest/internal/testvault"
	"github.com/nathanaday/almagest/internal/vault"
)

// A work document runs from the first step: progress lines gather in it, and the plan
// fills it, keeping its files and its progress.
func TestAWorkDocumentRunsThenTakesTheProposal(t *testing.T) {
	tv := testvault.New(t)
	tv.Write("ingest/DINOv2.pdf", "%PDF")
	tv.Write("ingest/notes.md", "notes\n")
	tv.Commit()
	if _, err := change.Start(tv.V, change.StartIn{Kind: "ingest", Files: []string{"missing.pdf"}}, tv.Clock); err == nil || !strings.Contains(err.Error(), "not a file in ingest/") {
		t.Fatalf("a file that is not in ingest/: %v", err)
	}
	if _, err := change.Start(tv.V, change.StartIn{Kind: "sync"}, tv.Clock); err == nil || !strings.Contains(err.Error(), "a repair, or a draft") {
		t.Fatalf("a kind that is none: %v", err)
	}
	pv, err := change.Start(tv.V, change.StartIn{Kind: "ingest", Files: []string{"DINOv2.pdf", "ingest/notes.md"}}, tv.Tick(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if pv.Status != "running" || !strings.HasSuffix(pv.Ref.Path, "Ingest 2 files.md") {
		t.Fatalf("the work document: %+v", pv)
	}
	work := tv.Read(pv.Ref.Path)
	for _, want := range []string{"status: running", "kind: ingest", "files: [DINOv2.pdf, notes.md]", "cssclasses: [almagest-change]", "> [!change] Running · ingest · 2 files", change.Widget, "## Files\n\n- `DINOv2.pdf`\n- `notes.md`", "## Progress"} {
		if !strings.Contains(work, want) {
			t.Fatalf("the work document lacks %q:\n%s", want, work)
		}
	}
	if _, err := change.Progress(tv.V, pv.Ref.ID, "  captured DINOv2.pdf  ", tv.Tick(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := change.Progress(tv.V, pv.Ref.ID, "", tv.Clock); err == nil {
		t.Fatal("an empty progress line")
	}
	clock := tv.Clock.Format(vault.ClockFormat)
	if got := tv.Read(pv.Ref.Path); !strings.Contains(got, "## Progress\n\n- "+clock+" captured DINOv2.pdf") {
		t.Fatalf("the progress line:\n%s", got)
	}
	filled := propose(t, tv, change.Plan{ID: pv.Ref.ID, Title: "ignored for a running document", Notes: "The paper's ideas.", Writes: []change.Write{
		{Op: "create", Type: "topic", Kind: "concept", Title: "Self-distillation", Fields: map[string]any{"description": "A student learns from a teacher of its own weights."}, Body: str("## Definition\n\nx\n"), Why: "the paper's core method"},
	}})
	if filled.Ref.ID != pv.Ref.ID || filled.Ref.Path != pv.Ref.Path || filled.Status != "proposed" {
		t.Fatalf("the proposal did not fill the work document: %+v", filled.Ref)
	}
	got := tv.Read(pv.Ref.Path)
	for _, want := range []string{"status: proposed", "kind: ingest", "## Summary\n\n- **create** topic (concept) [[Self-distillation]] · the paper's core method", "- " + clock + " captured DINOv2.pdf", "## Files\n\n- `DINOv2.pdf`", "## Notes\n\nThe paper's ideas.", "### create · topic concept · Self-distillation"} {
		if !strings.Contains(got, want) {
			t.Fatalf("the filled document lacks %q:\n%s", want, got)
		}
	}
	f, err := lint.Run(tv.Index(), lint.Options{Now: tv.Clock})
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range f.Findings {
		if x.Check == "dead-link" {
			t.Errorf("a proposed change's summary is a dead link: %s", x.Message)
		}
	}
	if _, err := change.Progress(tv.V, pv.Ref.ID, "late", tv.Clock); err == nil || !strings.Contains(err.Error(), "not running") {
		t.Fatalf("progress on a proposed change: %v", err)
	}
	applied := apply(t, tv, pv.Ref.ID)
	if applied.Status != "applied" || !tv.V.Exists(vault.DocPath("Self-distillation")) {
		t.Fatalf("apply %+v", applied)
	}
}

// Cancel of a running document stops the work: the agent's next step is refused.
func TestACancelledWorkDocumentRefusesTheAgent(t *testing.T) {
	tv := testvault.New(t)
	pv, err := change.Start(tv.V, change.StartIn{Kind: "repair", Title: "Repair the lint findings"}, tv.Tick(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := change.Reject(tv.V, pv.Ref.ID, "not now", tv.Tick(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := tv.Read(pv.Ref.Path); !strings.Contains(got, "status: rejected") || !strings.Contains(got, "> [!change] Rejected · no writes\n> Rejected: not now") {
		t.Fatalf("the cancelled document:\n%s", got)
	}
	if _, err := change.Progress(tv.V, pv.Ref.ID, "a step", tv.Clock); err == nil || !strings.Contains(err.Error(), "Repair the lint findings (not now); stop the work") {
		t.Fatalf("progress after cancel: %v", err)
	}
	_, err = change.Propose(tv.V, change.Plan{ID: pv.Ref.ID, Title: "x", Writes: []change.Write{{Op: "create", Type: "topic", Kind: "concept", Title: "X"}}}, tv.Clock)
	if err == nil || !strings.Contains(err.Error(), "stop the work") {
		t.Fatalf("a proposal after cancel: %v", err)
	}
	if tv.V.Exists(vault.DocPath("X")) {
		t.Fatal("a topic was written")
	}
}

// A remove sends the document to the trash; undo brings it back and empties its place
// in the trash.
func TestARemoveGoesToTheTrash(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("topic", "Old idea", map[string]any{"kind": "concept"}, "## Definition\n\nx\n")
	tv.Commit()
	before := tv.Read(vault.DocPath("Old idea"))
	pv := propose(t, tv, change.Plan{Title: "Drop the old idea", Writes: []change.Write{{Op: "remove", ID: "Old idea", Why: "no longer true"}}})
	if got := tv.Read(pv.Ref.Path); !strings.Contains(got, "- **remove** Old idea (to trash) · no longer true") {
		t.Fatalf("the summary:\n%s", got)
	}
	applied := apply(t, tv, pv.Ref.ID)
	trash := "tool/trash/" + vault.Date(tv.Clock) + "/" + vault.DocPath("Old idea")
	if applied.Writes[0].Trash != trash || tv.V.Exists(vault.DocPath("Old idea")) || tv.Read(trash) != before {
		t.Fatalf("the remove: %+v", applied.Writes)
	}
	tv.Clean()
	if tv.Index().ByID(tv.ID("Old idea")) != nil {
		t.Fatal("the index reads the trash")
	}
	if _, err := change.Undo(tv.V, pv.Ref.ID, tv.Tick(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if !tv.V.Exists(vault.DocPath("Old idea")) || tv.V.Exists(trash) {
		t.Fatal("undo did not bring the document back from the trash")
	}
	tv.Clean()
}

// The summary links what stays and names what goes.
func TestTheSummaryNamesEachWrite(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("topic", "Alpha", map[string]any{"kind": "concept"}, "## Definition\n\none\n")
	tv.Doc("topic", "Beta", map[string]any{"kind": "concept"}, "## Definition\n\nx\n")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Edits", Writes: []change.Write{
		{Op: "modify", ID: "Alpha", Body: str("## Definition\n\none\ntwo\n")},
		{Op: "rename", ID: "Beta", Title: "Gamma"},
		{Op: "confirm", ID: "Alpha"},
	}})
	got, _ := doc.Section(doc.Parse("", []byte(tv.Read(pv.Ref.Path))).Body, "Summary")
	want := "- **modify** [[Alpha]] · +1 −0\n- **rename** Beta → [[Gamma]]\n- **confirm** [[Alpha]]"
	if !strings.Contains(got, want) {
		t.Fatalf("the summary:\n%s\nwant:\n%s\n%s", got, want, tv.Read(pv.Ref.Path))
	}
}
