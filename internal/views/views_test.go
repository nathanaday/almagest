package views_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/views"
	"github.com/nathanaday/atlas-obsidian/internal/work"
)

func TestViews(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("topic", "CS513", map[string]any{"kind": "overview", "defines": "school/cs513", "tags": []string{"school"}}, "")
	tv.Doc("topic", "Occupancy grids", map[string]any{"kind": "concept", "tags": []string{"school/cs513", "self-driving", "project"}}, "")
	tv.Doc("topic", "Lidar", map[string]any{"kind": "entity", "tags": []string{"school/cs513", "self-driving"}}, "")
	tv.Write("Notes.md", "- [ ] buy a lidar #todo\n- [x] done #todo\n- [ ] @atlas add the grid paper\n```\n- [ ] #todo in code\n```\n")
	tv.Commit()
	o := work.Opts{Now: testvault.Now}
	if _, err := work.Specs(tv.V, work.SpecsIn{Specs: []work.SpecIn{{Title: "Build the grid", Kind: "plan", Tags: []string{"school/cs513"}, Text: "## Done when\n\n- built\n"}}}, o); err != nil {
		t.Fatal(err)
	}
	if _, err := work.Start(tv.V, "Build the grid", false, work.Opts{Now: testvault.Now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	tv.Write("views/tags/gone/Tag · gone.md", "old\n")
	wrote, err := views.Write(tv.Index(), testvault.Now.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if tv.V.Exists("views/tags/gone/Tag · gone.md") || tv.V.Exists("views/tags/gone") {
		t.Fatal("a view that stands for nothing goes, with its folder")
	}
	checks := map[string][]string{
		"views/View · Home.md":                            {views.Notice, "> [!atlas] Work", "## Tags", "[[Tag · school|#school]] · 5", "- [[Notes]]: @atlas add the grid paper", "## Recent", "started · [[Build the grid]]"},
		"views/View · Work.md":                            {"## Plans", "```base", `kind == "plan"`, "formulas:\n  rank: 'if(priority == \"high\", 1,", "property: formula.rank\n        direction: ASC", "## To-do lines\n\n- [[Notes]]: buy a lidar #todo\n\n## Mentions", "## Stubs"},
		"views/View · Timeline.md":                        {"### 2026-09-27", "15:32 · started · [[Build the grid]]", "14:32 · written · [[Build the grid]] · #school/cs513"},
		"views/View · Library.md":                         {"## Topics", "## Needs care"},
		"views/tags/school/cs513/Tag · school › cs513.md": {"> [!tag] #school/cs513 · 5 documents", "Page: [[CS513]]", "Under: [[Tag · school|#school]]", "## Narrow", "[self-driving (2)](obsidian://search?vault=work&query=tag%3A%23school%2Fcs513+tag%3A%23self-driving)", "## Open work", "## Topics", "## History", `file.hasTag("school/cs513")`},
		"views/tags/school/Tag · school.md":               {`file.hasTag("school", "school/cs513")`, "Below: [[Tag · school › cs513|cs513]] (5)"},
	}
	for rel, wants := range checks {
		got := tv.Read(rel)
		for _, w := range wants {
			if !strings.Contains(got, w) {
				t.Errorf("%s lacks %q:\n%s", rel, w, got)
			}
		}
	}
	if strings.Contains(tv.Read("views/View · Work.md"), "in code") || strings.Contains(tv.Read("views/View · Work.md"), "- [x]") {
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
