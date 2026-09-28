package core_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/core"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/threads"
)

func TestStatusCountsTheVault(t *testing.T) {
	tv := testvault.New(t)
	tv.Page("area", "p3", nil, "")
	tv.Page("concept", "A", map[string]any{"status": "draft"}, "")
	tv.Page("source", "Paper", map[string]any{"sha256": "abcdef0123456789", "file": "[[x.pdf]]"}, "")
	tv.Write("inbox/new.pdf", "%PDF")
	tv.Write("Ideas.md", "- [ ] @atlas look at this\n```\n- [ ] @atlas in code\n```\n- [ ] email me@atlas.dev\n")
	tv.Write("wiki/concepts/Note.md", "- [ ] @atlas not in the wiki\n")
	if _, err := threads.Open(tv.V, threads.OpenIn{Text: "x", Title: "Work"}, testvault.Now); err != nil {
		t.Fatal(err)
	}
	st := core.StatusOf(tv.Index(), testvault.Now)
	if st.Scopes.Areas != 1 || st.Wiki.Pages["concept"] != 1 || st.Wiki.Draft != 1 || st.Threads.Open["stub"] != 1 {
		t.Fatalf("counts %+v", st)
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
	r, err := threads.Open(tv.V, threads.OpenIn{Text: "x", Title: "Work"}, testvault.Now)
	if err != nil {
		t.Fatal(err)
	}
	stub := tv.Read(r.View.Stub.Path)
	tv.Write(r.View.Stub.Path, strings.Replace(stub, "stage: stub", "stage: tasks", 1))
	tv.Write("sessions/2026-09/2026-09-26 0900 aaaaaa.md", "---\nid: ses-aaaaaa\ntype: session\nharness_id: aaaaaa\nstatus: running\nupdated: 2026-09-26T09:00:00\n---\n")
	synced, err := core.Sync(tv.V, testvault.Now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(synced.Threads) != 1 || len(synced.Lost) != 1 {
		t.Fatalf("synced %+v", synced)
	}
	if !strings.Contains(tv.Read(r.View.Stub.Path), "stage: stub") {
		t.Fatal("sync puts the stage back")
	}
	tv.Write("Ideas.md", "intro\n- [ ] @atlas track this\n")
	m, err := core.CloseMention(tv.V, "Ideas.md", 2, "Work")
	if err != nil || m.Line != 2 {
		t.Fatalf("close %v %v", m, err)
	}
	if got := tv.Read("Ideas.md"); got != "intro\n- [x] @atlas track this → [[Work]]\n" {
		t.Fatalf("closed %q", got)
	}
	if _, err := core.CloseMention(tv.V, "Ideas.md", 1, "Work"); err == nil {
		t.Fatal("a line with no open mention is refused")
	}
}
