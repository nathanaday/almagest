package views_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/nathanaday/almagest/internal/doc"
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
	// The navigator Base of an older version, as Obsidian saves it: without the notice.
	tv.Write("wiki-view/View · Navigator.base", "filters:\n  and: []\nviews: []\n")
	wrote, _, err := views.Write(tv.Index(), testvault.Now.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if tv.V.Exists("wiki-view/nav/gone/Tag · gone.md") || tv.V.Exists("wiki-view/nav") || tv.V.Exists("wiki-view/Old.base") || tv.V.Exists("wiki-view/View · Navigator.base") {
		t.Fatal("a view that stands for nothing goes, with its folder")
	}
	if !tv.V.Exists("wiki-view/Mine.base") {
		t.Fatal("a Base without the notice is the user's")
	}
	checks := map[string][]string{
		"wiki-view/View · Home.md": {"---\ncssclasses: [almagest-home]\n",
			`start_here: ["[[tool/source-core/documents/CS513]]"]`,
			`under: {"tool/source-core/documents/CS513.md": 3}`,
			`todo: ["[[Notes]]"]`, `todo_lines: {"Notes.md": "buy a lidar #todo"}`,
			"journals: []\njournal_lines: {}\n---\n",
			views.Notice, "> [!almagest] Work", "> 3 topics · 0 sources · 2 repositories\n> Views: [[View · Library]] · [[View · Timeline]] · [[View · Repositories]]",
			"## Waiting for you\n\n```base\nfilters:\n  or:\n    - 'file.inFolder(\"changes\") && (status == \"proposed\" || status == \"running\")'",
			"## To-do lines\n\n```base", "  lines: 'this.todo_lines[file.path]'",
			"## Start here\n\nOpen one, then Navigator in the Almagest palette: it shows the documents above, beside, and below the note you read.\n\n```base",
			"    - 'list(this.start_here).contains(file.asLink())'", "  under: 'this.under[file.path]'",
			"```\n\n1 document with nothing above or below.\n\n## Recent\n\n```base",
			"    limit: 10\n", `  applied: 'date(applied).format("YYYY-MM-DD HH:mm")'`},
		"wiki-view/View · Timeline.md":     {"### 2026-09-27", "15:32 · change applied · [[2026-09-27 Add lidar]]"},
		"wiki-view/View · Repositories.md": {"## [[grid-sim]]\n\nThe grid simulator.\n\n`~/code/grid-sim`\n\nTag: [[grid-sim|#school/cs513/grid-sim]] · Under: [[CS513|#school/cs513]] · Also: #simulation\n\n```almagest-repo\n", "## Unlinked\n\n- [[old-sim]] · The first simulator."},
		"wiki-view/View · Home.md#":        {"[[View · Repositories]]"},
		"wiki-view/View · Library.md":      {"## Topics", "## Needs care"},
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
	if strings.Contains(home, "| ---") || strings.Contains(home, "## Views") {
		t.Fatalf("Home is Bases, not tables, and names the views in its callout:\n%s", home)
	}
	checkBases(t, home)
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
	front, _, _ := doc.Split(home)
	f, err := doc.ParseFront(front)
	if err != nil {
		t.Fatal(err)
	}
	start := f.List("start_here")
	if len(start) != views.MaxRoots || start[0] != "[[tool/source-core/documents/Machine Learning]]" || start[1] != "[[tool/source-core/documents/Autonomous Driving]]" || start[2] != "[[tool/source-core/documents/Root 00]]" {
		t.Fatalf("start_here holds the 20 roots, the most under first: %v", start)
	}
	var fields struct{ Under map[string]any }
	if err := yaml.Unmarshal([]byte(front), &fields); err != nil {
		t.Fatal(err)
	}
	under := fields.Under
	if under["tool/source-core/documents/Machine Learning.md"] != 3 || under["tool/source-core/documents/Root 17.md"] != 1 || len(under) != views.MaxRoots {
		t.Fatalf("under counts the documents below each root, as numbers: %v", under)
	}
	if !strings.Contains(home, "3 more with documents under them, and 1 document with nothing above or below.\n") {
		t.Fatalf("Start here holds 20 roots and a line for the rest:\n%s", home)
	}
	// Block 7 Slides waits to be absorbed.
	if !strings.Contains(home, "## Waiting for you") || !strings.Contains(home, `type == "source" && status == "pending"`) {
		t.Fatalf("Waiting for you lists the pending sources:\n%s", home)
	}
	checkBases(t, home)
}

// checkBases parses every inline Base of a note as YAML, with a table view.
func checkBases(t *testing.T, note string) {
	t.Helper()
	blocks := strings.Split(note, "```base\n")[1:]
	if len(blocks) == 0 {
		t.Fatalf("no Base:\n%s", note)
	}
	for _, b := range blocks {
		text, _, _ := strings.Cut(b, "```")
		var base struct {
			Filters map[string]any
			Views   []struct{ Type, Name string }
		}
		if err := yaml.Unmarshal([]byte(text), &base); err != nil || len(base.Views) != 1 || base.Views[0].Type != "table" || len(base.Filters) != 1 {
			t.Fatalf("a Base that does not parse (%v):\n%s", err, text)
		}
	}
}
