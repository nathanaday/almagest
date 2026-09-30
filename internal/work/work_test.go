package work_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
	"github.com/nathanaday/atlas-obsidian/internal/work"
)

func opts(tv *testvault.T) work.Opts { return work.Opts{Now: tv.Tick(time.Minute), By: work.ByAgent} }

func ok(t *testing.T) func(*work.Result, error) *work.Result {
	return func(r *work.Result, err error) *work.Result {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
}

func load(tv *testvault.T, key string) (*work.Board, *doc.Doc) {
	idx := tv.Index()
	d, err := idx.Resolve(key)
	if err != nil {
		panic(err)
	}
	return work.Load(idx), d
}

// setup links a repository and writes a plan with two parts.
func setup(t *testing.T) *testvault.T {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", nil)
	tv.Doc("repository", "p3-edge", map[string]any{"path": repo, "defines": "work/p3/p3-edge", "tags": []string{"work/p3"}}, "")
	tv.Commit()
	ok(t)(work.Specs(tv.V, work.SpecsIn{Specs: []work.SpecIn{
		{Title: "Filter false alarms", Kind: "plan", Text: "## Goal\n\nFewer false alarms. More text.\n\n## Done when\n\n- alarms drop by half\n", Tags: []string{"work/p3/p3-edge"}},
		{Title: "Score boxes by motion", Kind: "plan", Parent: "Filter false alarms", Repositories: []string{"p3-edge"}, Text: "## Goal\n\nScore boxes.\n\n## Done when\n\n- scored\n"},
		{Title: "Tune the threshold", Kind: "plan", Parent: "Filter false alarms", Repositories: []string{"p3-edge"}, Depends: []string{"Score boxes by motion"}, Text: "## Goal\n\nTune.\n\n## Done when\n\n- tuned\n"},
	}}, opts(tv)))
	return tv
}

func TestSpecsWriteAPlanAndItsParts(t *testing.T) {
	tv := setup(t)
	if log := tv.Log(); log[0] != "work: specs Filter false alarms and 2 more" {
		t.Fatalf("log %v", log)
	}
	tv.Clean()
	b, root := load(tv, "Filter false alarms")
	parts := b.Parts(root)
	if len(parts) != 2 || parts[0].Title() != "Score boxes by motion" || parts[1].Front.Int("order") != 2 {
		t.Fatalf("parts %v", parts)
	}
	if root.Str("description") != "Fewer false alarms." || root.Str("parts") != "0/2" || root.Str("root") != "[[Filter false alarms]]" || root.Str("status") != "open" {
		t.Fatalf("root fields: %s", root.Content)
	}
	for _, want := range []string{"> [!spec] Open · plan · normal", "## Parts", "| 1 | [[Score boxes by motion]] | open | [[p3-edge]] |  |", "## History", "list(subject).contains(this.file.asLink())"} {
		if !strings.Contains(root.Content, want) {
			t.Errorf("root lacks %q:\n%s", want, root.Content)
		}
	}
	if !strings.Contains(tv.Read("wiki/documents/Tune the threshold.md"), "Depends on [[Score boxes by motion]] (open)") {
		t.Fatal("a part's callout names its dependency")
	}
	if n := b.Next(root); n != "start Score boxes by motion" {
		t.Fatalf("next %q", n)
	}
	if !b.Ready(parts[0]) || b.Ready(parts[1]) {
		t.Fatal("ready")
	}
	if _, err := work.Specs(tv.V, work.SpecsIn{Specs: []work.SpecIn{{Title: "X", Kind: "plan", Parent: "Filter false alarms", Depends: []string{"Unrelated"}}}}, opts(tv)); err == nil {
		t.Fatal("a dependency must be a sibling")
	}
	if _, err := work.Specs(tv.V, work.SpecsIn{Specs: []work.SpecIn{{Title: "Filter false alarms", Kind: "plan"}}}, opts(tv)); err == nil || !strings.Contains(err.Error(), "held by") {
		t.Fatalf("titles are unique: %v", err)
	}
	if _, err := work.Specs(tv.V, work.SpecsIn{Specs: []work.SpecIn{{Title: "View · Mine", Kind: "plan"}}}, opts(tv)); err == nil {
		t.Fatal("a view's title is refused")
	}
}

func TestStartCompleteAndTheStatusFromEvents(t *testing.T) {
	tv := setup(t)
	if _, err := work.Start(tv.V, "Filter false alarms", false, opts(tv)); err == nil || !strings.Contains(err.Error(), "start a part") {
		t.Fatalf("a plan with parts is started through a part: %v", err)
	}
	if _, err := work.Start(tv.V, "Tune the threshold", false, opts(tv)); err == nil || !strings.Contains(err.Error(), "depends on Score boxes by motion") {
		t.Fatalf("a dependency must be done: %v", err)
	}
	res := ok(t)(work.Start(tv.V, "Score boxes by motion", false, opts(tv)))
	if res.Started != "Score boxes by motion" || len(res.Events) != 2 {
		t.Fatalf("start %+v", res)
	}
	b, part := load(tv, "Score boxes by motion")
	_, root := load(tv, "Filter false alarms")
	if b.Status(part) != work.Started || b.Status(root) != work.Started || part.Str("status") != "started" {
		t.Fatal("a start starts the part and the plan above it")
	}
	if !strings.Contains(part.Content, "> [!spec] Started · plan · normal") || !strings.Contains(part.Content, "[[Filter false alarms]] › **Score boxes by motion** · [[p3-edge]]") {
		t.Fatalf("callout:\n%s", part.Content)
	}
	ev := tv.Read(res.Events[0].Path)
	for _, want := range []string{"kind: started", "subject: \"[[Score boxes by motion]]\"", "by: agent", "> [!event-started] Started · [[Score boxes by motion]]", "part of [[Filter false alarms]]"} {
		if !strings.Contains(ev, want) {
			t.Errorf("event lacks %q:\n%s", want, ev)
		}
	}
	if !strings.HasPrefix(res.Events[0].Title, "Score boxes by motion · started 2026-09-27 ") {
		t.Fatalf("event title %s", res.Events[0].Title)
	}
	again := ok(t)(work.Start(tv.V, "Score boxes by motion", false, opts(tv)))
	if len(again.Events) != 1 || again.Events[0].Kind != "continued" {
		t.Fatalf("a second start continues: %+v", again.Events)
	}
	if _, err := work.Complete(tv.V, "Score boxes by motion", work.ResultIn{Delivered: "x"}, opts(tv)); err == nil {
		t.Fatal("done needs verified")
	}
	done := ok(t)(work.Complete(tv.V, "Score boxes by motion", work.ResultIn{Delivered: "Scored boxes in `a1b2c3`.", Verified: "go test ./... passes", Learned: "Motion helps."}, opts(tv)))
	result := tv.Read(done.Events[0].Path)
	if !strings.Contains(result, "## Delivered\n\nScored boxes in `a1b2c3`.") || !strings.Contains(result, "## Learned") || strings.Contains(result, "## Follow-ups") {
		t.Fatalf("result:\n%s", result)
	}
	b, root = load(tv, "Filter false alarms")
	if root.Str("parts") != "1/2" || b.Next(root) != "start Tune the threshold" {
		t.Fatalf("parts %s next %s", root.Str("parts"), b.Next(root))
	}
	if !strings.Contains(tv.Read("wiki/documents/Score boxes by motion.md"), "> [!spec-done] Done 2026-09-27 → [["+done.Events[0].Title+"|result]]") {
		t.Fatal("a done plan links its result")
	}
	if _, err := work.Complete(tv.V, "Filter false alarms", work.ResultIn{Delivered: "a", Verified: "b"}, opts(tv)); err == nil || !strings.Contains(err.Error(), "Tune the threshold") {
		t.Fatalf("a plan with an open part is not done: %v", err)
	}
	// A hand edit works: delete the completed event, and the plan is started again.
	if err := os.Remove(tv.V.Abs(done.Events[0].Path)); err != nil {
		t.Fatal(err)
	}
	if b, part := load(tv, "Score boxes by motion"); b.Status(part) != work.Started {
		t.Fatal("the status follows the events")
	}
}

func TestHoldersDropReopenBlock(t *testing.T) {
	tv := setup(t)
	ok(t)(work.Start(tv.V, "Score boxes by motion", false, opts(tv)))
	tv.Write("sessions/2026-09/2026-09-27 1432 aaaaaa.md", "---\nid: ses-aaaaaa\ntype: session\nstatus: running\nstarted: 2026-09-27T14:32:00\nspecs: [\"[[Score boxes by motion]]\"]\n---\n")
	if _, err := work.Start(tv.V, "Score boxes by motion", false, opts(tv)); err == nil || !strings.Contains(err.Error(), "take: true") {
		t.Fatalf("a plan a live session holds: %v", err)
	}
	ok(t)(work.Start(tv.V, "Score boxes by motion", true, opts(tv)))
	if b, part := load(tv, "Score boxes by motion"); !b.Active(part) || part.Str("active") != "true" {
		t.Fatal("active")
	}
	ok(t)(work.Block(tv.V, "Tune the threshold", "waits on the new camera", opts(tv)))
	if b, d := load(tv, "Tune the threshold"); b.Blocked(d) != "waits on the new camera" || d.Str("blocked") != "waits on the new camera" {
		t.Fatal("blocked")
	}
	ok(t)(work.Unblock(tv.V, "Tune the threshold", opts(tv)))
	res := ok(t)(work.Drop(tv.V, "Filter false alarms", "The camera vendor fixed it.", opts(tv)))
	if len(res.Events) != 3 {
		t.Fatalf("a drop drops each open part: %d events", len(res.Events))
	}
	b, root := load(tv, "Filter false alarms")
	if b.Status(root) != work.Dropped || !strings.Contains(root.Content, "> [!spec-dropped] Dropped") {
		t.Fatalf("dropped:\n%s", root.Content)
	}
	if _, err := work.Specs(tv.V, work.SpecsIn{Specs: []work.SpecIn{{Title: "More", Kind: "plan", Parent: "Filter false alarms"}}}, opts(tv)); err == nil || !strings.Contains(err.Error(), "reopen") {
		t.Fatalf("a dropped plan takes no part: %v", err)
	}
	ok(t)(work.Reopen(tv.V, "Filter false alarms", "It came back.", opts(tv)))
	if b, root := load(tv, "Filter false alarms"); b.Status(root) != work.Open {
		t.Fatal("reopened")
	}
}

func TestStubPromoteAndResolve(t *testing.T) {
	tv := setup(t)
	tv.Write("inbox/idea.md", "try dinov2\n")
	res := ok(t)(work.Stub(tv.V, work.StubIn{Text: "Try DINOv2 features to filter vehicle false alarms. They may help.", Tags: []string{"ML", "work/p3"}, Inbox: "idea.md"}, opts(tv)))
	stub := tv.Read(res.View.Doc.Path)
	if res.View.Doc.Title != "Try DINOv2 features to filter vehicle false alarms. They" || tv.V.Exists("inbox/idea.md") {
		t.Fatalf("title %q", res.View.Doc.Title)
	}
	for _, want := range []string{"type: stub", "description: Try DINOv2 features to filter vehicle false alarms.", "tags: [ml, work/p3]", "status: open", "> [!stub] Open · normal · planted 2026-09-27", "#ml · #work/p3", "## Idea\n\nTry DINOv2"} {
		if !strings.Contains(stub, want) {
			t.Errorf("stub lacks %q:\n%s", want, stub)
		}
	}
	if res.View.Next != "write" {
		t.Fatal("next")
	}
	id := res.View.Doc.ID
	tv.Write("Notes.md", "See [[Try DINOv2 features to filter vehicle false alarms. They]].\n")
	tv.Commit()
	p := ok(t)(work.Promote(tv.V, work.PromoteIn{Stub: id, Kind: "plan", Title: "Try DINOv2 for alarms", Text: "## Goal\n\nTry it.\n\n## Done when\n\n- measured\n", Repositories: []string{"p3-edge"}}, opts(tv)))
	spec := tv.Read("wiki/documents/Try DINOv2 for alarms.md")
	if p.View.Doc.ID != id || p.View.Doc.Type != "spec" || !strings.Contains(spec, "## Origin\n\nTry DINOv2 features") || !strings.Contains(spec, "tags: [ml, work/p3]") {
		t.Fatalf("promote:\n%s", spec)
	}
	if tv.Read("Notes.md") != "See [[Try DINOv2 for alarms]].\n" {
		t.Fatalf("links follow: %s", tv.Read("Notes.md"))
	}
	if len(p.Events) != 1 || p.Events[0].Kind != "promoted" || !strings.Contains(tv.Read(p.Events[0].Path), "Promoted from stub to spec") {
		t.Fatal("promoted event")
	}
	s2 := ok(t)(work.Stub(tv.V, work.StubIn{Text: "Read the OTA paper", Title: "Read the OTA paper"}, opts(tv)))
	ok(t)(work.Specs(tv.V, work.SpecsIn{Resolve: true, Specs: []work.SpecIn{{Title: "OTA reading notes", Kind: "design", From: s2.View.Doc.ID, Text: "## Purpose\n\nKnow OTA.\n"}}}, opts(tv)))
	b, stubDoc := load(tv, "Read the OTA paper")
	if b.Status(stubDoc) != work.Resolved || !strings.Contains(stubDoc.Content, "Resolved 2026-09-27 → [[OTA reading notes]]") {
		t.Fatalf("resolved:\n%s", stubDoc.Content)
	}
	if _, d := load(tv, "OTA reading notes"); d.Str("from") != "[[Read the OTA paper]]" || d.Str("status") != "current" || !strings.Contains(d.Content, "> [!design] Current · design") {
		t.Fatalf("design:\n%s", d.Content)
	}
	tv.Clean()
}

func TestKnownTagsAndSet(t *testing.T) {
	tv := setup(t)
	tv.Write("Atlas.md", strings.Replace(tv.Read("Atlas.md"), "tagging: open", "tagging: known", 1))
	tv.Commit()
	v, err := vault.Open(tv.V.Root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := work.Stub(v, work.StubIn{Text: "x", Tags: []string{"brand-new"}}, opts(tv)); err == nil || !strings.Contains(err.Error(), "new_tags") {
		t.Fatalf("known tags: %v", err)
	}
	ok(t)(work.Stub(v, work.StubIn{Text: "x", Tags: []string{"work/p3", "brand-new"}, NewTags: true}, opts(tv)))
	title := "Scoring"
	ok(t)(work.Set(v, work.SetIn{Doc: "Score boxes by motion", Title: &title}, opts(tv)))
	if !strings.Contains(tv.Read("wiki/documents/Tune the threshold.md"), "depends: [\"[[Scoring]]\"]") {
		t.Fatal("a rename rewrites the links in fields")
	}
	deps := []string{"Tune the threshold"}
	if _, err := work.Set(v, work.SetIn{Doc: "Scoring", Depends: &deps}, opts(tv)); err == nil || !strings.Contains(err.Error(), "wait on each other") {
		t.Fatalf("a loop of depends: %v", err)
	}
}
