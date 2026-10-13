package views_test

import (
	"fmt"
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
	tv.Write("wiki-view/Old.base", views.BaseNotice+"\nviews: []\n")
	tv.Write("wiki-view/Mine.base", "views: []\n")
	wrote, _, err := views.Write(tv.Index(), testvault.Now.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if tv.V.Exists("wiki-view/nav/gone/Tag · gone.md") || tv.V.Exists("wiki-view/nav") || tv.V.Exists("wiki-view/Old.base") {
		t.Fatal("a view that stands for nothing goes, with its folder")
	}
	if !tv.V.Exists("wiki-view/Mine.base") {
		t.Fatal("a Base without the notice is the user's")
	}
	checks := map[string][]string{
		"wiki-view/View · Home.md":         {views.Notice, "> [!almagest] Work", "## Waiting for you\n\n- [[2026-09-27 Add grids]] · proposed change", "## To-do lines\n\n- [[Notes]]: buy a lidar #todo\n\n## Start here\n\n- [[CS513]] · 3 under it\n- 1 document with nothing above or below\n\n## Views\n\n- The navigator: the documents above, beside, and below the note you read. Run \"Almagest: Open the navigator\"", "## Recent", "change applied · [[2026-09-27 Add lidar]]"},
		"wiki-view/View · Timeline.md":     {"### 2026-09-27", "15:32 · change applied · [[2026-09-27 Add lidar]]"},
		"wiki-view/View · Repositories.md": {"## [[grid-sim]]\n\nThe grid simulator.\n\n`~/code/grid-sim`\n\nTag: [[grid-sim|#school/cs513/grid-sim]] · Under: [[CS513|#school/cs513]] · Also: #simulation\n\n```almagest-repo\n", "## Unlinked\n\n- [[old-sim]] · The first simulator."},
		"wiki-view/View · Home.md#":        {"[[View · Repositories]]"},
		"wiki-view/View · Library.md":      {"## Topics", "## Needs care"},
		"wiki-view/View · Navigator.base":  {views.BaseNotice, "groupBy:\n      property: formula.relation", `file.inFolder("tool/source-core/documents")`},
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
	for _, rel := range []string{"wiki-view/View · Repositories.md"} {
		if got := tv.Read(rel); strings.Contains(got, "Threads") || strings.Contains(got, "Open threads") || strings.Contains(got, "## History") {
			t.Errorf("%s names threads or events:\n%s", rel, got)
		}
	}
	if len(wrote) < 6 {
		t.Fatalf("wrote %v", wrote)
	}
	again, _, _ := views.Write(tv.Index(), testvault.Now.Add(2*time.Hour))
	if len(again) != 0 {
		t.Fatalf("a second write writes nothing: %v", again)
	}
	tv.Clean()
}

func TestStartHere(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("topic", "Machine Learning", map[string]any{"kind": "overview", "defines": "ml"}, "")
	tv.Doc("topic", "CS566", map[string]any{"kind": "overview", "defines": "ml/cs566", "tags": []string{"ml"}}, "")
	tv.Doc("topic", "Autonomous Driving", map[string]any{"kind": "overview", "defines": "ad"}, "")
	tv.Doc("topic", "Depth Project", map[string]any{"kind": "concept", "tags": []string{"ml/cs566"}, "up": []string{"[[CS566]]", "[[Autonomous Driving]]"}}, "")
	tv.Doc("topic", "Lidar", map[string]any{"kind": "concept", "tags": []string{"ad"}}, "")
	tv.Doc("source", "Block 7 Slides", map[string]any{"tags": []string{"ml/cs566"}}, "")
	tv.Doc("topic", "Loose", map[string]any{"kind": "concept", "tags": []string{"tool"}}, "")
	for i := range views.MaxRoots + 1 {
		tv.Doc("topic", fmt.Sprintf("Root %02d", i), map[string]any{"kind": "overview", "defines": fmt.Sprintf("r%02d", i)}, "")
		tv.Doc("topic", fmt.Sprintf("Child %02d", i), map[string]any{"kind": "concept", "tags": []string{fmt.Sprintf("r%02d", i)}}, "")
	}
	tv.Commit()
	home := views.Render(tv.Index(), testvault.Now)["wiki-view/View · Home.md"]
	want := "## Start here\n\n- [[Machine Learning]] · 3 under it\n- [[Autonomous Driving]] · 2 under it\n- [[Root 00]] · 1 under it\n"
	if !strings.Contains(home, want) {
		t.Fatalf("Start here lacks %q:\n%s", want, home)
	}
	if !strings.Contains(home, "- [[Root 17]] · 1 under it\n- 3 more with documents under them, and 1 document with nothing above or below\n") {
		t.Fatalf("Start here holds 20 roots and a line for the rest:\n%s", home)
	}
}

func TestNavigatorBase(t *testing.T) {
	tv := testvault.New(t)
	base := views.Render(tv.Index(), testvault.Now)[views.NavigatorPath]
	for _, want := range []string{
		`- 'list(this.up).contains(file.asLink())'`,
		`- '!this.defines.isEmpty() && up.isEmpty() && ("," + list(tags).join(",") + ",").contains(",#" + this.defines + ",")'`,
		`- '!this.up.isEmpty() && list(up).containsAny(list(this.up))'`,
		`"4 · Sources below", "3 · Below"`,
		"- file.path != this.file.path",
	} {
		if !strings.Contains(base, want) {
			t.Errorf("the navigator lacks %q:\n%s", want, base)
		}
	}
	if !strings.HasPrefix(base, views.BaseNotice) {
		t.Errorf("the navigator opens with the notice:\n%s", base)
	}
}
