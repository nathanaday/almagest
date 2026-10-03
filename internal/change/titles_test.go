package change_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/change"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
)

func TestARenameAppliesTheTitleItsPreviewShowed(t *testing.T) {
	tv := testvault.New(t)
	id := tv.Doc("topic", "Motion scoring", map[string]any{"kind": "concept"}, "")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Rename", Writes: []change.Write{{Op: "rename", ID: id, Title: "Old → New"}}})
	previewed := ""
	for _, w := range pv.Writes {
		if w.Op == "rename" {
			previewed = strings.TrimSuffix(strings.TrimPrefix(w.Path, "wiki/documents/"), ".md")
		}
	}
	if previewed != "Old New" {
		t.Fatalf("the preview shows %q: %+v", previewed, pv.Writes)
	}
	apply(t, tv, pv.Ref.ID)
	if !tv.V.Exists("wiki/documents/" + previewed + ".md") {
		t.Fatalf("the applied title is not the previewed one; the documents are %v", tv.Index().Docs)
	}
	tv.Clean()
}

func TestAChangeRefusesALongTitle(t *testing.T) {
	tv := testvault.New(t)
	refused(t, tv, change.Plan{Title: "Create", Writes: []change.Write{{Op: "create", Type: "topic", Kind: "concept", Title: strings.Repeat("t", 151), Fields: map[string]any{"description": "A topic.", "status": "stable"}, Body: str("## Definition\n\nA topic.\n")}}}, "a title holds at most 150")
	refused(t, tv, change.Plan{Title: strings.Repeat("p", 151)}, "the plan's title")
}

func TestACaseOnlyRenameStillApplies(t *testing.T) {
	tv := testvault.New(t)
	id := tv.Doc("topic", "Motion scoring", map[string]any{"kind": "concept"}, "")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Rename", Writes: []change.Write{{Op: "rename", ID: id, Title: "Motion Scoring"}}})
	apply(t, tv, pv.Ref.ID)
	if got := tv.V.OnDisk("wiki/documents/Motion Scoring.md"); got != "wiki/documents/Motion Scoring.md" {
		t.Fatalf("the file on disk is %s", got)
	}
	tv.Clean()
}

func TestAHandEditedHeadingCannotWriteOutsideTheDocuments(t *testing.T) {
	for _, bad := range []string{"../../outside", "x/../../y", "Old → New"} {
		tv := testvault.New(t)
		pv := propose(t, tv, change.Plan{Title: "Create", Writes: []change.Write{{Op: "create", Type: "topic", Kind: "concept", Title: "Alpha", Fields: map[string]any{"description": "A topic.", "status": "stable"}, Body: str("## Definition\n\nA topic.\n")}}})
		rel := pv.Ref.Path
		content := tv.Read(rel)
		edited := strings.Replace(content, " · Alpha · ", " · "+bad+" · ", 1)
		if edited == content {
			t.Fatalf("no heading to edit in:\n%s", content)
		}
		tv.Write(rel, edited)
		before, _ := os.ReadDir(filepath.Dir(tv.V.Root))
		if _, err := change.Show(tv.Index(), pv.Ref.ID); err == nil || !strings.Contains(err.Error(), "propose the change again") {
			t.Errorf("%s: show: %v", bad, err)
		}
		if _, err := change.Apply(tv.V, pv.Ref.ID, tv.Tick(time.Minute), nil); err == nil || !strings.Contains(err.Error(), "propose the change again") {
			t.Errorf("%s: apply: %v", bad, err)
		}
		after, _ := os.ReadDir(filepath.Dir(tv.V.Root))
		if len(after) != len(before) {
			t.Errorf("%s: a file appeared beside the vault: %v", bad, after)
		}
		if _, err := os.Stat(filepath.Join(tv.V.Root, "outside.md")); err == nil {
			t.Errorf("%s: outside.md was written", bad)
		}
	}
}

func TestProposeWritesNoChangeDocumentThroughALinkOut(t *testing.T) {
	tv := testvault.New(t)
	now := tv.Tick(time.Minute)
	away := filepath.Join(filepath.Dir(tv.V.Root), "away")
	if err := os.MkdirAll(away, 0o755); err != nil {
		t.Fatal(err)
	}
	month := tv.V.Abs("changes/" + now.Add(time.Minute).Format("2006-01"))
	os.RemoveAll(month)
	if err := os.Symlink(away, month); err != nil {
		t.Fatal(err)
	}
	_, err := change.Propose(tv.V, change.Plan{Title: "Linked", Writes: []change.Write{{Op: "create", Type: "topic", Kind: "concept", Title: "Alpha", Fields: map[string]any{"description": "A topic.", "status": "stable"}, Body: str("## Definition\n\nA topic.\n")}}}, now.Add(time.Minute))
	if err == nil || !strings.Contains(err.Error(), "not a path inside the vault") {
		t.Fatalf("propose through a linked month folder: %v", err)
	}
	if entries, _ := os.ReadDir(away); len(entries) != 0 {
		t.Fatalf("propose wrote outside the vault: %v", entries)
	}
}

func TestAChangeDocumentNeverReplacesOneInAnotherUnicodeForm(t *testing.T) {
	tv := testvault.New(t)
	probe := tv.V.Abs("changes/probe-Café")
	os.WriteFile(probe, nil, 0o644)
	_, err := os.Lstat(tv.V.Abs("changes/probe-Café"))
	os.Remove(probe)
	if err != nil {
		t.Skip("this file system keeps NFC and NFD names apart")
	}
	plan := func(title, topic string) change.Plan {
		return change.Plan{Title: title, Writes: []change.Write{{Op: "create", Type: "topic", Kind: "concept", Title: topic, Fields: map[string]any{"description": "A topic.", "status": "stable"}, Body: str("## Definition\n\nA topic.\n")}}}
	}
	first := propose(t, tv, plan("Café", "Alpha"))
	before := tv.Read(first.Ref.Path)
	second := propose(t, tv, plan("Café", "Beta"))
	if second.Ref.Path == first.Ref.Path || !strings.HasSuffix(second.Ref.Path, " (2).md") {
		t.Fatalf("the second change document is %s", second.Ref.Path)
	}
	if tv.Read(first.Ref.Path) != before {
		t.Fatal("the first change document changed")
	}
}
