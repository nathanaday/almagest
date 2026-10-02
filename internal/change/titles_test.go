package change_test

import (
	"strings"
	"testing"

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
