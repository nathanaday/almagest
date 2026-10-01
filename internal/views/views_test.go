package views_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/thread"
	"github.com/nathanaday/atlas-obsidian/internal/views"
)

func TestViews(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("topic", "CS513", map[string]any{"kind": "overview", "defines": "school/cs513", "tags": []string{"school"}}, "")
	tv.Doc("topic", "Occupancy grids", map[string]any{"kind": "concept", "tags": []string{"school/cs513", "self-driving", "project"}}, "")
	tv.Doc("topic", "Lidar", map[string]any{"kind": "entity", "tags": []string{"school/cs513", "self-driving"}}, "")
	tv.Doc("repository", "grid-sim", map[string]any{"path": "~/code/grid-sim", "defines": "school/cs513/grid-sim", "tags": []string{"school/cs513", "simulation"}, "description": "The grid simulator."}, "")
	tv.Doc("repository", "old-sim", map[string]any{"path": "", "unlinked": true, "description": "The first simulator."}, "")
	tv.Write("Notes.md", "- [ ] buy a lidar #todo\n- [x] done #todo\n- [ ] @atlas add the grid paper\n```\n- [ ] #todo in code\n```\n")
	tv.Commit()
	at := testvault.Now
	step := func(_ *thread.Result, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		at = at.Add(10 * time.Minute)
	}
	step(thread.ChordCreate(tv.V, thread.ChordIn{Title: "Grid project", Text: "The grid runs in the simulator.", Tags: []string{"school/cs513"}, Threads: []thread.ChordThreadIn{
		{Title: "Build the grid", Text: "Build the occupancy grid."},
		{Title: "Show the grid", Text: "Draw it.", After: []string{"Build the grid"}},
	}}, thread.Opts{Now: at}))
	step(thread.Stub(tv.V, thread.StubIn{Text: "Read the lidar paper", Title: "Lidar paper"}, thread.Opts{Now: at}))
	step(thread.Spec(tv.V, thread.SpecIn{Thread: "Build the grid", Text: "## Goal\n\nA grid.\n\n## Requirements\n\n- R1: The grid is built.\n\n## Knowledge\n\n- [[Occupancy grids]]\n"}, thread.Opts{Now: at}))
	step(thread.TasksWrite(tv.V, thread.TasksIn{Thread: "Build the grid", Repository: "grid-sim", Tasks: []thread.TaskIn{{Text: "Build it", Requirements: []string{"R1"}}}}, thread.Opts{Now: at}))
	step(thread.Start(tv.V, "Build the grid", false, thread.Opts{Now: testvault.Now.Add(time.Hour)}))
	tv.Write("views/tags/gone/Tag · gone.md", "old\n")
	wrote, err := views.Write(tv.Index(), testvault.Now.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if tv.V.Exists("views/tags/gone/Tag · gone.md") || tv.V.Exists("views/tags/gone") {
		t.Fatal("a view that stands for nothing goes, with its folder")
	}
	checks := map[string][]string{
		"views/View · Home.md": {views.Notice, "> [!atlas] Work", "2 stubs · 1 open thread · 1 open chord", "## Tags", "[[Tag · school|#school]] · 10", "[[View · Threads]]", "- [[Notes]]: @atlas add the grid paper", "## Recent", "started · [[Build the grid]]"},
		"views/View · Threads.md": {"> [!info]- What is a thread?", "> [!info]- What is a chord?", "`Resume Atlas chord doc-…`",
			"## Chords\n\n### [[Grid project]]\n\nstarted · 0/2 threads closed · The grid runs in the simulator.",
			"| 1 | [[Build the grid]] | started | 0/1 |  | [[grid-sim]] |", "| 2 | [[Show the grid]] | stub |  | [[Build the grid]] |  |", "Canvas: [[chords/Grid project.canvas|the order as a graph]]",
			"## Threads in no chord\n\n```base", "- '!chord'", "formulas:\n  stage: 'if(status == \"started\", 1,", "property: formula.rank\n        direction: ASC",
			"## To-do lines\n\n- [[Notes]]: buy a lidar #todo\n\n## Mentions"},
		"views/View · Timeline.md":                        {"### 2026-09-27", "15:32 · started · [[Build the grid]]", "14:32 · planted · [[Build the grid]] · #school/cs513", "14:32 · chord made · [[Grid project]]", "spec written · [[Build the grid · Spec]]"},
		"views/View · Repositories.md":                    {"## [[grid-sim]]\n\nThe grid simulator.\n\n`~/code/grid-sim`\n\nTag: [[Tag · school › cs513 › grid-sim|#school/cs513/grid-sim]] · Under: [[Tag · school › cs513|#school/cs513]] · Also: [[Tag · simulation|#simulation]]\n\nThreads: [[Build the grid]] (started)\n\n```atlas-repo\n", "## Unlinked\n\n- [[old-sim]] · The first simulator."},
		"views/View · Home.md#":                           {"[[View · Repositories]]"},
		"views/View · Library.md":                         {"## Topics", "## Needs care"},
		"views/tags/school/cs513/Tag · school › cs513.md": {"> [!tag] #school/cs513 · 10 documents", "Page: [[CS513]]", "Under: [[Tag · school|#school]]", "## Narrow", "[self-driving (2)](obsidian://search?vault=work&query=tag:%23school%2Fcs513%20tag:%23self-driving)", "## Open threads", "## Topics", "## History", `file.hasTag("school/cs513", "school/cs513/grid-sim")`},
		"views/tags/school/Tag · school.md":               {`file.hasTag("school", "school/cs513", "school/cs513/grid-sim")`, "Below: [[Tag · school › cs513|cs513]] (10)"},
	}
	for rel, wants := range checks {
		got := tv.Read(strings.TrimSuffix(rel, "#"))
		for _, w := range wants {
			if !strings.Contains(got, w) {
				t.Errorf("%s lacks %q:\n%s", rel, w, got)
			}
		}
	}
	if strings.Contains(tv.Read("views/View · Threads.md"), "in code") || strings.Contains(tv.Read("views/View · Threads.md"), "- [x]") {
		t.Fatal("only open task lines outside code")
	}
	if len(wrote) < 8 {
		t.Fatalf("wrote %v", wrote)
	}
	again, _ := views.Write(tv.Index(), testvault.Now.Add(2*time.Hour))
	if len(again) != 0 {
		t.Fatalf("a second write writes nothing: %v", again)
	}
	tv.Clean()
}
