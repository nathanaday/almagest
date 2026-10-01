package core_test

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/core"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/thread"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

func TestStatusCountsTheVault(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("topic", "A", map[string]any{"kind": "concept", "status": "draft", "tags": []string{"work/p3"}}, "")
	tv.Doc("source", "Paper", map[string]any{"sha256": "abcdef0123456789", "file": "[[x.pdf]]"}, "")
	tv.Write("inbox/new.pdf", "%PDF")
	tv.Write("Ideas.md", "- [ ] @atlas look at this\n```\n- [ ] @atlas in code\n```\n- [ ] email me@atlas.dev\n")
	tv.Write("wiki/documents/Note.md", "- [ ] @atlas not in the documents\n")
	tv.Commit()
	if _, err := thread.Stub(tv.V, thread.StubIn{Text: "x", Title: "Idea"}, thread.Opts{Now: testvault.Now}); err != nil {
		t.Fatal(err)
	}
	st := core.StatusOf(tv.Index(), testvault.Now)
	if st.Documents["topic"] != 1 || st.Topics.Kinds["concept"] != 1 || st.Topics.Draft != 1 || st.Threads.Stubs != 1 || len(st.Threads.List) != 1 {
		t.Fatalf("counts %+v", st)
	}
	if len(st.Tags) != 2 || st.Tags[0].Tag != "work" || st.Vault.Layout != 4 {
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
	r, err := thread.Stub(tv.V, thread.StubIn{Text: "x", Title: "Idea"}, thread.Opts{Now: testvault.Now})
	if err != nil {
		t.Fatal(err)
	}
	stub := tv.Read(r.State.Thread.Path)
	tv.Write(r.State.Thread.Path, strings.Replace(stub, "status: stub", "status: dropped", 1))
	tv.Write("sessions/2026-09/2026-09-26 0900 aaaaaa.md", "---\nid: ses-aaaaaa\ntype: session\nharness_id: aaaaaa\nstatus: running\nupdated: 2026-09-26T09:00:00\n---\n")
	tv.Write("wiki/stray/Moved.md", "---\nid: doc-moved1\ntype: topic\nkind: concept\ndescription: x\n---\n")
	synced, err := core.Sync(tv.V, testvault.Now.Add(time.Hour), core.SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(synced.Threads) != 1 || len(synced.Lost) != 1 || len(synced.Moved) != 1 || synced.Views == 0 {
		t.Fatalf("synced %+v", synced)
	}
	if !strings.Contains(tv.Read(r.State.Thread.Path), "status: stub") || !tv.V.Exists("wiki/documents/Moved.md") || tv.V.Exists("wiki/stray") {
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
	// A vault of an earlier layout refuses a sync.
	tv.Write("Atlas.md", strings.Replace(tv.Read("Atlas.md"), "layout: 4", "layout: 3", 1))
	v, _ := vault.Open(tv.V.Root)
	if _, err := core.Sync(v, testvault.Now, core.SyncOptions{}); err == nil || !strings.Contains(err.Error(), "vault migrate") {
		t.Fatalf("legacy: %v", err)
	}
}

func TestSyncEndsASessionWhoseAgentIsGone(t *testing.T) {
	tv := testvault.New(t)
	gone := exec.Command("true")
	if err := gone.Run(); err != nil {
		t.Fatal(err)
	}
	now := testvault.Now
	recent := vault.Stamp(now.Add(-time.Minute))
	session := func(name string, pid int) string {
		rel := "sessions/2026-09/2026-09-27 1400 " + name + ".md"
		tv.Write(rel, "---\nid: ses-"+name+"\ntype: session\nharness_id: "+name+"\nstatus: idle\nupdated: "+recent+"\npid: "+strconv.Itoa(pid)+"\n---\n")
		return rel
	}
	dead := session("aaaaaa", gone.Process.Pid)
	// A live process that is no agent: its id was reused.
	other := session("bbbbbb", os.Getpid())
	quiet := session("cccccc", 0)
	synced, err := core.Sync(tv.V, now, core.SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{dead, other} {
		if got := tv.Read(rel); !strings.Contains(got, "status: ended") || !strings.Contains(got, "ended: "+vault.Stamp(now)) {
			t.Fatalf("a session whose agent is gone ends:\n%s", got)
		}
	}
	if got := tv.Read(quiet); !strings.Contains(got, "status: idle") || len(synced.Lost) != 2 {
		t.Fatalf("a recent session with no process id stays: %v\n%s", synced.Lost, got)
	}
}
