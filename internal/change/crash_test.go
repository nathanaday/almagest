package change_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/change"
	"github.com/nathanaday/atlas-obsidian/internal/core"
	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/thread"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
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

// A crash after the last write and before the commit: the files hold what the apply
// wrote, and the document says applied, with its real Writes section, and still holds
// paths; no commit exists. The state is the real one: the apply's commit taken off again.
// Recovery puts the change back.
func TestACrashBeforeTheCommitIsPutBack(t *testing.T) {
	tv := testvault.New(t)
	pv, _ := crashPlan(t, tv)
	before := tv.Read("wiki/documents/Motion scoring.md")
	apply(t, tv, pv.Ref.ID)
	final := tv.Read(pv.Ref.Path)
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", tv.V.Root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("reset", "-q", "--soft", "HEAD~1")
	git("reset", "-q")
	tv.Write(pv.Ref.Path, doc.SetField(final, "paths", []string{"wiki/documents/Motion scoring.md", "wiki/documents/Radar.md"}))
	nextWrite(t, tv)
	if tv.Read("wiki/documents/Motion scoring.md") != before || tv.V.Exists("wiki/documents/Radar.md") {
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

// The reviewer's probe: after a crash, the user appends a paragraph to a listed file, and
// a sync runs. Recovery puts the file back, and git keeps the paragraph.
func TestRecoveryKeepsAnEditMadeAfterTheCrash(t *testing.T) {
	tv := testvault.New(t)
	pv, _ := crashPlan(t, tv)
	before := tv.Read("wiki/documents/Motion scoring.md")
	content := doc.SetField(doc.SetField(tv.Read(pv.Ref.Path), "status", "applying"), "paths", []string{"wiki/documents/Motion scoring.md", "wiki/documents/Radar.md"})
	tv.Write(pv.Ref.Path, content)
	tv.Write("wiki/documents/Motion scoring.md", before+"\nA paragraph the user wrote after the crash.\n")
	if _, err := core.Sync(tv.V, tv.Tick(time.Minute), core.SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := tv.Read("wiki/documents/Motion scoring.md"); strings.Contains(got, "after the crash") {
		t.Fatalf("the file was not put back:\n%s", got)
	}
	if got := tv.Read(pv.Ref.Path); !strings.Contains(got, "status: proposed") || strings.Contains(got, "paths:") {
		t.Fatalf("the change is not proposed again:\n%s", got)
	}
	if tv.V.Exists("wiki/documents/Radar.md") {
		t.Fatal("the created document stayed")
	}
	out, err := exec.Command("git", "-C", tv.V.Root, "log", "-p", "--", "wiki/documents/Motion scoring.md").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "+A paragraph the user wrote after the crash.") || !strings.Contains(string(out), "recovery: ") {
		t.Fatalf("git does not keep the paragraph:\n%s %v", out, err)
	}
}

// A crash inside the commit leaves the change document staged as applied. The recovery
// commit holds only the listed paths it found changed, not what else the index holds.
func TestTheRecoveryCommitHoldsOnlyItsPaths(t *testing.T) {
	tv := testvault.New(t)
	pv, _ := crashPlan(t, tv)
	tv.Commit()
	content := tv.Read(pv.Ref.Path)
	inFlight := doc.SetField(doc.SetField(content, "status", "applied"), "paths", []string{"wiki/documents/Motion scoring.md", "wiki/documents/Radar.md"})
	tv.Write(pv.Ref.Path, inFlight)
	tv.Write("wiki/documents/Motion scoring.md", "half written")
	tv.Write("wiki/documents/Radar.md", "half written")
	if err := tv.V.Git().StageContent(pv.Ref.Path, []byte(doc.SetField(content, "status", "applied"))); err != nil {
		t.Fatal(err)
	}
	if _, err := core.Sync(tv.V, tv.Tick(time.Minute), core.SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("git", "-C", tv.V.Root, "log", "--grep", "recovery: ", "--name-only", "--format=").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(string(out)), "\n")
	if strings.Join(got, "|") != "wiki/documents/Motion scoring.md|wiki/documents/Radar.md" {
		t.Fatalf("the recovery commit holds:\n%s", out)
	}
	if staged, err := exec.Command("git", "-C", tv.V.Root, "diff", "--cached", "--name-only").CombinedOutput(); err != nil || strings.TrimSpace(string(staged)) != "" {
		t.Fatalf("recovery left staged: %q %v", staged, err)
	}
}

func TestRecoveryLeavesNothingStaged(t *testing.T) {
	tv := testvault.New(t)
	pv, _ := crashPlan(t, tv)
	content := doc.SetField(doc.SetField(tv.Read(pv.Ref.Path), "status", "applying"), "paths", []string{"wiki/documents/Motion scoring.md", "wiki/documents/Radar.md"})
	tv.Write(pv.Ref.Path, content)
	tv.Write("wiki/documents/Motion scoring.md", "half written")
	tv.Write("wiki/documents/Radar.md", "half written")
	if _, err := core.Sync(tv.V, tv.Tick(time.Minute), core.SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("git", "-C", tv.V.Root, "diff", "--cached", "--name-only").CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "" {
		t.Fatalf("recovery left staged: %q %v", out, err)
	}
}

// After a crash past the commit, a line the user adds to the change document goes into
// git before recovery takes the document from its commit.
func TestRecoveryKeepsAnEditOfTheChangeDocument(t *testing.T) {
	tv := testvault.New(t)
	pv, _ := crashPlan(t, tv)
	apply(t, tv, pv.Ref.ID)
	applied := tv.Read(pv.Ref.Path)
	inFlight := doc.SetField(applied, "paths", []string{"wiki/documents/Motion scoring.md", "wiki/documents/Radar.md"})
	tv.Write(pv.Ref.Path, inFlight+"\nA line the user added after the crash.\n")
	nextWrite(t, tv)
	if got := tv.Read(pv.Ref.Path); got != applied {
		t.Fatalf("recovery did not take the document from its commit:\n%s", got)
	}
	out, err := exec.Command("git", "-C", tv.V.Root, "log", "-p", "--", pv.Ref.Path).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "+A line the user added after the crash.") || !strings.Contains(string(out), "recovery: ") {
		t.Fatalf("git does not keep the line:\n%s %v", out, err)
	}
}

// An undo that fails before its commit leaves the files and the index as they were.
func TestAnUndoThatFailsLeavesNothingStaged(t *testing.T) {
	tv := testvault.New(t)
	pv, _ := crashPlan(t, tv)
	apply(t, tv, pv.Ref.ID)
	if _, err := thread.ChordCreate(tv.V, thread.ChordIn{Title: "Plan C", Text: "Ship it.", Threads: []thread.ChordThreadIn{{Title: "First", Text: "Do the first part."}}}, thread.Opts{Now: tv.Tick(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	topic := tv.Read("wiki/documents/Motion scoring.md")
	away := filepath.Join(filepath.Dir(tv.V.Root), "away")
	os.MkdirAll(away, 0o755)
	os.RemoveAll(tv.V.Abs("chords"))
	if err := os.Symlink(away, tv.V.Abs("chords")); err != nil {
		t.Fatal(err)
	}
	tv.Commit()
	if _, err := change.Undo(tv.V, pv.Ref.ID, tv.Tick(time.Minute)); err == nil || !strings.Contains(err.Error(), "the vault is back as it was before this call") {
		t.Fatalf("the undo through a linked chords/: %v", err)
	}
	if tv.Read("wiki/documents/Motion scoring.md") != topic || !tv.V.Exists("wiki/documents/Radar.md") {
		t.Fatal("the failed undo left files changed")
	}
	out, err := exec.Command("git", "-C", tv.V.Root, "diff", "--cached", "--name-only").CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "" {
		t.Fatalf("the failed undo left staged: %q %v", out, err)
	}
}

// A crash after the apply's derived sync and before its commit: the vault is copied at
// that moment. Recovery on the copy puts the absorbed source back as pending too, since
// the apply listed it before it wrote it.
func TestACrashAfterTheDerivedSyncPutsTheSourceBack(t *testing.T) {
	tv := testvault.New(t)
	src := tv.Doc("source", "DINOv2", map[string]any{"sha256": "3f9c1e2a7b8d44aa", "file": "[[doc-aaaaaa.pdf]]", "media": "pdf", "origin": "inbox"}, "")
	tv.Write("wiki/assets/doc-aaaaaa.pdf", "%PDF")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Absorb", Absorbs: []string{src}, Writes: []change.Write{{Op: "create", Type: "topic", Kind: "entity", Title: "Radar", Fields: map[string]any{"description": "A sensor."}}}})
	crashed := filepath.Join(t.TempDir(), "crashed")
	change.SetBeforeApplyCommit(func() {
		if err := os.CopyFS(crashed, os.DirFS(tv.V.Root)); err != nil {
			t.Fatal(err)
		}
	})
	defer change.SetBeforeApplyCommit(nil)
	apply(t, tv, pv.Ref.ID)
	if !strings.Contains(tv.Read("wiki/documents/DINOv2.md"), "status: absorbed") {
		t.Fatal("the apply did not absorb the source, so the test proves nothing")
	}
	v, err := vault.Open(crashed)
	if err != nil {
		t.Fatal(err)
	}
	// The next write is a thread write, which runs recovery but derives no source; a sync
	// would derive the source back from the proposed change and hide the gap.
	if _, err := thread.Stub(v, thread.StubIn{Text: "After the crash.", Title: "After"}, thread.Opts{Now: tv.Tick(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(crashed, "wiki/documents/DINOv2.md"))
	if strings.Contains(string(got), "status: absorbed") || strings.Contains(string(got), "absorbed by") {
		t.Fatalf("the source stays absorbed:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(crashed, "wiki/documents/Radar.md")); err == nil {
		t.Fatal("the created topic stayed")
	}
	doc, _ := os.ReadFile(filepath.Join(crashed, pv.Ref.Path))
	if !strings.Contains(string(doc), "status: proposed") || strings.Contains(string(doc), "paths:") {
		t.Fatalf("the change is not proposed again:\n%s", doc)
	}
}

// Recovery that a crash stops after its recovery commit, before it put the paths back:
// the change document records the base revision. The next recovery puts the paths back
// from that revision, not from its own recovery commit.
func TestARecoveryStoppedPartwayFinishesNextTime(t *testing.T) {
	tv := testvault.New(t)
	pv, _ := crashPlan(t, tv)
	tv.Commit()
	head, err := tv.V.Git().Head()
	if err != nil {
		t.Fatal(err)
	}
	content := doc.SetField(doc.SetField(tv.Read(pv.Ref.Path), "status", "applied"), "paths", []string{"wiki/documents/Motion scoring.md", "wiki/documents/Radar.md"})
	tv.Write(pv.Ref.Path, doc.SetField(content, "recovering", head))
	tv.Write("wiki/documents/Motion scoring.md", "half written")
	tv.Write("wiki/documents/Radar.md", "half written")
	// The state the first recovery left: its recovery commit of the crash state.
	g := tv.V.Git()
	if err := g.Add("wiki/documents/Motion scoring.md", "wiki/documents/Radar.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.CommitOnly("recovery: 2 files as found after a crash", "wiki/documents/Motion scoring.md", "wiki/documents/Radar.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := core.Sync(tv.V, tv.Tick(time.Minute), core.SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	// The sync also derives the topic's lead, so the test checks the text it put back.
	if got := tv.Read("wiki/documents/Motion scoring.md"); !strings.Contains(got, "Old.") || strings.Contains(got, "half written") || tv.V.Exists("wiki/documents/Radar.md") {
		t.Fatalf("the second recovery did not put the paths back:\n%s", got)
	}
	got := tv.Read(pv.Ref.Path)
	if !strings.Contains(got, "status: proposed") || strings.Contains(got, "paths:") || strings.Contains(got, "recovering:") {
		t.Fatalf("the change is not proposed again:\n%s", got)
	}
}

// Recovery that a crash stops after it committed an edited, landed change document: the
// next recovery takes the document from the apply's commit, and it ends without paths.
func TestALandedApplysRecoveryStoppedPartwayFinishesNextTime(t *testing.T) {
	tv := testvault.New(t)
	pv, _ := crashPlan(t, tv)
	apply(t, tv, pv.Ref.ID)
	applied := tv.Read(pv.Ref.Path)
	tv.Write(pv.Ref.Path, doc.SetField(applied, "paths", []string{"wiki/documents/Motion scoring.md"})+"\nAn edit after the crash.\n")
	tv.Commit() // the state the first recovery left: the edited document in a commit
	nextWrite(t, tv)
	if got := tv.Read(pv.Ref.Path); got != applied {
		t.Fatalf("the document is not the applied one:\n%s", got)
	}
}

// A save to a restored path while an undo runs survives the undo's failure.
func TestAFailedUndoKeepsASaveMadeDuringIt(t *testing.T) {
	tv := testvault.New(t)
	pv, _ := crashPlan(t, tv)
	apply(t, tv, pv.Ref.ID)
	if _, err := thread.ChordCreate(tv.V, thread.ChordIn{Title: "Plan C", Text: "Ship it.", Threads: []thread.ChordThreadIn{{Title: "First", Text: "Do the first part."}}}, thread.Opts{Now: tv.Tick(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	away := filepath.Join(filepath.Dir(tv.V.Root), "away")
	os.MkdirAll(away, 0o755)
	os.RemoveAll(tv.V.Abs("chords"))
	if err := os.Symlink(away, tv.V.Abs("chords")); err != nil {
		t.Fatal(err)
	}
	tv.Commit()
	saved := "a save made while the undo ran\n"
	change.SetAfterUndoRestore(func() { tv.Write("wiki/documents/Motion scoring.md", saved) })
	defer change.SetAfterUndoRestore(nil)
	_, err := change.Undo(tv.V, pv.Ref.ID, tv.Tick(time.Minute))
	if err == nil || !strings.Contains(err.Error(), "left as saved") || !strings.Contains(err.Error(), "Motion scoring.md") {
		t.Fatalf("the failed undo: %v", err)
	}
	if got := tv.Read("wiki/documents/Motion scoring.md"); got != saved {
		t.Fatalf("the save was rolled back:\n%s", got)
	}
}

// A recovering field that names no commit stops recovery before it changes anything.
func TestARecoveringFieldThatNamesNoCommitIsRefused(t *testing.T) {
	for _, bogus := range []string{"deadbeef", "HEAD", "short", "a commit outside the history"} {
		t.Run(bogus, func(t *testing.T) {
			tv := testvault.New(t)
			pv, _ := crashPlan(t, tv)
			tv.Commit()
			value := bogus
			switch bogus {
			case "short":
				value = strings.TrimSpace(git(t, tv.V.Root, "rev-parse", "--short", "HEAD"))
			case "a commit outside the history":
				value = strings.TrimSpace(git(t, tv.V.Root, "commit-tree", "HEAD^{tree}", "-m", "loose"))
			}
			content := doc.SetField(doc.SetField(tv.Read(pv.Ref.Path), "status", "applied"), "paths", []string{"wiki/documents/Motion scoring.md", "wiki/documents/Radar.md"})
			tv.Write(pv.Ref.Path, doc.SetField(content, "recovering", value))
			tv.Write("wiki/documents/Motion scoring.md", "half written")
			head := git(t, tv.V.Root, "rev-parse", "HEAD")
			_, err := thread.Stub(tv.V, thread.StubIn{Text: "After the crash.", Title: "After"}, thread.Opts{Now: tv.Tick(time.Minute)})
			if err == nil || !strings.Contains(err.Error(), "not the full id of a commit") {
				t.Fatalf("the write after recovering: %s: %v", value, err)
			}
			if tv.Read("wiki/documents/Motion scoring.md") != "half written" || git(t, tv.V.Root, "rev-parse", "HEAD") != head {
				t.Fatal("recovery changed the vault")
			}
		})
	}
}

// A promote of a stub in a chord rewrites the chord's canvas. A crash before the commit
// leaves the canvas listed, and recovery puts it back with the documents.
func TestACrashBeforeTheCommitPutsTheCanvasBack(t *testing.T) {
	tv := testvault.New(t)
	_, err := thread.ChordCreate(tv.V, thread.ChordIn{Title: "Plan C", Text: "Ship it.", Threads: []thread.ChordThreadIn{{Title: "First", Text: "Motion helps to score boxes."}, {Title: "Second", Text: "Then this.", After: []string{"First"}}}}, thread.Opts{Now: tv.Tick(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	tv.Commit()
	canvas := "chords/Plan C.canvas"
	before := tv.Read(canvas)
	first := tv.Index().ByPath("wiki/documents/First.md")
	pv := propose(t, tv, change.Plan{Title: "Promote", Writes: []change.Write{{Op: "promote", ID: first.ID(), Kind: "concept", Title: "Motion scoring", Fields: map[string]any{"description": "Scoring boxes by motion."}, Body: str("## Definition\n\nMotion.\n")}}})
	crashed := filepath.Join(t.TempDir(), "crashed")
	change.SetBeforeApplyCommit(func() {
		if err := os.CopyFS(crashed, os.DirFS(tv.V.Root)); err != nil {
			t.Fatal(err)
		}
	})
	defer change.SetBeforeApplyCommit(nil)
	apply(t, tv, pv.Ref.ID)
	if tv.Read(canvas) == before {
		t.Fatal("the promote did not rewrite the canvas, so the test proves nothing")
	}
	inFlight, _ := os.ReadFile(filepath.Join(crashed, pv.Ref.Path))
	if !strings.Contains(string(inFlight), canvas) {
		t.Fatalf("the in-flight paths do not list the canvas:\n%s", inFlight)
	}
	v, err := vault.Open(crashed)
	if err != nil {
		t.Fatal(err)
	}
	// Recovery alone, before any sync could derive the canvas back.
	if err := vault.Recover(v); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(crashed, canvas)); string(got) != before {
		t.Fatalf("recovery did not put the canvas back:\n%s", got)
	}
}

// git runs git in dir and returns its output.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}
