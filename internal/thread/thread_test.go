package thread_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/change"
	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/thread"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

func opts(tv *testvault.T) thread.Opts {
	return thread.Opts{Now: tv.Tick(time.Minute), By: thread.ByAgent}
}

func ok(t *testing.T) func(*thread.Result, error) *thread.Result {
	return func(r *thread.Result, err error) *thread.Result {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
}

func refuses(t *testing.T, want string) func(*thread.Result, error) {
	return func(_ *thread.Result, err error) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("want a refusal with %q, got %v", want, err)
		}
	}
}

func load(tv *testvault.T, key string) (*thread.Board, *doc.Doc) {
	idx := tv.Index()
	d, err := idx.Resolve(key)
	if err != nil {
		panic(err)
	}
	return thread.Load(idx), d
}

func status(tv *testvault.T, key string) string {
	b, d := load(tv, key)
	return b.Status(d)
}

const specText = "## Goal\n\nFewer false alarms. More text.\n\n## Requirements\n\n- R1: Each box has a motion score.\n- R2: A box below the threshold raises no alarm.\n\n## Rules\n\nNo new dependency.\n\n## Knowledge\n\n- [[p3-edge]]: where the scoring runs.\n"

// setup links a repository and plants one stub.
func setup(t *testing.T) *testvault.T {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", nil)
	tv.Doc("repository", "p3-edge", map[string]any{"path": repo, "defines": "work/p3/p3-edge", "tags": []string{"work/p3"}}, "")
	tv.Commit()
	ok(t)(thread.Stub(tv.V, thread.StubIn{Text: "Score boxes by motion, so a still box raises no alarm.", Title: "Score boxes by motion", Tags: []string{"work/p3/p3-edge"}}, opts(tv)))
	return tv
}

// specified takes the stub of setup to a spec and a task list of two tasks.
func specified(t *testing.T) *testvault.T {
	tv := setup(t)
	ok(t)(thread.Spec(tv.V, thread.SpecIn{Thread: "Score boxes by motion", Text: specText}, opts(tv)))
	ok(t)(thread.TasksWrite(tv.V, thread.TasksIn{Thread: "Score boxes by motion", Repository: "p3-edge", Tasks: []thread.TaskIn{
		{Text: "Compute the score", Requirements: []string{"R1"}, Details: "In score.go."},
		{Text: "Gate the alarm", Requirements: []string{"r2", "R1"}},
	}}, opts(tv)))
	return tv
}

func pass(reqs ...string) []thread.ResultIn {
	var out []thread.ResultIn
	for _, r := range reqs {
		out = append(out, thread.ResultIn{Requirement: r, Result: "pass", Evidence: "go test ./... passes | 12 tests"})
	}
	return out
}

func TestAStubIsAShortCard(t *testing.T) {
	tv := setup(t)
	tv.Clean()
	if log := tv.Log(); log[0] != "thread: stub Score boxes by motion" {
		t.Fatalf("log %v", log)
	}
	b, d := load(tv, "Score boxes by motion")
	if b.Status(d) != thread.StatusStub || d.Str("status") != "stub" || d.Str("verification") != "none" {
		t.Fatalf("stub fields:\n%s", d.Content)
	}
	for _, want := range []string{
		"> [!thread] Stub · normal · planted 2026-09-27",
		"> Missing: a spec · a task list · a verification · the wiki change",
		"> Next: the thread has no spec with requirements (thread-spec)",
		"## Idea\n\nScore boxes by motion",
		"## Thread\n\n- Spec: none\n- Tasks: none\n- Verification: none",
		"> [!handoff] Hand off to an agent\n> ```\n> Resume Atlas thread " + d.ID() + "\n> ```",
	} {
		if !strings.Contains(d.Content, want) {
			t.Errorf("the stub lacks %q:\n%s", want, d.Content)
		}
	}
	if n := b.Next(d); n.Step != "spec" || n.Skill != "thread-spec" {
		t.Fatalf("next %+v", n)
	}
}

func TestASpecHoldsOnlyRequirements(t *testing.T) {
	tv := setup(t)
	spec := func(text string) (*thread.Result, error) {
		return thread.Spec(tv.V, thread.SpecIn{Thread: "Score boxes by motion", Text: text}, opts(tv))
	}
	refuses(t, "## Progress is no section of a spec")(spec(specText + "\n## Progress\n\n- did it\n"))
	refuses(t, "a spec holds no check box")(spec(specText + "\n## Decisions\n\n- [ ] decide\n"))
	refuses(t, "the spec needs ## Requirements")(spec("## Goal\n\nA goal.\n"))
	refuses(t, "this line has no id")(spec("## Goal\n\nA goal.\n\n## Requirements\n\n- R1: one\n- two\n"))
	refuses(t, "two requirements are R1")(spec("## Goal\n\nA goal.\n\n## Requirements\n\n- R1: one\n- R1: two\n"))
	refuses(t, "the spec needs ## Goal")(spec("## Requirements\n\n- R1: one\n"))
	r := ok(t)(spec(specText))
	if r.State.Thread.Status != thread.Specified || r.State.Next.Step != "tasks" {
		t.Fatalf("state %+v", r.State)
	}
	_, d := load(tv, "Score boxes by motion · Spec")
	if d.Str("thread") != "[[Score boxes by motion]]" || d.Str("status") != thread.NotImplemented || d.Str("description") != "Fewer false alarms." || len(d.List("tags")) != 1 {
		t.Fatalf("spec fields:\n%s", d.Content)
	}
	if !strings.Contains(d.Content, "> [!spec] Not implemented · 2 requirements\n> Thread: [[Score boxes by motion]] (specified)") {
		t.Fatalf("spec lead:\n%s", d.Content)
	}
	_, stub := load(tv, "Score boxes by motion")
	for _, want := range []string{"- Spec: [[Score boxes by motion · Spec]] · 2 requirements · not implemented", "- Knowledge: [[p3-edge]]"} {
		if !strings.Contains(stub.Content, want) {
			t.Errorf("the stub lacks %q:\n%s", want, stub.Content)
		}
	}
	// A second spec call revises the one spec.
	ok(t)(spec(strings.Replace(specText, "raises no alarm", "raises no alert", 1)))
	if b, s := load(tv, "Score boxes by motion"); b.Thread(s).Spec == nil || len(b.Extra) != 0 || !strings.Contains(tv.Read("wiki/documents/Score boxes by motion · Spec.md"), "no alert") {
		t.Fatal("the spec is revised in place")
	}
}

func TestTasksServeRequirements(t *testing.T) {
	tv := setup(t)
	tasks := func(in thread.TasksIn) (*thread.Result, error) {
		in.Thread = "Score boxes by motion"
		return thread.TasksWrite(tv.V, in, opts(tv))
	}
	refuses(t, "has no spec with requirements")(tasks(thread.TasksIn{Tasks: []thread.TaskIn{{Text: "x", Requirements: []string{"R1"}}}}))
	ok(t)(thread.Spec(tv.V, thread.SpecIn{Thread: "Score boxes by motion", Text: specText}, opts(tv)))
	refuses(t, "which is no requirement")(tasks(thread.TasksIn{Tasks: []thread.TaskIn{{Text: "x", Requirements: []string{"R9"}}}}))
	refuses(t, "names no requirement")(tasks(thread.TasksIn{Tasks: []thread.TaskIn{{Text: "x"}}}))
	refuses(t, "no document is named")(tasks(thread.TasksIn{Repository: "nope", Tasks: []thread.TaskIn{{Text: "x", Requirements: []string{"R1"}}}}))
	refuses(t, "has no task list")(thread.Start(tv.V, "Score boxes by motion", false, opts(tv)))
	r := ok(t)(tasks(thread.TasksIn{Repository: "p3-edge", Tasks: []thread.TaskIn{
		{Text: "Compute the score", Requirements: []string{"R1"}, Details: "In score.go."},
		{Text: "Gate the alarm", Requirements: []string{"r2", "R1"}},
	}}))
	if r.State.Thread.Status != thread.Planned || r.State.Next.Step != "start" {
		t.Fatalf("state %+v", r.State)
	}
	list := tv.Read("wiki/documents/Score boxes by motion · Tasks (p3-edge).md")
	for _, want := range []string{"repository: \"[[p3-edge]]\"", "total: 2", "> [!tasks] 0/2 done · [[p3-edge]]", "## Tasks\n\n- [ ] T1: Compute the score (R1)\n- [ ] T2: Gate the alarm (R2, R1)\n", "## Details\n\n### T1\n\nIn score.go."} {
		if !strings.Contains(list, want) {
			t.Errorf("the list lacks %q:\n%s", want, list)
		}
	}
	// More tasks join the list of their repository; a list with no repository is its own.
	ok(t)(tasks(thread.TasksIn{Repository: "p3-edge", Tasks: []thread.TaskIn{{Text: "Log the score", Requirements: []string{"R1"}, Details: "At debug level."}}}))
	ok(t)(tasks(thread.TasksIn{Tasks: []thread.TaskIn{{Text: "Write the note", Requirements: []string{"R2"}}}}))
	b, stub := load(tv, "Score boxes by motion")
	if th := b.Thread(stub); len(th.Lists) != 2 || th.CountsText() != "0/4" || stub.Str("tasks") != "0/4" || len(stub.List("repositories")) != 1 {
		t.Fatalf("lists: %s", stub.Content)
	}
	if !strings.Contains(tv.Read("wiki/documents/Score boxes by motion · Tasks (p3-edge).md"), "- [ ] T3: Log the score (R1)") || !strings.Contains(tv.Read("wiki/documents/Score boxes by motion · Tasks.md"), "- [ ] T4: Write the note (R2)") {
		t.Fatal("ids run on across the lists of a thread")
	}
}

func TestTheLifecycleIsDerived(t *testing.T) {
	tv := specified(t)
	name := "Score boxes by motion"
	check := func(in thread.CheckIn) (*thread.Result, error) {
		in.Thread = name
		return thread.Check(tv.V, in, opts(tv))
	}
	verify := func(in thread.VerifyIn) (*thread.Result, error) {
		in.Thread = name
		return thread.Verify(tv.V, in, opts(tv))
	}
	refuses(t, "is not started")(check(thread.CheckIn{Task: "T1", Commits: []string{"a3f9c21"}}))
	r := ok(t)(thread.Start(tv.V, name, false, opts(tv)))
	if r.Started != name || len(r.Events) != 1 || r.State.Thread.Status != thread.Started || !strings.Contains(r.State.Next.Reason, "T1") {
		t.Fatalf("start %+v", r)
	}
	refuses(t, "a done task carries what did it")(check(thread.CheckIn{Task: "T1"}))
	refuses(t, "has no task \"T9\"; its open tasks are T1, T2")(check(thread.CheckIn{Task: "T9", Note: "x"}))
	refuses(t, "has 2 open tasks (T1, T2)")(verify(thread.VerifyIn{Scope: "p3-edge", Results: pass("R1", "R2")}))
	ok(t)(check(thread.CheckIn{Task: "t1", Commits: []string{"a3f9c21deadbeef"}, Note: "scores in score.go"}))
	r = ok(t)(check(thread.CheckIn{Task: "T2", State: "dropped", Reason: "the gate exists"}))
	list := tv.Read("wiki/documents/Score boxes by motion · Tasks (p3-edge).md")
	if !strings.Contains(list, "- [x] T1: Compute the score (R1) · a3f9c21dea · scores in score.go\n- [-] T2: Gate the alarm (R2, R1) · dropped: the gate exists") || !strings.Contains(list, "> [!tasks] 1/1 done") {
		t.Fatalf("checked list:\n%s", list)
	}
	if r.State.Thread.Status != thread.Unverified || r.State.Next.Step != "verify" || tv.Read("wiki/documents/Score boxes by motion · Spec.md") == "" {
		t.Fatalf("state %+v", r.State)
	}
	if !strings.Contains(tv.Read("wiki/documents/Score boxes by motion · Spec.md"), "status: complete (unverified)") {
		t.Fatal("the spec follows its thread")
	}
	// A verification gives every requirement a result, and a failure a finding.
	refuses(t, "no result for R2")(verify(thread.VerifyIn{Scope: "p3-edge at a3f9c21", Results: pass("R1")}))
	refuses(t, "R7 is no requirement")(verify(thread.VerifyIn{Scope: "p3-edge", Results: pass("R1", "R2", "R7")}))
	refuses(t, "a result needs its evidence")(verify(thread.VerifyIn{Scope: "p3-edge", Results: []thread.ResultIn{{Requirement: "R1", Result: "pass"}}}))
	failed := append(pass("R1"), thread.ResultIn{Requirement: "R2", Result: "fail", Evidence: "a still box alarms"})
	refuses(t, "a failed requirement needs a finding")(verify(thread.VerifyIn{Scope: "p3-edge", Results: failed}))
	r = ok(t)(verify(thread.VerifyIn{Scope: "p3-edge at a3f9c21", Results: failed, Findings: []string{"The gate reads the old score.", "The threshold is a constant."}, Notes: "Ran on the bench."}))
	if r.State.Thread.Status != thread.Unverified || r.State.Next.Step != "findings" {
		t.Fatalf("state %+v", r.State)
	}
	v1 := tv.Read("wiki/documents/Score boxes by motion · Verification 1.md")
	for _, want := range []string{"round: 1", "verdict: fail", "> [!verification-fail] Round 1 · fail", "| R1: Each box has a motion score. | pass | go test ./... passes \\| 12 tests |", "| R2: A box below the threshold raises no alarm. | fail | a still box alarms |", "- [ ] F1: The gate reads the old score.\n- [ ] F2: The threshold is a constant.", "## Notes\n\nRan on the bench."} {
		if !strings.Contains(v1, want) {
			t.Errorf("the verification lacks %q:\n%s", want, v1)
		}
	}
	_, stub := load(tv, name)
	if !strings.Contains(stub.Content, "> Missing: round 1 failed R2 · the wiki change") || !strings.Contains(stub.Content, "- Verification: [[Score boxes by motion · Verification 1]] · fail · 2 open findings") {
		t.Fatalf("the stub says what is missing:\n%s", stub.Content)
	}
	// Each finding gets one outcome. A task takes the thread back to started.
	finding := func(in thread.FindingIn) (*thread.Result, error) {
		in.Thread = name
		return thread.FindingOutcome(tv.V, in, opts(tv))
	}
	refuses(t, "needs reason")(finding(thread.FindingIn{Finding: "F2", Outcome: "accepted"}))
	refuses(t, "revise the spec first")(finding(thread.FindingIn{Finding: "F2", Outcome: "spec"}))
	refuses(t, "outcome is \"later\"")(finding(thread.FindingIn{Finding: "F2", Outcome: "later"}))
	ok(t)(finding(thread.FindingIn{Finding: "F2", Outcome: "stub", Text: "Make the threshold a setting"}))
	r = ok(t)(finding(thread.FindingIn{Finding: "F1", Outcome: "task", Repository: "p3-edge", Task: &thread.TaskIn{Text: "Read the new score in the gate", Requirements: []string{"R2"}}}))
	if r.State.Thread.Status != thread.Started || len(r.State.Missing) == 0 {
		t.Fatalf("state %+v", r.State)
	}
	refuses(t, "has its outcome already")(finding(thread.FindingIn{Finding: "F1", Outcome: "accepted", Reason: "x"}))
	v1 = tv.Read("wiki/documents/Score boxes by motion · Verification 1.md")
	if !strings.Contains(v1, "- [x] F1: The gate reads the old score. → task T3\n- [x] F2: The threshold is a constant. → [[Make the threshold a setting]]") || !strings.Contains(v1, "verdict: stale") {
		t.Fatalf("outcomes:\n%s", v1)
	}
	if status(tv, "Make the threshold a setting") != thread.StatusStub {
		t.Fatal("the outcome stub plants a stub")
	}
	// The next round passes, and the thread is verified.
	if r := ok(t)(thread.Start(tv.V, name, true, opts(tv))); r.Events[0].Kind != "continued" {
		t.Fatalf("a second start continues: %+v", r.Events)
	}
	ok(t)(check(thread.CheckIn{Task: "T3", Commits: []string{"b4e8d10"}}))
	refuses(t, "every task is done")(thread.Start(tv.V, name, true, opts(tv)))
	r = ok(t)(verify(thread.VerifyIn{Scope: "p3-edge at b4e8d10", Results: pass("R1", "R2")}))
	if r.State.Thread.Status != thread.Verified || r.State.Next.Step != "close" || r.State.Next.Skill != "thread-close" {
		t.Fatalf("state %+v", r.State)
	}
	spec := tv.Index().ByPath("wiki/documents/Score boxes by motion · Spec.md")
	v2 := tv.Index().ByPath("wiki/documents/Score boxes by motion · Verification 2.md")
	if spec.Str("status") != thread.CompleteVerified || v2.Str("verdict") != thread.Pass || !tv.Index().Pending(spec) || !tv.Index().Pending(v2) {
		t.Fatalf("a verified thread is pending for the wiki:\n%s", v2.Content)
	}
	// The applied change that absorbs the spec and the verification closes the thread.
	pv, err := change.Propose(tv.V, change.Plan{Title: "Absorb motion scoring", Notes: "p3-edge scores boxes by motion.", Work: name, Absorbs: []string{spec.ID(), v2.ID()},
		Writes: []change.Write{{Op: "create", Type: "topic", Kind: "concept", Title: "Motion scoring", Fields: map[string]any{"description": "Scoring boxes by motion.", "sources": []any{spec.Title()}}, Body: ptr("## Definition\n\nA score per box.\n")}}}, tv.Tick(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if status(tv, name) != thread.Verified {
		t.Fatal("a proposal closes nothing")
	}
	if _, err := change.Apply(tv.V, pv.Ref.ID, tv.Tick(time.Minute), nil); err != nil {
		t.Fatal(err)
	}
	b, stub := load(tv, name)
	if b.Status(stub) != thread.Closed || stub.Str("status") != "closed" || !strings.Contains(stub.Content, "> [!thread-closed] Closed 2026-09-27 · absorbed by [[2026-09-27 Absorb motion scoring]] · 2/2 tasks") || strings.Contains(stub.Content, "[!handoff]") {
		t.Fatalf("closed:\n%s", stub.Content)
	}
	if n := b.Next(stub); n.Step != "none" {
		t.Fatalf("next %+v", n)
	}
	tv.Clean()
	// A new task takes a closed thread up again, with no reopen.
	refuses(t, "is closed; new work on it starts with a new task")(thread.Reopen(tv.V, name, "", opts(tv)))
	ok(t)(thread.TasksWrite(tv.V, thread.TasksIn{Thread: name, Repository: "p3-edge", Tasks: []thread.TaskIn{{Text: "One more", Requirements: []string{"R1"}}}}, opts(tv)))
	if status(tv, name) != thread.Started {
		t.Fatalf("status %s", status(tv, name))
	}
}

func ptr(s string) *string { return &s }

func TestAnEditOfTheSpecMakesAVerificationStale(t *testing.T) {
	tv := specified(t)
	name := "Score boxes by motion"
	ok(t)(thread.Start(tv.V, name, false, opts(tv)))
	for _, id := range []string{"T1", "T2"} {
		ok(t)(thread.Check(tv.V, thread.CheckIn{Thread: name, Task: id, Note: "done by hand"}, opts(tv)))
	}
	ok(t)(thread.Verify(tv.V, thread.VerifyIn{Thread: name, Scope: "p3-edge", Results: pass("R1", "R2")}, opts(tv)))
	if status(tv, name) != thread.Verified {
		t.Fatal("verified")
	}
	// A reworded goal is no change to what was verified; a reworded requirement is.
	rel := "wiki/documents/Score boxes by motion · Spec.md"
	tv.Write(rel, strings.Replace(tv.Read(rel), "Fewer false alarms.", "Far fewer false alarms.", 1))
	if status(tv, name) != thread.Verified {
		t.Fatal("the goal is not what a verification checks")
	}
	tv.Write(rel, strings.Replace(tv.Read(rel), "raises no alarm", "raises no alarm within one second", 1))
	b, stub := load(tv, name)
	if b.Status(stub) != thread.Unverified || b.VerificationText(b.Thread(stub)) != "round 1: stale" || b.Next(stub).Step != "verify" {
		t.Fatalf("status %s, %s", b.Status(stub), b.VerificationText(b.Thread(stub)))
	}
	// A box the user checks by hand counts, and one the user opens takes the thread back.
	list := "wiki/documents/Score boxes by motion · Tasks (p3-edge).md"
	tv.Write(list, strings.Replace(tv.Read(list), "- [x] T1", "- [ ] T1", 1))
	if status(tv, name) != thread.Started {
		t.Fatal("an open box is an open task")
	}
	if _, err := thread.Load(tv.Index()).Sync(tv.V.WriteIfChanged); err != nil {
		t.Fatal(err)
	}
	if got := tv.Index().ByPath(list); got.Front.Int("done") != 1 || got.Front.Int("total") != 2 {
		t.Fatalf("sync writes the counts:\n%s", got.Content)
	}
}

func TestDropReopenBlockResolveAndSet(t *testing.T) {
	tv := specified(t)
	name := "Score boxes by motion"
	refuses(t, "only a stub with no spec resolves")(thread.Resolve(tv.V, name, []string{"p3-edge"}, opts(tv)))
	ok(t)(thread.Block(tv.V, name+" · Spec", "the camera is away", opts(tv)))
	b, stub := load(tv, name)
	if b.Blocked(stub) != "the camera is away" || b.Next(stub).Step != "wait" || !strings.Contains(stub.Content, "> Blocked: the camera is away") {
		t.Fatalf("blocked:\n%s", stub.Content)
	}
	ok(t)(thread.Unblock(tv.V, name, opts(tv)))
	ok(t)(thread.Drop(tv.V, name, "No longer needed.", opts(tv)))
	if status(tv, name) != thread.Dropped {
		t.Fatal("dropped")
	}
	refuses(t, "is dropped; thread reopen")(thread.TasksWrite(tv.V, thread.TasksIn{Thread: name, Tasks: []thread.TaskIn{{Text: "x", Requirements: []string{"R1"}}}}, opts(tv)))
	ok(t)(thread.Reopen(tv.V, name, "Needed again.", opts(tv)))
	if status(tv, name) != thread.Planned {
		t.Fatalf("a reopened thread takes its derived status: %s", status(tv, name))
	}
	// A rename takes every document of the thread, and every link.
	title := "Score every box by motion"
	ok(t)(thread.Set(tv.V, thread.SetIn{Doc: name, Title: &title, Tags: &[]string{"work/p3"}}, opts(tv)))
	tv.Clean()
	for _, rel := range []string{"Score every box by motion.md", "Score every box by motion · Spec.md", "Score every box by motion · Tasks (p3-edge).md"} {
		if !tv.V.Exists("wiki/documents/" + rel) {
			t.Fatalf("%s is missing", rel)
		}
	}
	spec := tv.Read("wiki/documents/Score every box by motion · Spec.md")
	if tv.V.Exists("wiki/documents/Score boxes by motion · Spec.md") || !strings.Contains(spec, "thread: \"[[Score every box by motion]]\"") || !strings.Contains(spec, "tags: [work/p3]") {
		t.Fatalf("the spec follows the rename and the tags:\n%s", spec)
	}
	b, stub = load(tv, title)
	if th := b.Thread(stub); th.Spec == nil || len(th.Lists) != 1 || len(b.Events(stub)) != 4 {
		t.Fatalf("the thread holds together after a rename: %d events", len(b.Events(stub)))
	}
	// A stub with no spec may become other documents.
	ok(t)(thread.Stub(tv.V, thread.StubIn{Text: "Read the motion paper", Title: "Read the motion paper"}, opts(tv)))
	ok(t)(thread.Resolve(tv.V, "Read the motion paper", []string{"p3-edge"}, opts(tv)))
	_, read := load(tv, "Read the motion paper")
	if read.Str("status") != thread.Resolved || !strings.Contains(read.Content, "> [!thread-resolved] Resolved 2026-09-27 → [[p3-edge]]") {
		t.Fatalf("resolved:\n%s", read.Content)
	}
}

func TestLoadGivesEverythingInOneRead(t *testing.T) {
	tv := specified(t)
	name := "Score boxes by motion"
	ok(t)(thread.Start(tv.V, name, false, opts(tv)))
	ok(t)(thread.Check(tv.V, thread.CheckIn{Thread: name, Task: "T1", Commits: []string{"a3f9c21"}}, opts(tv)))
	ok(t)(thread.Note(tv.V, name+" · Spec", "The bench camera drops frames.", opts(tv)))
	_, stub := load(tv, name)
	l, err := thread.LoadThread(tv.Index(), stub.ID())
	if err != nil {
		t.Fatal(err)
	}
	if l.Handoff != "Resume Atlas thread "+stub.ID() || l.Thread.Status != thread.Started || l.Next.Step != "run" || !strings.Contains(l.Idea, "Score boxes") {
		t.Fatalf("load %+v", l)
	}
	if l.Spec == nil || !strings.Contains(l.Spec.Text, "## Requirements") || strings.Contains(l.Spec.Text, "[!spec]") || len(l.Requirements) != 2 {
		t.Fatalf("spec %+v", l.Spec)
	}
	if len(l.Lists) != 1 || l.Lists[0].Repository == nil || l.Lists[0].Repository.Title != "p3-edge" || !strings.HasSuffix(l.Lists[0].Repository.Path, "/code/p3-edge") {
		t.Fatalf("lists %+v", l.Lists)
	}
	if open := l.Lists[0].Open; len(open) != 1 || open[0].ID != "T2" || len(open[0].Requirements) != 2 || len(l.Lists[0].Closed) != 1 || !strings.Contains(l.Lists[0].Closed[0], "T1: Compute the score (R1) · a3f9c21") {
		t.Fatalf("tasks %+v", l.Lists[0])
	}
	if len(l.Knowledge) != 1 || l.Knowledge[0].Title != "p3-edge" || len(l.Notes) != 1 || l.Notes[0].Text != "The bench camera drops frames." || len(l.Events) != 2 {
		t.Fatalf("knowledge %+v, notes %+v, events %d", l.Knowledge, l.Notes, len(l.Events))
	}
	if _, err := json.Marshal(l); err != nil {
		t.Fatal(err)
	}
	// A part of the thread names the thread.
	if byPart, err := thread.LoadThread(tv.Index(), name+" · Tasks (p3-edge)"); err != nil || byPart.Thread.ID != stub.ID() {
		t.Fatalf("load by a part: %v", err)
	}
}

// chord makes a chord of four threads: Curate, then Annotate and Baseline, then Train
// after both.
func chord(t *testing.T) *testvault.T {
	tv := testvault.New(t)
	ok(t)(thread.ChordCreate(tv.V, thread.ChordIn{Title: "Vehicle detection model", Text: "A YOLO model that finds vehicles runs on the device.", Tags: []string{"ml"}, Threads: []thread.ChordThreadIn{
		{Title: "Train round 1", Text: "Train the first model.", After: []string{"Annotate the dataset", "Baseline"}},
		{Title: "Curate the dataset", Text: "Pick the images."},
		{Title: "Annotate the dataset", Text: "Label the images.", After: []string{"Curate the dataset"}},
		{Title: "Baseline", Text: "Measure the stock model.", After: []string{"Curate the dataset"}, Priority: "high"},
	}}, opts(tv)))
	return tv
}

func TestAChordOrdersItsThreads(t *testing.T) {
	tv := chord(t)
	tv.Clean()
	if log := tv.Log(); log[0] != "thread: chord Vehicle detection model with 4 threads" {
		t.Fatalf("log %v", log)
	}
	b, c := load(tv, "Vehicle detection model")
	var order []string
	for _, s := range b.Members(c) {
		order = append(order, s.Title())
	}
	if strings.Join(order, ", ") != "Curate the dataset, Baseline, Annotate the dataset, Train round 1" {
		t.Fatalf("order %v", order)
	}
	_, train := load(tv, "Train round 1")
	if b.Rank(train) != 2 || train.Front.Int("rank") != 2 || b.Ready(train) || b.Next(train).Step != "spec" || len(train.List("tags")) != 1 {
		t.Fatalf("train:\n%s", train.Content)
	}
	if ready := b.ChordReady(c); len(ready) != 1 || ready[0].Title() != "Curate the dataset" || b.Status(c) != thread.ChordOpen {
		t.Fatalf("ready %v", ready)
	}
	for _, want := range []string{
		"> [!chord] Open · 0/4 threads closed · normal", "> Ready: [[Curate the dataset]]",
		"| 1 | [[Curate the dataset]] | stub, ready |  |  |  |", "| Step | Thread |",
		"| 3 | [[Train round 1]] | stub |  | [[Annotate the dataset]], [[Baseline]] |  |",
		"Canvas: [[chords/Vehicle detection model.canvas|the order as a graph]]",
		"> Resume Atlas chord " + c.ID(),
	} {
		if !strings.Contains(c.Content, want) {
			t.Errorf("the chord lacks %q:\n%s", want, c.Content)
		}
	}
	if !strings.Contains(train.Content, "> Chord: [[Vehicle detection model]] · after [[Annotate the dataset]] (stub), [[Baseline]] (stub)") || !strings.Contains(train.Content, "> Missing: [[Annotate the dataset]] first (stub) · [[Baseline]] first (stub) · a spec") {
		t.Fatalf("the stub names its place:\n%s", train.Content)
	}
	// A loop is refused, in a create, a set, and an order.
	refuses(t, "would wait on each other")(thread.ChordCreate(tv.V, thread.ChordIn{Title: "Loop", Text: "x", Threads: []thread.ChordThreadIn{{Title: "A1", Text: "a", After: []string{"B1"}}, {Title: "B1", Text: "b", After: []string{"A1"}}}}, opts(tv)))
	refuses(t, "Curate the dataset waits on Train round 1 waits on")(thread.Set(tv.V, thread.SetIn{Doc: "Curate the dataset", After: &[]string{"Train round 1"}}, opts(tv)))
	refuses(t, "would wait on each other")(thread.ChordOrder(tv.V, "Vehicle detection model", []thread.OrderIn{{Thread: "Curate the dataset", After: []string{"Baseline"}}}, opts(tv)))
	// A stub joins and leaves; the order changes.
	ok(t)(thread.Stub(tv.V, thread.StubIn{Text: "Quantize to INT8", Title: "INT8 study", Chord: "Vehicle detection model", After: []string{"Train round 1"}}, opts(tv)))
	ok(t)(thread.Stub(tv.V, thread.StubIn{Text: "Write the paper", Title: "Paper"}, opts(tv)))
	r := ok(t)(thread.ChordAdd(tv.V, "Vehicle detection model", "Paper", []string{"INT8 study"}, opts(tv)))
	if r.Chord == nil || len(r.Chord.Threads) != 6 || r.Chord.Threads[5].Title != "Paper" {
		t.Fatalf("add %+v", r.Chord)
	}
	ok(t)(thread.ChordOrder(tv.V, "Vehicle detection model", []thread.OrderIn{{Thread: "Paper", After: []string{"Train round 1"}}}, opts(tv)))
	ok(t)(thread.ChordRemove(tv.V, "Vehicle detection model", "INT8 study", opts(tv)))
	refuses(t, "INT8 study is no thread of Vehicle detection model")(thread.ChordOrder(tv.V, "Vehicle detection model", []thread.OrderIn{{Thread: "INT8 study", After: nil}}, opts(tv)))
	b, c = load(tv, "Vehicle detection model")
	_, paper := load(tv, "Paper")
	if len(b.Members(c)) != 5 || b.Rank(paper) != 3 || c.Str("threads") != "0/5" {
		t.Fatalf("members %d, rank %d", len(b.Members(c)), b.Rank(paper))
	}
	// A verified thread lets the next ones go on; a dropped chord drops its threads.
	ok(t)(thread.Drop(tv.V, "Curate the dataset", "The set exists.", opts(tv)))
	b, c = load(tv, "Vehicle detection model")
	if ready := b.ChordReady(c); len(ready) != 2 || ready[0].Title() != "Baseline" {
		t.Fatalf("ready after a drop: %v", ready)
	}
	r = ok(t)(thread.Drop(tv.V, "Vehicle detection model", "Another team has it.", opts(tv)))
	if len(r.Events) != 5 || status(tv, "Vehicle detection model") != thread.ChordDropped || status(tv, "Paper") != thread.Dropped {
		t.Fatalf("drop %d events", len(r.Events))
	}
	r = ok(t)(thread.Reopen(tv.V, "Vehicle detection model", "It is ours again.", opts(tv)))
	if len(r.Events) != 5 || status(tv, "Paper") != thread.StatusStub || status(tv, "Curate the dataset") != thread.Dropped {
		t.Fatalf("a reopened chord brings back the threads dropped with it: %d events, Paper %s", len(r.Events), status(tv, "Paper"))
	}
	tv.Clean()
}

type canvasFile struct {
	Nodes  []map[string]any `json:"nodes"`
	Edges  []map[string]any `json:"edges"`
	Legend []map[string]any `json:"-"`
}

// readCanvas reads a canvas, with the legend's cards apart from the others.
func readCanvas(t *testing.T, tv *testvault.T, rel string) canvasFile {
	t.Helper()
	var c canvasFile
	if err := json.Unmarshal([]byte(tv.Read(rel)), &c); err != nil {
		t.Fatal(err)
	}
	var nodes []map[string]any
	for _, n := range c.Nodes {
		if id, _ := n["id"].(string); strings.HasPrefix(id, thread.LegendPrefix) {
			c.Legend = append(c.Legend, n)
		} else {
			nodes = append(nodes, n)
		}
	}
	c.Nodes = nodes
	return c
}

func TestTheCanvasShowsTheOrderAndSavesIt(t *testing.T) {
	tv := chord(t)
	rel := "chords/Vehicle detection model.canvas"
	c := readCanvas(t, tv, rel)
	if len(c.Nodes) != 4 || len(c.Edges) != 4 {
		t.Fatalf("canvas: %d nodes, %d edges", len(c.Nodes), len(c.Edges))
	}
	node := map[string]map[string]any{}
	for _, n := range c.Nodes {
		node[vault.NoteTitle(n["file"].(string))] = n
	}
	if node["Curate the dataset"]["x"].(float64) != 0 || node["Train round 1"]["x"].(float64) != 2*(thread.CardWidth+thread.CardGapX) || node["Baseline"]["y"].(float64) == node["Annotate the dataset"]["y"].(float64) {
		t.Fatalf("layout %v", node)
	}
	state, err := thread.CanvasStatus(tv.Index(), "Vehicle detection model")
	if err != nil || state.Differs || !state.Exists {
		t.Fatalf("a fresh canvas agrees with the stubs: %+v %v", state, err)
	}
	// The user moves a card, adds a text node, draws Baseline → Annotate, and drags in a stub.
	ok(t)(thread.Stub(tv.V, thread.StubIn{Text: "Deploy on the device", Title: "Deploy"}, opts(tv)))
	_, deploy := load(tv, "Deploy")
	id := func(title string) string { return node[title]["id"].(string) }
	node["Curate the dataset"]["x"] = -900.0
	c.Nodes = append(c.Nodes, map[string]any{"id": "note1", "type": "text", "text": "Round 1", "x": 0.0, "y": -300.0, "width": 200.0, "height": 60.0},
		map[string]any{"id": "abc123", "type": "file", "file": deploy.Path, "x": 2400.0, "y": 0.0, "width": 400.0, "height": 300.0})
	c.Edges = append(c.Edges, map[string]any{"id": "e1", "fromNode": id("Baseline"), "toNode": id("Annotate the dataset"), "fromSide": "bottom", "toSide": "top"},
		map[string]any{"id": "e2", "fromNode": id("Train round 1"), "toNode": "abc123"})
	data, _ := json.Marshal(c)
	tv.Write(rel, string(data))
	state, _ = thread.CanvasStatus(tv.Index(), "Vehicle detection model")
	if !state.Differs || strings.Join(state.Changes, "; ") != "add the thread Deploy; put Annotate the dataset after Baseline; put Deploy after Train round 1" {
		t.Fatalf("changes %v", state.Changes)
	}
	if strings.Join(state.Threads, ", ") != "Annotate the dataset, Deploy" {
		t.Fatalf("the threads that move: %v", state.Threads)
	}
	// A sync keeps what the user drew, and a status change still recolors a card.
	ok(t)(thread.Block(tv.V, "Baseline", "no GPU", opts(tv)))
	c = readCanvas(t, tv, rel)
	if len(c.Nodes) != 6 || len(c.Edges) != 6 {
		t.Fatalf("a sync keeps the user's canvas: %d nodes, %d edges", len(c.Nodes), len(c.Edges))
	}
	for _, n := range c.Nodes {
		if n["id"] == id("Baseline") && n["color"] != thread.ColorBlocked {
			t.Fatalf("a blocked card is yellow: %v", n)
		}
	}
	// Save writes the order to the stubs.
	r := ok(t)(thread.CanvasSave(tv.V, "Vehicle detection model", thread.Opts{Now: tv.Tick(time.Minute), By: thread.ByUser}))
	if len(r.Chord.Threads) != 5 {
		t.Fatalf("saved %+v", r.Chord)
	}
	b, annotate := load(tv, "Annotate the dataset")
	_, deploy = load(tv, "Deploy")
	if got := strings.Join(annotate.List("after"), ","); got != "[[Baseline]],[[Curate the dataset]]" || b.Chord(deploy) == nil || deploy.Str("after") != "[[Train round 1]]" || b.Rank(deploy) != 4 {
		t.Fatalf("after %s; deploy:\n%s", got, deploy.Content)
	}
	c = readCanvas(t, tv, rel)
	kept := 0
	for _, n := range c.Nodes {
		if (n["id"] == "note1") || (n["id"] == id("Curate the dataset") && n["x"].(float64) == -900) || (n["id"] == "abc123" && n["x"].(float64) == 2400) {
			kept++
		}
	}
	if kept != 3 || len(c.Nodes) != 6 || len(c.Edges) != 6 {
		t.Fatalf("the canvas keeps the user's places and nodes: %d kept, %d nodes, %d edges", kept, len(c.Nodes), len(c.Edges))
	}
	if state, _ = thread.CanvasStatus(tv.Index(), "Vehicle detection model"); state.Differs {
		t.Fatalf("after a save the canvas agrees: %v", state.Changes)
	}
	tv.Clean()
	// A loop on the canvas is refused, and a card taken off leaves the chord.
	c.Edges = append(c.Edges, map[string]any{"id": "e3", "fromNode": "abc123", "toNode": id("Curate the dataset")})
	data, _ = json.Marshal(c)
	tv.Write(rel, string(data))
	refuses(t, "the arrows make a loop")(thread.CanvasSave(tv.V, "Vehicle detection model", opts(tv)))
	// Revert takes the stubs' order back; tidy places the cards again.
	ok(t)(thread.CanvasWrite(tv.V, "Vehicle detection model", true, opts(tv)))
	c = readCanvas(t, tv, rel)
	if state, _ = thread.CanvasStatus(tv.Index(), "Vehicle detection model"); state.Differs || len(c.Edges) != 6 {
		t.Fatalf("revert: %v, %d edges", state.Changes, len(c.Edges))
	}
	for _, n := range c.Nodes {
		if n["id"] == id("Curate the dataset") && n["x"].(float64) != 0 {
			t.Fatalf("tidy places the cards again: %v", n)
		}
	}
	var nodes []map[string]any
	for _, n := range c.Nodes {
		if n["id"] != "abc123" {
			nodes = append(nodes, n)
		}
	}
	c.Nodes = nodes
	data, _ = json.Marshal(c)
	tv.Write(rel, string(data))
	ok(t)(thread.CanvasSave(tv.V, "Vehicle detection model", opts(tv)))
	if b, deploy = load(tv, "Deploy"); b.Chord(deploy) != nil {
		t.Fatal("a card taken off the canvas leaves the chord")
	}
}

func TestAChordClosesWhenItsThreadsDo(t *testing.T) {
	tv := testvault.New(t)
	ok(t)(thread.ChordCreate(tv.V, thread.ChordIn{Title: "Small chord", Text: "One thing is done.", Threads: []thread.ChordThreadIn{{Title: "Only thread", Text: "Do it."}}}, opts(tv)))
	name := "Only thread"
	ok(t)(thread.Spec(tv.V, thread.SpecIn{Thread: name, Text: "## Goal\n\nDo it.\n\n## Requirements\n\n- R1: It is done.\n"}, opts(tv)))
	ok(t)(thread.TasksWrite(tv.V, thread.TasksIn{Thread: name, Tasks: []thread.TaskIn{{Text: "Do it", Requirements: []string{"R1"}}}}, opts(tv)))
	ok(t)(thread.Start(tv.V, name, false, opts(tv)))
	if status(tv, "Small chord") != thread.ChordStarted {
		t.Fatal("started")
	}
	ok(t)(thread.Check(tv.V, thread.CheckIn{Thread: name, Task: "T1", Note: "did it"}, opts(tv)))
	ok(t)(thread.Verify(tv.V, thread.VerifyIn{Thread: name, Scope: "no repository", Results: pass("R1")}, opts(tv)))
	b, c := load(tv, "Small chord")
	if b.ChordNext(c).Step != "work" || b.Status(c) != thread.ChordStarted {
		t.Fatalf("next %+v", b.ChordNext(c))
	}
	idx := tv.Index()
	absorb := func(title string, ids ...string) {
		pv, err := change.Propose(tv.V, change.Plan{Title: title, Notes: "Nothing new for the wiki.", Absorbs: ids}, tv.Tick(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := change.Apply(tv.V, pv.Ref.ID, tv.Tick(time.Minute), nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := change.Propose(tv.V, change.Plan{Title: "Too early", Notes: "x", Absorbs: []string{c.ID()}}, tv.Tick(time.Minute)); err == nil {
		t.Fatal("the wiki does not absorb a chord whose threads are open")
	}
	absorb("Close the thread", idx.ByPath("wiki/documents/Only thread · Spec.md").ID(), idx.ByPath("wiki/documents/Only thread · Verification 1.md").ID())
	b, c = load(tv, "Small chord")
	if b.Status(c) != thread.ChordDone || b.ChordNext(c).Skill != "chord-close" || !tv.Index().Pending(c) {
		t.Fatalf("done: %s", b.Status(c))
	}
	absorb("Close the chord", c.ID())
	if _, c = load(tv, "Small chord"); c.Str("status") != thread.ChordClosed || !strings.Contains(c.Content, "> [!chord-closed] Closed · 1/1 threads closed · absorbed by [[2026-09-27 Close the chord]]") {
		t.Fatalf("closed:\n%s", c.Content)
	}
}

func TestCardColorsFollowWhatEachThreadNeeds(t *testing.T) {
	tv := chord(t)
	o := func() thread.Opts { return opts(tv) }
	spec := "## Goal\n\nx\n\n## Requirements\n\n- R1: one\n"
	plan := func(name string) {
		ok(t)(thread.Spec(tv.V, thread.SpecIn{Thread: name, Text: spec}, o()))
		ok(t)(thread.TasksWrite(tv.V, thread.TasksIn{Thread: name, Tasks: []thread.TaskIn{{Text: "Do it", Requirements: []string{"R1"}}}}, o()))
	}
	// Curate is verified, Annotate is started, Baseline has every task done and no
	// verification, Train waits on both.
	plan("Curate the dataset")
	ok(t)(thread.Start(tv.V, "Curate the dataset", false, o()))
	ok(t)(thread.Check(tv.V, thread.CheckIn{Thread: "Curate the dataset", Task: "T1", Note: "done"}, o()))
	ok(t)(thread.Verify(tv.V, thread.VerifyIn{Thread: "Curate the dataset", Scope: "x", Results: pass("R1")}, o()))
	plan("Annotate the dataset")
	ok(t)(thread.Start(tv.V, "Annotate the dataset", false, o()))
	plan("Baseline")
	ok(t)(thread.Start(tv.V, "Baseline", false, o()))
	ok(t)(thread.Check(tv.V, thread.CheckIn{Thread: "Baseline", Task: "T1", Note: "done"}, o()))
	ok(t)(thread.Stub(tv.V, thread.StubIn{Text: "Deploy it", Title: "Deploy", Chord: "Vehicle detection model"}, o()))
	ok(t)(thread.Stub(tv.V, thread.StubIn{Text: "Paper", Title: "Paper", Chord: "Vehicle detection model"}, o()))
	ok(t)(thread.Block(tv.V, "Paper", "no venue", o()))
	b, _ := load(tv, "Vehicle detection model")
	want := map[string]string{
		"Curate the dataset":   thread.ColorDone,
		"Annotate the dataset": thread.ColorStarted,
		"Baseline":             thread.ColorChecking,
		"Train round 1":        "",
		"Deploy":               thread.ColorReady,
		"Paper":                thread.ColorBlocked,
	}
	for title, color := range want {
		_, d := load(tv, title)
		if got := b.CardColor(d); got != color {
			t.Errorf("%s: color %q, want %q", title, got, color)
		}
	}
	c := readCanvas(t, tv, "chords/Vehicle detection model.canvas")
	for _, n := range c.Nodes {
		title := vault.NoteTitle(n["file"].(string))
		got, _ := n["color"].(string)
		if got != want[title] {
			t.Errorf("the card of %s is %q, want %q", title, got, want[title])
		}
	}
}

func TestSyncGivesAStubOf7xItsChordAndAfter(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("stub", "Old idea", map[string]any{"status": "open", "priority": "low"}, "## Idea\n\nLater.\n")
	tv.Commit()
	if _, err := thread.Load(tv.Index()).Sync(tv.V.WriteIfChanged); err != nil {
		t.Fatal(err)
	}
	got := tv.Read("wiki/documents/Old idea.md")
	if !strings.Contains(got, "chord: \"\"") || !strings.Contains(got, "after: []") || !strings.Contains(got, "status: stub") {
		t.Fatalf("a 7.x stub gains chord and after:\n%s", got)
	}
	if again, _ := thread.Load(tv.Index()).Sync(tv.V.WriteIfChanged); len(again) != 0 {
		t.Fatalf("a second sync writes nothing: %v", again)
	}
}

// legendArrow is an arrow the user drew from the first card of a canvas to its legend.
func legendArrow(raw map[string]any) map[string]any {
	card := raw["nodes"].([]any)[0].(map[string]any)["id"]
	return map[string]any{"id": "to-legend", "fromNode": card, "fromSide": "top", "toNode": thread.LegendPrefix + "1", "toSide": "bottom"}
}

func TestASyncDropsTheOldLegendWithItsArrows(t *testing.T) {
	tv := chord(t)
	rel := "chords/Vehicle detection model.canvas"
	// A canvas of 8.1.0 that code wrote, with an arrow of the user's to the legend.
	var raw map[string]any
	json.Unmarshal([]byte(tv.Read(rel)), &raw)
	arrow := legendArrow(raw)
	raw["nodes"] = append(raw["nodes"].([]any),
		map[string]any{"id": thread.LegendPrefix + "1", "type": "group", "label": "Verified", "x": -100.0, "y": -120.0, "width": 320.0, "height": 28.0, "color": "4"})
	raw["edges"] = append(raw["edges"].([]any), arrow)
	data, _ := json.Marshal(raw)
	tv.Write(rel, string(data))
	if _, err := thread.Load(tv.Index()).Sync(tv.V.WriteIfChanged); err != nil {
		t.Fatal(err)
	}
	c := readCanvas(t, tv, rel)
	if len(c.Legend) != 0 || len(c.Nodes) != 4 || len(c.Edges) != 4 {
		t.Fatalf("the sync drops the legend and its arrow only: %d legend, %d nodes, %d edges", len(c.Legend), len(c.Nodes), len(c.Edges))
	}
}

func TestAWriteDropsTheOldLegendAndANewThreadGetsItsCard(t *testing.T) {
	tv := chord(t)
	rel := "chords/Vehicle detection model.canvas"
	// A canvas of 8.1.0 holds a legend; the user redraws an arrow and does not save.
	var raw map[string]any
	json.Unmarshal([]byte(tv.Read(rel)), &raw)
	raw["nodes"] = append(raw["nodes"].([]any),
		map[string]any{"id": thread.LegendPrefix + "1", "type": "group", "label": "Verified", "x": -100.0, "y": -120.0, "width": 320.0, "height": 28.0, "color": "4"})
	raw["edges"] = append(raw["edges"].([]any)[1:], legendArrow(raw))
	data, _ := json.Marshal(raw)
	tv.Write(rel, string(data))
	// A thread planted in the chord gets its card at once; the user's drawing stays.
	ok(t)(thread.Stub(tv.V, thread.StubIn{Text: "Deploy it", Title: "Deploy", Chord: "Vehicle detection model"}, opts(tv)))
	c := readCanvas(t, tv, rel)
	found := false
	for _, n := range c.Nodes {
		if n["file"] == "wiki/documents/Deploy.md" {
			found = n["color"] == thread.ColorReady && n["width"] != nil
		}
	}
	if !found || len(c.Edges) != 3 {
		t.Fatalf("the new card, and the user's three arrows: %v, %d edges", found, len(c.Edges))
	}
	if len(c.Legend) != 0 {
		t.Fatalf("the write drops the old legend: %v", c.Legend)
	}
	state, _ := thread.CanvasStatus(tv.Index(), "Vehicle detection model")
	if len(state.Threads) != 1 || state.Threads[0] == "Deploy" {
		t.Fatalf("only the user's arrow is unsaved, and the new thread is saved: %v", state.Threads)
	}
}
