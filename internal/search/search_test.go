package search_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/schema"

	"github.com/nathanaday/atlas-obsidian/internal/search"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
)

func TestRankingFiltersAndFacets(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("repository", "p3-cloud", map[string]any{"path": "/x", "defines": "work/p3/p3-cloud", "tags": []string{"work/p3"}, "description": "The p3 cloud front end: a React app.", "aliases": []string{"p3 cloud"}}, "")
	tv.Doc("repository", "p3-edge", map[string]any{"path": "/y", "defines": "work/p3/p3-edge", "tags": []string{"work/p3"}, "description": "The edge service."}, "")
	tv.Doc("topic", "Over the air updates", map[string]any{"kind": "concept", "tags": []string{"work/p3"}, "description": "How devices update their firmware remotely."}, "## Definition\n\nRemote updates ship firmware to devices.\n")
	tv.Doc("topic", "Gardening", map[string]any{"kind": "concept", "tags": []string{"home"}, "description": "Growing things."}, "Update the garden.\n")
	tv.Doc("topic", "Occupancy grids", map[string]any{"kind": "concept", "tags": []string{"school/cs513", "self-driving", "project"}, "description": "A grid of cells."}, "")
	tv.Doc("topic", "Lidar", map[string]any{"kind": "entity", "tags": []string{"school/cs513", "self-driving"}, "description": "A sensor."}, "")
	tv.Doc("topic", "Kinematics", map[string]any{"kind": "concept", "tags": []string{"school/cs513"}, "description": "Motion."}, "")
	tv.Doc("stub", "Fix alarms", map[string]any{"status": "open", "tags": []string{"work/p3/p3-edge"}}, "## Idea\n\nvehicle false alarms\n")
	tv.Doc("stub", "Old idea", map[string]any{"status": "dropped"}, "## Idea\n\nvehicle alarms long ago\n")
	tv.Doc("spec", "Score boxes", map[string]any{"kind": "plan", "status": "started", "repositories": []string{"[[p3-edge]]"}}, "")
	idx := tv.Index()

	hits, err := search.Search(idx, search.Query{Text: "p3 cloud front end", Types: []string{"repository"}})
	if err != nil || len(hits.Hits) == 0 || hits.Hits[0].Ref.Title != "p3-cloud" {
		t.Fatalf("the repository ranks first: %+v %v", hits, err)
	}
	hits, _ = search.Search(idx, search.Query{Text: "remote update", Tags: []string{"work/p3"}})
	if len(hits.Hits) != 1 || hits.Hits[0].Ref.Title != "Over the air updates" || hits.Hits[0].Snippet == "" {
		t.Fatalf("a tag keeps home out: %+v", hits.Hits)
	}
	hits, _ = search.Search(idx, search.Query{Text: "vehicle alarms", Types: []string{"stub"}, Status: []string{"open"}})
	if len(hits.Hits) != 1 || hits.Hits[0].Ref.Title != "Fix alarms" {
		t.Fatalf("open stubs only: %+v", hits.Hits)
	}
	// The meeting of three tags.
	hits, _ = search.Search(idx, search.Query{Tags: []string{"school/cs513", "self-driving", "project"}})
	if hits.Total != 1 || hits.Hits[0].Ref.Title != "Occupancy grids" {
		t.Fatalf("intersection: %+v", hits)
	}
	hits, _ = search.Search(idx, search.Query{Tags: []string{"school"}})
	if hits.Total != 3 || hits.Facets.Tags["self-driving"] != 2 || hits.Facets.Tags["project"] != 1 || hits.Facets.Tags["school"] != 0 || hits.Facets.Types["topic"] != 3 {
		t.Fatalf("facets: %+v", hits.Facets)
	}
	// Words of a tag rank too.
	hits, _ = search.Search(idx, search.Query{Text: "cs513 self driving project"})
	if len(hits.Hits) == 0 || hits.Hits[0].Ref.Title != "Occupancy grids" {
		t.Fatalf("tag words: %+v", hits.Hits)
	}
	hits, _ = search.Search(idx, search.Query{Kinds: []string{"entity"}})
	if hits.Total != 1 {
		t.Fatalf("kinds: %+v", hits)
	}
	hits, _ = search.Search(idx, search.Query{Repository: "p3-edge"})
	if hits.Total != 3 {
		t.Fatalf("repository: the repository, the spec that names it, and the stub that holds its tag: %+v", hits)
	}
	for _, q := range []search.Query{{Types: []string{"card"}}, {Kinds: []string{"idea"}}, {Status: []string{"soon"}}, {Tags: []string{"bad tag!"}}, {Repository: "Gardening"}} {
		if _, err := search.Search(idx, q); err == nil {
			t.Fatalf("refused: %+v", q)
		}
	}
}

func TestTokens(t *testing.T) {
	got := search.Tokens("The Remote Updates of p3's policies, and the class")
	want := []string{"remote", "update", "p3", "policy", "class"}
	if len(got) != len(want) {
		t.Fatalf("%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%v", got)
		}
	}
}

// A verification's status is its verdict, and the status filter selects by it.
func TestTheStatusFilterSelectsAVerification(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("stub", "Fix alarms", map[string]any{"status": "unverified"}, "## Idea\n\nvehicle false alarms\n")
	tv.Doc("verification", "Fix alarms · Verification 1", map[string]any{"thread": "[[Fix alarms]]", "round": 1, "verdict": "pass"}, "## Scope\n\nx\n")
	tv.Doc("verification", "Fix alarms · Verification 2", map[string]any{"thread": "[[Fix alarms]]", "round": 2, "verdict": "findings"}, "## Scope\n\nx\n")
	hits, err := search.Search(tv.Index(), search.Query{Status: []string{"findings"}})
	if err != nil || hits.Total != 1 || hits.Hits[0].Ref.Title != "Fix alarms · Verification 2" {
		t.Fatalf("status findings: %+v %v", hits, err)
	}
}

// The hints of the search tool name exactly what the call accepts.
func TestTheSearchHintsFollowTheSchema(t *testing.T) {
	hint := func(field string) string {
		f, _ := reflect.TypeOf(search.Query{}).FieldByName(field)
		return f.Tag.Get("jsonschema")
	}
	// byType reads "type: a, b; type: c" into its values, checking each type.
	byType := func(text string) []string {
		var out []string
		for _, group := range strings.Split(text, "; ") {
			name, values, ok := strings.Cut(group, ": ")
			if !ok || schema.Get(name[strings.LastIndex(name, " ")+1:]) == nil {
				t.Fatalf("%q names no type", group)
			}
			out = append(out, strings.Split(values, ", ")...)
		}
		return out
	}
	same := func(field string, got, want []string) {
		for _, v := range want {
			if !slices.Contains(got, v) {
				t.Errorf("the %s hint lacks %q", field, v)
			}
		}
		for _, v := range got {
			if !slices.Contains(want, v) {
				t.Errorf("the %s hint names %q, which the call refuses", field, v)
			}
		}
	}
	types, rest, _ := strings.Cut(strings.TrimPrefix(hint("Types"), "document types: "), "; or ")
	same("types", strings.Split(types, ", "), schema.DocumentTypes)
	if !strings.HasPrefix(rest, "session, change. Empty: the nine document types") {
		t.Errorf("the types hint ends %q", rest)
	}
	same("kinds", byType(strings.TrimPrefix(hint("Kinds"), "kinds by type: ")), schema.AllKinds())
	same("status", byType(strings.TrimPrefix(hint("Status"), "the statuses to keep, by type: ")), schema.Statuses())
}

// A task list for a repository belongs to it.
func TestARepositoryHoldsItsTaskLists(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("repository", "p3-edge", map[string]any{"path": "/y"}, "")
	tv.Doc("stub", "Fix alarms", map[string]any{"status": "planned"}, "## Idea\n\nx\n")
	tv.Doc("tasks", "Fix alarms · Tasks (p3-edge)", map[string]any{"thread": "[[Fix alarms]]", "repository": "[[p3-edge]]"}, "## Tasks\n\n- [ ] T1 Do it\n")
	hits, err := search.Search(tv.Index(), search.Query{Repository: "p3-edge", Types: []string{"tasks"}})
	if err != nil || hits.Total != 1 {
		t.Fatalf("the task list of p3-edge: %+v %v", hits, err)
	}
	// The thread's spec and verification belong where the thread does.
	tv.Doc("stub", "Score boxes", map[string]any{"status": "unverified", "repositories": []string{"[[p3-edge]]"}}, "## Idea\n\nx\n")
	tv.Doc("spec", "Score boxes · Spec", map[string]any{"thread": "[[Score boxes]]"}, "## Goal\n\nx\n")
	tv.Doc("verification", "Score boxes · Verification 1", map[string]any{"thread": "[[Score boxes]]", "round": 1, "verdict": "pass"}, "## Scope\n\nx\n")
	hits, err = search.Search(tv.Index(), search.Query{Repository: "p3-edge", Types: []string{"spec", "verification"}})
	if err != nil || hits.Total != 2 {
		t.Fatalf("the spec and verification of a p3-edge thread: %+v %v", hits, err)
	}
}

// The facets list only statuses the filter takes.
func TestTheFacetsListOnlyStatusesTheFilterTakes(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("stub", "Fix alarms", map[string]any{"status": "planned"}, "## Idea\n\nx\n")
	tv.Doc("tasks", "Fix alarms · Tasks", map[string]any{"thread": "[[Fix alarms]]", "status": "0/1"}, "## Tasks\n\n- [ ] T1 Do it\n")
	hits, err := search.Search(tv.Index(), search.Query{})
	if err != nil || hits.Facets.Status["planned"] != 1 || len(hits.Facets.Status) != 1 {
		t.Fatalf("status facets: %+v %v", hits.Facets.Status, err)
	}
}
