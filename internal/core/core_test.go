package core_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/almagest/internal/change"
	"github.com/nathanaday/almagest/internal/core"
	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/testvault"
	"github.com/nathanaday/almagest/internal/vault"
)

func TestStatusCountsTheVault(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("topic", "A", map[string]any{"kind": "concept", "status": "draft", "tags": []string{"work/p3"}}, "")
	tv.Doc("source", "Paper", map[string]any{"sha256": "abcdef0123456789", "file": "[[x.pdf]]"}, "")
	tv.Write("ingest/new.pdf", "%PDF")
	tv.Write("threads/Idea.md", "---\nid: doc-idea01\ntype: stub\ndescription: An idea.\n---\n\nAn idea.\n")
	tv.Write("journals/cs566/Week 1.md", "My notes.\n")
	tv.Commit()
	st := core.StatusOf(tv.Index(), testvault.Now)
	if st.Documents["topic"] != 1 || st.Documents["source"] != 1 || len(st.Documents) != 3 || st.Topics.Kinds["concept"] != 1 || st.Topics.Draft != 1 {
		t.Fatalf("counts %+v", st)
	}
	if len(st.Tags) != 2 || st.Tags[0].Tag != "work" || st.Vault.Layout != 7 {
		t.Fatalf("tags %+v", st.Tags)
	}
	if len(st.Ingest) != 1 || st.Ingest[0].Kind != "pdf" || len(st.Pending) != 1 {
		t.Fatalf("ingest and pending %+v %+v", st.Ingest, st.Pending)
	}
	var keys map[string]any
	data, _ := json.Marshal(st)
	json.Unmarshal(data, &keys)
	for _, gone := range []string{"threads", "recent", "mentions", "inbox"} {
		if _, ok := keys[gone]; ok {
			t.Fatalf("the status holds %q: %s", gone, data)
		}
	}
}

func TestSyncRewrites(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("topic", "Idea", map[string]any{"kind": "concept"}, "")
	tv.Commit()
	tv.Write("sessions/2026-09/2026-09-26 0900 aaaaaa.md", "---\nid: ses-aaaaaa\ntype: session\nharness_id: aaaaaa\nstatus: running\nupdated: 2026-09-26T09:00:00\n---\n")
	tv.Write("source-core/stray/Moved.md", "---\nid: doc-moved1\ntype: topic\nkind: concept\ndescription: x\n---\n")
	synced, err := core.Sync(tv.V, testvault.Now.Add(time.Hour), core.SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(synced.Lost) != 1 || len(synced.Moved) != 1 || synced.Views == 0 {
		t.Fatalf("synced %+v", synced)
	}
	if !tv.V.Exists("source-core/documents/Moved.md") || tv.V.Exists("source-core/stray/Moved.md") {
		t.Fatal("sync puts the moved document back")
	}
	if !tv.V.Exists("wiki-view/View · Home.md") {
		t.Fatal("sync writes the views")
	}
	// A vault of an earlier layout refuses a sync.
	tv.Write("Almagest.md", strings.Replace(tv.Read("Almagest.md"), "layout: 7", "layout: 5", 1))
	v, _ := vault.Open(tv.V.Root)
	if _, err := core.Sync(v, testvault.Now, core.SyncOptions{}); err == nil || !strings.Contains(err.Error(), "vault migrate") {
		t.Fatalf("legacy: %v", err)
	}
}

func TestSnapshotCommitsTheHandEdits(t *testing.T) {
	tv := testvault.New(t)
	tv.Commit()
	if sha, n, err := core.Snapshot(tv.V); err != nil || sha != "" || n != 0 {
		t.Fatalf("a clean vault: %q %d %v", sha, n, err)
	}
	tv.Write("scratchpad/Idea.md", "an idea\n")
	tv.Write("journals/cs566/Week 1.md", "notes\n")
	sha, n, err := core.Snapshot(tv.V)
	if err != nil || sha == "" || n != 2 {
		t.Fatalf("two hand edits: %q %d %v", sha, n, err)
	}
	tv.Clean()
	if log := tv.Log(); !strings.HasPrefix(log[0], "snapshot: 2 files edited by hand") {
		t.Fatalf("the commit: %v", log)
	}
	unlock, err := tv.V.Lock()
	if err != nil {
		t.Fatal(err)
	}
	tv.Write("scratchpad/Idea.md", "a second thought\n")
	if _, _, err := core.Snapshot(tv.V); err == nil {
		t.Fatal("a snapshot while a write holds the lock")
	}
	unlock()
	if sha, _, err := core.Snapshot(tv.V); err != nil || sha == "" {
		t.Fatalf("the snapshot after the write: %q %v", sha, err)
	}
}

// A snapshot after a crashed apply puts the apply back first, so it never records the
// half-written documents as hand edits.
func TestASnapshotRecoversACrashedApplyFirst(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("topic", "Alpha", map[string]any{"kind": "concept"}, "## Definition\n\nOld.\n")
	tv.Commit()
	before := tv.Read(vault.DocPath("Alpha"))
	body := "## Definition\n\nNew.\n"
	pv, err := change.Propose(tv.V, change.Plan{Title: "Rewrite Alpha", Writes: []change.Write{{Op: "modify", ID: "Alpha", Body: &body}}}, tv.Tick(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	// The state an apply leaves when it stops before its commit: the change says
	// applied and lists its paths, and the document holds the new text.
	tv.Write(pv.Ref.Path, doc.SetFields(tv.Read(pv.Ref.Path), []doc.Field{{Key: "status", Value: "applied"}, {Key: "paths", Value: []string{vault.DocPath("Alpha")}}}))
	tv.Write(vault.DocPath("Alpha"), strings.Replace(before, "Old.", "New.", 1))
	if _, _, err := core.Snapshot(tv.V); err != nil {
		t.Fatal(err)
	}
	if got := tv.Read(vault.DocPath("Alpha")); got != before {
		t.Fatalf("the snapshot kept the half-applied text:\n%s", got)
	}
	if got := doc.Parse("", []byte(tv.Read(pv.Ref.Path))); got.Str("status") != "proposed" || got.Front.Has("paths") {
		t.Fatalf("the change after the recovery:\n%s", got.Content)
	}
	tv.Clean()
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
