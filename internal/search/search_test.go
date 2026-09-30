package search_test

import (
	"testing"

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
