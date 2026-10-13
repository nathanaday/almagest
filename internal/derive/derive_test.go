package derive_test

import (
	"strings"
	"testing"

	"github.com/nathanaday/almagest/internal/derive"
	"github.com/nathanaday/almagest/internal/testvault"
)

func TestDerivedParts(t *testing.T) {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", nil)
	tv.Doc("repository", "p3-edge", map[string]any{"path": repo, "defines": "work/p3/p3-edge", "tags": []string{"work/p3"}}, "## What it is\n\nThe edge service.\n")
	tv.Doc("source", "DINOv2", map[string]any{"file": "[[doc-aaaaaa.pdf]]", "media": "pdf", "sha256": "abc", "measure": "31 pages", "authority": "primary", "origin": "ingest", "locator": "DINOv2.pdf", "captured": "2026-09-27T14:40:12", "authors": []string{"A", "B"}}, "## Summary\n\nx\n")
	tv.Doc("topic", "Edge", map[string]any{"kind": "overview", "defines": "work/p3/p3-edge", "tags": []string{"work/p3"}}, "## Summary\n\nx\n")
	tv.Doc("topic", "Pin deps", map[string]any{"kind": "policy", "strength": "must", "tags": []string{"work/p3", "go"}, "status": "draft"}, "## Rule\n\nPin.\n")
	tv.Doc("topic", "Scoring", map[string]any{"kind": "concept", "tags": []string{"work/p3/p3-edge/ml"}}, "")
	idx := tv.Index()
	wrote, err := derive.Sync(idx, tv.V.WriteIfChanged)
	if err != nil || len(wrote) != 5 {
		t.Fatalf("wrote %v %v", wrote, err)
	}
	checks := map[string][]string{
		"DINOv2":   {"status: pending", "> [!source] PDF · 31 pages · primary\n> A and B\n> Captured 2026-09-27 from `DINOv2.pdf` (ingest) · pending: not yet ingested", "\n![[doc-aaaaaa.pdf]]\n\n## Summary"},
		"p3-edge":  {"> [!repository] `" + repo + "`", "Not described yet (repo-ingest) · tag #work/p3/p3-edge", "```almagest-repo\n", "## Knowledge\n\n```base", `file.hasTag("work/p3/p3-edge", "work/p3/p3-edge/ml")`},
		"Edge":     {"> [!overview] The page of #work/p3/p3-edge · under #work/p3\n", "## Map\n\n```base"},
		"Pin deps": {"> [!policy] Draft · Must · holds for repositories tagged #work/p3 and #go"},
		"Scoring":  {"> [!concept] Concept\n> 0 sources · #work/p3/p3-edge/ml"},
	}
	for title, wants := range checks {
		got := tv.Read("tool/source-core/documents/" + title + ".md")
		for _, w := range wants {
			if !strings.Contains(got, w) {
				t.Errorf("%s lacks %q:\n%s", title, w, got)
			}
		}
		if strings.Contains(got, "## Threads") {
			t.Errorf("%s has a Threads section:\n%s", title, got)
		}
	}
	if again, _ := derive.Sync(tv.Index(), tv.V.WriteIfChanged); len(again) != 0 {
		t.Fatalf("a second sync writes nothing: %v", again)
	}
	facts, err := derive.GitFacts(tv.Index(), tv.V.WriteIfChanged, testvault.Now)
	if err != nil || len(facts) != 1 {
		t.Fatalf("facts %v %v", facts, err)
	}
	got := tv.Read("tool/source-core/documents/p3-edge.md")
	if !strings.Contains(got, "branch: ") || !strings.Contains(got, "head: ") || !strings.Contains(got, "refreshed: 2026-09-27T14:32:00") {
		t.Fatalf("facts:\n%s", got)
	}
	if again, _ := derive.GitFacts(tv.Index(), tv.V.WriteIfChanged, testvault.Now); len(again) != 0 {
		t.Fatal("a quiet repository gets no write")
	}
}

func TestLeadShowsUp(t *testing.T) {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", nil)
	tv.Doc("topic", "Work", map[string]any{"kind": "overview", "defines": "work", "aliases": []string{"WK"}}, "## Summary\n\nx\n")
	tv.Doc("topic", "P3", map[string]any{"kind": "overview", "defines": "work/p3", "tags": []string{"work"}, "up": []string{"[[Work]]", "[[Edge AI]]"}}, "## Summary\n\nx\n")
	tv.Doc("topic", "Edge AI", map[string]any{"kind": "concept", "tags": []string{"work"}}, "")
	tv.Doc("repository", "p3-edge", map[string]any{"path": repo, "defines": "work/p3/p3-edge", "tags": []string{"work/p3"}, "up": []string{"[[P3]]"}}, "## What it is\n\nThe edge service.\n")
	tv.Doc("topic", "Scoring", map[string]any{"kind": "concept", "tags": []string{"work/p3"}, "up": []string{"[[Nothing]]"}}, "")
	tv.Doc("topic", "AL", map[string]any{"kind": "overview", "defines": "work/al", "tags": []string{"work"}, "aliases": []string{"Active learning"}, "up": []string{"[[WK]]"}}, "## Summary\n\nx\n")
	tv.Doc("topic", "Dead", map[string]any{"kind": "overview", "defines": "work/dead", "tags": []string{"work"}, "up": []string{"[[Nothing]]"}}, "## Summary\n\nx\n")
	if _, err := derive.Sync(tv.Index(), tv.V.WriteIfChanged); err != nil {
		t.Fatal(err)
	}
	checks := map[string]struct{ has, lacks string }{
		"P3":      {"> [!overview] The page of #work/p3\n> 0 sources · #work\n> Up: [[Work]] · [[Edge AI]]\n", "under"},
		"p3-edge": {"\n> Up: [[P3]]\n", ""},
		"Edge AI": {"> [!concept] Concept\n> 0 sources · #work", "Up:"},
		"Scoring": {"> [!concept] Concept\n> 0 sources · #work/p3", "Up:"},
		"AL":      {"> [!overview] The page of #work/al\n", "under"},
		"Dead":    {"> [!overview] The page of #work/dead\n", "under"},
	}
	for title, c := range checks {
		got := tv.Read("tool/source-core/documents/" + title + ".md")
		if !strings.Contains(got, c.has) || c.lacks != "" && strings.Contains(got, c.lacks) {
			t.Errorf("%s: want %q and no %q:\n%s", title, c.has, c.lacks, got)
		}
	}
	if wrote, _ := derive.Sync(tv.Index(), tv.V.WriteIfChanged); len(wrote) != 0 {
		t.Errorf("a second sync wrote %v", wrote)
	}
}
