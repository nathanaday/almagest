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
	tv.Doc("topic", "False alarms", map[string]any{"kind": "concept", "status": "draft", "tags": []string{"work/p3/p3-edge"}}, "## Definition\n\nvehicle false alarms\n")
	tv.Doc("topic", "Old alarms", map[string]any{"kind": "concept", "status": "deprecated"}, "## Definition\n\nvehicle alarms long ago\n")
	tv.Doc("source", "Alarm log", map[string]any{"file": "[[log.txt]]", "sha256": "abc"}, "vehicle alarms\n")
	tv.Write("sessions/2026-09/2026-09-26 0900 aaaaaa.md", "---\nid: ses-aaaaaa\ntype: session\nharness_id: aaaaaa\nstatus: ended\nupdated: 2026-09-26T09:00:00\nrepositories: [\"[[p3-edge]]\"]\n---\n")
	idx := tv.Index()

	hits, err := search.Search(idx, search.Query{Text: "p3 cloud front end", Types: []string{"repository"}})
	if err != nil || len(hits.Hits) == 0 || hits.Hits[0].Ref.Title != "p3-cloud" {
		t.Fatalf("the repository ranks first: %+v %v", hits, err)
	}
	hits, _ = search.Search(idx, search.Query{Text: "remote update", Tags: []string{"work/p3"}})
	if len(hits.Hits) != 1 || hits.Hits[0].Ref.Title != "Over the air updates" || hits.Hits[0].Snippet == "" {
		t.Fatalf("a tag keeps home out: %+v", hits.Hits)
	}
	hits, _ = search.Search(idx, search.Query{Text: "vehicle alarms", Types: []string{"topic"}, Status: []string{"draft"}})
	if len(hits.Hits) != 1 || hits.Hits[0].Ref.Title != "False alarms" {
		t.Fatalf("draft topics only: %+v", hits.Hits)
	}
	hits, _ = search.Search(idx, search.Query{Text: "vehicle alarms", Status: []string{"pending"}})
	if len(hits.Hits) != 1 || hits.Hits[0].Ref.Title != "Alarm log" {
		t.Fatalf("pending sources only: %+v", hits.Hits)
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
	if hits.Total != 2 {
		t.Fatalf("repository: the repository and the topic that holds its tag: %+v", hits)
	}
	hits, _ = search.Search(idx, search.Query{Repository: "p3-edge", Types: []string{"repository", "topic", "session"}})
	if hits.Total != 3 || hits.Facets.Types["session"] != 1 {
		t.Fatalf("repository: the repository, the session that lists it, and the topic that holds its tag: %+v", hits)
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

// The hints of the search tool name exactly what the call accepts.
func TestTheSearchHintsFollowTheSchema(t *testing.T) {
	hint := func(field string) string {
		f, _ := reflect.TypeOf(search.Query{}).FieldByName(field)
		return f.Tag.Get("jsonschema")
	}
	// byType reads "type: a, b; type: c" into each type's values.
	byType := func(text string) map[string][]string {
		out := map[string][]string{}
		for _, group := range strings.Split(text, "; ") {
			name, values, ok := strings.Cut(group, ": ")
			if !ok || schema.Get(name) == nil {
				t.Fatalf("%q names no type", group)
			}
			out[name] = strings.Split(values, ", ")
		}
		return out
	}
	same := func(what string, got, want []string) {
		a, b := slices.Clone(got), slices.Clone(want)
		slices.Sort(a)
		slices.Sort(b)
		if !slices.Equal(a, b) {
			t.Errorf("the hint names %v for %s; the schema has %v", got, what, want)
		}
	}
	// Each type's own values, under its own name, and every type that has values.
	values := func(field string) map[string][]string {
		out := map[string][]string{}
		for _, ty := range schema.Types {
			if f := ty.Field(field); f != nil && len(f.Values) > 0 {
				out[ty.Name] = f.Values
			}
		}
		return out
	}
	kinds := map[string][]string{"topic": strings.Split(strings.TrimPrefix(hint("Kinds"), "topic kinds: "), ", ")}
	statuses := byType(strings.TrimPrefix(hint("Status"), "the statuses to keep, by type: "))
	for want, got := range map[string]map[string][]string{"kind": kinds, "status": statuses} {
		have := values(want)
		for name, vs := range have {
			same("the "+want+" values of "+name, got[name], vs)
		}
		for name := range got {
			if have[name] == nil {
				t.Errorf("the %s hint names %s, which has no %s", want, name, want)
			}
		}
	}
	// The document types, then the other types the call takes.
	types, rest, _ := strings.Cut(strings.TrimPrefix(hint("Types"), "document types: "), "; or ")
	same("the document types", strings.Split(types, ", "), schema.DocumentTypes)
	extra, tail, _ := strings.Cut(rest, ". ")
	var others []string
	for _, ty := range schema.Types {
		if !schema.IsDocument(ty.Name) && ty.Name != "vault" {
			others = append(others, ty.Name)
		}
	}
	same("the other types", strings.Split(extra, ", "), others)
	if tail != "Empty: the three document types" {
		t.Errorf("the types hint ends %q", tail)
	}
}

// The facets list only statuses the filter takes.
func TestTheFacetsListOnlyStatusesTheFilterTakes(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("topic", "Fix alarms", map[string]any{"kind": "concept", "status": "draft"}, "## Definition\n\nx\n")
	tv.Doc("topic", "Score boxes", map[string]any{"kind": "concept", "status": "0/1"}, "## Definition\n\nx\n")
	hits, err := search.Search(tv.Index(), search.Query{})
	if err != nil || hits.Facets.Status["draft"] != 1 || len(hits.Facets.Status) != 1 {
		t.Fatalf("status facets: %+v %v", hits.Facets.Status, err)
	}
}
