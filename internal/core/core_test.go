package core_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/core"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
	"github.com/nathanaday/atlas-obsidian/internal/work"
)

func TestStatusCountsTheVault(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("topic", "A", map[string]any{"kind": "concept", "status": "draft", "tags": []string{"work/p3"}}, "")
	tv.Doc("source", "Paper", map[string]any{"sha256": "abcdef0123456789", "file": "[[x.pdf]]"}, "")
	tv.Write("inbox/new.pdf", "%PDF")
	tv.Write("Ideas.md", "- [ ] @atlas look at this\n```\n- [ ] @atlas in code\n```\n- [ ] email me@atlas.dev\n")
	tv.Write("wiki/documents/Note.md", "- [ ] @atlas not in the documents\n")
	tv.Commit()
	if _, err := work.Stub(tv.V, work.StubIn{Text: "x", Title: "Idea"}, work.Opts{Now: testvault.Now}); err != nil {
		t.Fatal(err)
	}
	st := core.StatusOf(tv.Index(), testvault.Now)
	if st.Documents["topic"] != 1 || st.Topics.Kinds["concept"] != 1 || st.Topics.Draft != 1 || st.Work.Stubs != 1 || len(st.Work.List) != 1 {
		t.Fatalf("counts %+v", st)
	}
	if len(st.Tags) != 2 || st.Tags[0].Tag != "work" || st.Vault.Layout != 3 {
		t.Fatalf("tags %+v", st.Tags)
	}
	if len(st.Inbox) != 1 || st.Inbox[0].Kind != "pdf" || len(st.Pending) != 1 {
		t.Fatalf("inbox and pending %+v %+v", st.Inbox, st.Pending)
	}
	if len(st.Mentions) != 1 || st.Mentions[0].Line != 1 || st.Mentions[0].Text != "@atlas look at this" {
		t.Fatalf("mentions %+v", st.Mentions)
	}
}

func TestSyncHealsAndCloseMention(t *testing.T) {
	tv := testvault.New(t)
	r, err := work.Stub(tv.V, work.StubIn{Text: "x", Title: "Idea"}, work.Opts{Now: testvault.Now})
	if err != nil {
		t.Fatal(err)
	}
	stub := tv.Read(r.View.Doc.Path)
	tv.Write(r.View.Doc.Path, strings.Replace(stub, "status: open", "status: dropped", 1))
	tv.Write("sessions/2026-09/2026-09-26 0900 aaaaaa.md", "---\nid: ses-aaaaaa\ntype: session\nharness_id: aaaaaa\nstatus: running\nupdated: 2026-09-26T09:00:00\n---\n")
	tv.Write("wiki/stray/Moved.md", "---\nid: doc-moved1\ntype: topic\nkind: concept\ndescription: x\n---\n")
	synced, err := core.Sync(tv.V, testvault.Now.Add(time.Hour), core.SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(synced.Work) != 1 || len(synced.Lost) != 1 || len(synced.Moved) != 1 || synced.Views == 0 {
		t.Fatalf("synced %+v", synced)
	}
	if !strings.Contains(tv.Read(r.View.Doc.Path), "status: open") || !tv.V.Exists("wiki/documents/Moved.md") || tv.V.Exists("wiki/stray") {
		t.Fatal("sync puts the status back, and the moved document")
	}
	if !tv.V.Exists("views/View · Home.md") {
		t.Fatal("sync writes the views")
	}
	tv.Write("Ideas.md", "intro\n- [ ] @atlas track this\n")
	m, err := core.CloseMention(tv.V, "Ideas.md", 2, "Idea")
	if err != nil || m.Line != 2 {
		t.Fatalf("close %v %v", m, err)
	}
	if got := tv.Read("Ideas.md"); got != "intro\n- [x] @atlas track this → [[Idea]]\n" {
		t.Fatalf("closed %q", got)
	}
	if _, err := core.CloseMention(tv.V, "Ideas.md", 1, "Idea"); err == nil {
		t.Fatal("a line with no open mention is refused")
	}
	// A vault of the 6.x layout refuses a sync.
	tv.Write("Atlas.md", strings.Replace(tv.Read("Atlas.md"), "layout: 3", "layout: 2", 1))
	v, _ := vault.Open(tv.V.Root)
	if _, err := core.Sync(v, testvault.Now, core.SyncOptions{}); err == nil || !strings.Contains(err.Error(), "vault migrate") {
		t.Fatalf("legacy: %v", err)
	}
}
