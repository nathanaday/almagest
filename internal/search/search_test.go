package search_test

import (
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/search"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
)

func TestRankingAndFilters(t *testing.T) {
	tv := testvault.New(t)
	tv.Page("area", "p3", map[string]any{"description": "The p3 product."}, "")
	tv.Page("area", "home", nil, "")
	tv.Page("repository", "p3-cloud", map[string]any{"parent": "[[p3]]", "path": "/x", "description": "The p3 cloud front end: a React app.", "aliases": []string{"p3 cloud"}}, "")
	tv.Page("repository", "p3-edge", map[string]any{"parent": "[[p3]]", "path": "/y", "description": "The edge service."}, "")
	tv.Page("concept", "Over the air updates", map[string]any{"scope": "[[p3]]", "description": "How devices update their firmware remotely."}, "## Definition\n\nRemote updates ship firmware to devices.\n")
	tv.Page("concept", "Gardening", map[string]any{"scope": "[[home]]", "description": "Growing things."}, "Update the garden.\n")
	tv.Write("threads/Fix alarms/Fix alarms.md", "---\nid: thr-aaaaaa\ntype: stub\ncreated: 2026-09-27\nupdated: 2026-09-27\nscope: [\"[[p3-edge]]\"]\nstage: stub\n---\n## Stub\n\nvehicle false alarms\n")
	tv.Write("threads/Old/Old.md", "---\nid: thr-bbbbbb\ntype: stub\ncreated: 2026-09-20\nupdated: 2026-09-20\nstage: closed\n---\n## Stub\n\nvehicle alarms long ago\n")
	idx := tv.Index()

	hits, err := search.Search(idx, search.Query{Text: "p3 cloud front end", Types: []string{"repository", "area"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits.Hits) == 0 || hits.Hits[0].Ref.Title != "p3-cloud" {
		t.Fatalf("the repository ranks first: %+v", hits.Hits)
	}
	hits, _ = search.Search(idx, search.Query{Text: "remote update", Scope: "p3"})
	if len(hits.Hits) != 1 || hits.Hits[0].Ref.Title != "Over the air updates" || hits.Hits[0].Snippet == "" {
		t.Fatalf("scope keeps home out: %+v", hits.Hits)
	}
	hits, _ = search.Search(idx, search.Query{Text: "vehicle alarms", Types: []string{"stub"}, State: map[string][]string{"stage": {"stub", "spec", "tasks"}}})
	if len(hits.Hits) != 1 || hits.Hits[0].Ref.Title != "Fix alarms" {
		t.Fatalf("open threads only: %+v", hits.Hits)
	}
	hits, _ = search.Search(idx, search.Query{Types: []string{"repository"}, Scope: "p3"})
	if hits.Total != 2 {
		t.Fatalf("filters alone list: %+v", hits)
	}
	if _, err := search.Search(idx, search.Query{Types: []string{"card"}}); err == nil {
		t.Fatal("an unknown type is refused")
	}
	if _, err := search.Search(idx, search.Query{Scope: "Gardening"}); err == nil {
		t.Fatal("a scope that is no scope page is refused")
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
