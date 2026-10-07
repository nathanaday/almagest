package core_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/almagest/internal/change"
	"github.com/nathanaday/almagest/internal/core"
	"github.com/nathanaday/almagest/internal/testvault"
	"github.com/nathanaday/almagest/internal/vault"
)

func TestSafeDelete(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("topic", "Linked", map[string]any{"kind": "concept"}, "## Definition\n\nx\n")
	tv.Doc("topic", "Alone", map[string]any{"kind": "concept"}, "## Definition\n\nx\n")
	tv.Write("journals/cs566/Week 1.md", "I read [[Linked]].\n")
	tv.Write("scratchpad/Draft.md", "a draft\n")
	tv.Commit()
	now := testvault.Now.Add(time.Hour)

	if tv.V.Exists("ALMAGEST.md") { // a disk that folds case
		if res, err := core.Trash(tv.V, "source-core/documents/linked.md", now); err != nil || res.Moved != "" || len(res.Backlinks) != 1 {
			t.Fatalf("a linked topic in another case: %+v %v", res, err)
		}
	}
	res, err := core.Trash(tv.V, vault.DocPath("Linked"), now)
	if err != nil || res.Moved != "" || len(res.Backlinks) != 1 || res.Backlinks[0].Path != "journals/cs566/Week 1.md" {
		t.Fatalf("a linked topic: %+v %v", res, err)
	}
	if !tv.V.Exists(vault.DocPath("Linked")) {
		t.Fatal("a linked topic moved")
	}

	res, err = core.Trash(tv.V, tv.V.Abs("scratchpad/Draft.md"), now)
	want := "trash/" + vault.Date(now) + "/scratchpad/Draft.md"
	if err != nil || res.Moved != want || tv.Read(want) != "a draft\n" || tv.V.Exists("scratchpad/Draft.md") {
		t.Fatalf("a note: %+v %v", res, err)
	}
	tv.Clean()
	if log := tv.Log(); log[0] != "trash: scratchpad/Draft.md" {
		t.Fatalf("the commit: %v", log)
	}

	res, err = core.Trash(tv.V, vault.DocPath("Alone"), now)
	if err != nil || res.Change == nil || !strings.HasSuffix(res.Moved, vault.DocPath("Alone")) || tv.V.Exists(vault.DocPath("Alone")) {
		t.Fatalf("a topic nothing links: %+v %v", res, err)
	}
	if got := tv.Read(res.Change.Path); !strings.Contains(got, "status: applied") || !strings.Contains(got, "- **remove** Alone (to trash) · safe delete; nothing links it") {
		t.Fatalf("the change:\n%s", got)
	}
	tv.Clean()

	for _, rel := range []string{"Almagest.md", "almagest.md", "Changes/Changes.base", "sessions/Sessions.base", "changes/Changes.base", res.Moved, "wiki-view/View · Home.md", "../outside.md", "nowhere.md"} {
		if _, err := core.Trash(tv.V, rel, now); err == nil {
			t.Errorf("%s was taken", rel)
		}
	}
	// A topic a change created stays deletable: its change record names it.
	pv, err := change.Propose(tv.V, change.Plan{Title: "Add Gamma", Writes: []change.Write{{Op: "create", Type: "topic", Kind: "concept", Title: "Gamma", Fields: map[string]any{"description": "G."}}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := change.Apply(tv.V, pv.Ref.ID, now, nil); err != nil {
		t.Fatal(err)
	}
	if res, err := core.Trash(tv.V, vault.DocPath("Gamma"), now.Add(time.Minute)); err != nil || res.Moved == "" {
		t.Fatalf("a topic a change created: %+v %v", res, err)
	}
	st := core.StatusOf(tv.Index(), now)
	if st.Trash != 3 {
		t.Fatalf("the trash count: %d", st.Trash)
	}
}
