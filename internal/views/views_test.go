package views_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/almagest/internal/testvault"
	"github.com/nathanaday/almagest/internal/views"
)

func TestViews(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("topic", "CS513", map[string]any{"kind": "overview", "defines": "school/cs513", "tags": []string{"school"}}, "")
	tv.Doc("topic", "Occupancy grids", map[string]any{"kind": "concept", "tags": []string{"school/cs513", "self-driving", "project"}}, "")
	tv.Doc("topic", "Lidar", map[string]any{"kind": "entity", "tags": []string{"school/cs513", "self-driving"}}, "")
	tv.Doc("repository", "grid-sim", map[string]any{"path": "~/code/grid-sim", "defines": "school/cs513/grid-sim", "tags": []string{"school/cs513", "simulation"}, "description": "The grid simulator."}, "")
	tv.Doc("repository", "old-sim", map[string]any{"path": "", "unlinked": true, "description": "The first simulator."}, "")
	tv.Write("Notes.md", "- [ ] buy a lidar #todo\n- [x] done #todo\n```\n- [ ] #todo in code\n```\n")
	tv.Write("changes/2026-09/2026-09-27 Add lidar.md", "---\nid: chg-aaaaaa\ntype: change\ncreated: 2026-09-27T15:00:00\nupdated: 2026-09-27T15:32:00\nstatus: applied\napplied: 2026-09-27T15:32:00\ncounts: 1 create, 0 modify, 0 remove\n---\n\n## Notes\n\nAdds lidar.\n")
	tv.Write("changes/2026-09/2026-09-27 Add grids.md", "---\nid: chg-bbbbbb\ntype: change\ncreated: 2026-09-27T16:00:00\nupdated: 2026-09-27T16:00:00\nstatus: proposed\nproposed: 2026-09-27T16:00:00\ncounts: 0 create, 1 modify, 0 remove\n---\n\n## Notes\n\nGrids.\n")
	tv.Commit()
	tv.Write("wiki-view/nav/gone/Tag · gone.md", views.Notice+"\n\nold\n")
	wrote, _, err := views.Write(tv.Index(), testvault.Now.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if tv.V.Exists("wiki-view/nav/gone/Tag · gone.md") || tv.V.Exists("wiki-view/nav/gone") {
		t.Fatal("a view that stands for nothing goes, with its folder")
	}
	checks := map[string][]string{
		"wiki-view/View · Home.md":                           {views.Notice, "> [!almagest] Work", "## Waiting for you\n\n- [[2026-09-27 Add grids]] · proposed change", "## To-do lines\n\n- [[Notes]]: buy a lidar #todo\n\n## Tags", "[[Tag · school|#school]] · 4", "## Recent", "change applied · [[2026-09-27 Add lidar]]"},
		"wiki-view/View · Timeline.md":                       {"### 2026-09-27", "15:32 · change applied · [[2026-09-27 Add lidar]]"},
		"wiki-view/View · Repositories.md":                   {"## [[grid-sim]]\n\nThe grid simulator.\n\n`~/code/grid-sim`\n\nTag: [[Tag · school › cs513 › grid-sim|#school/cs513/grid-sim]] · Under: [[Tag · school › cs513|#school/cs513]] · Also: [[Tag · simulation|#simulation]]\n\n```almagest-repo\n", "## Unlinked\n\n- [[old-sim]] · The first simulator."},
		"wiki-view/View · Home.md#":                          {"[[View · Repositories]]"},
		"wiki-view/View · Library.md":                        {"## Topics", "## Needs care"},
		"wiki-view/nav/school/cs513/Tag · school › cs513.md": {"> [!tag] #school/cs513 · 4 documents", "Page: [[CS513]]", "Under: [[Tag · school|#school]]", "## Narrow", "[self-driving (2)](obsidian://search?vault=work&query=tag:%23school%2Fcs513%20tag:%23self-driving)", "## Topics", "## Repositories", `file.hasTag("school/cs513", "school/cs513/grid-sim")`},
		"wiki-view/nav/school/Tag · school.md":               {`file.hasTag("school", "school/cs513", "school/cs513/grid-sim")`, "Below: [[Tag · school › cs513|cs513]] (4)"},
	}
	for rel, wants := range checks {
		got := tv.Read(strings.TrimSuffix(rel, "#"))
		for _, w := range wants {
			if !strings.Contains(got, w) {
				t.Errorf("%s lacks %q:\n%s", rel, w, got)
			}
		}
	}
	if _, ok := views.Render(tv.Index(), testvault.Now)["wiki-view/View · Threads.md"]; ok || tv.V.Exists("wiki-view/View · Threads.md") {
		t.Fatal("no threads view")
	}
	home := tv.Read("wiki-view/View · Home.md")
	if strings.Contains(home, "in code") || strings.Contains(home, "- [x]") || strings.Contains(home, "Threads") {
		t.Fatalf("only open task lines outside code, and no threads:\n%s", home)
	}
	if strings.Contains(tv.Read("wiki-view/View · Timeline.md"), "Add grids") {
		t.Fatal("the timeline lists applied changes only")
	}
	for _, rel := range []string{"wiki-view/View · Repositories.md", "wiki-view/nav/school/cs513/Tag · school › cs513.md"} {
		if got := tv.Read(rel); strings.Contains(got, "Threads") || strings.Contains(got, "Open threads") || strings.Contains(got, "## History") {
			t.Errorf("%s names threads or events:\n%s", rel, got)
		}
	}
	if len(wrote) < 8 {
		t.Fatalf("wrote %v", wrote)
	}
	again, _, _ := views.Write(tv.Index(), testvault.Now.Add(2*time.Hour))
	if len(again) != 0 {
		t.Fatalf("a second write writes nothing: %v", again)
	}
	tv.Clean()
}
