package brief_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/brief"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
)

func TestBriefOfARepositoryAndOfTags(t *testing.T) {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", map[string]string{"CLAUDE.md": "Use gofmt.\n"})
	tv.Doc("repository", "p3-edge", map[string]any{"path": repo, "defines": "work/p3/p3-edge", "tags": []string{"work/p3", "go"}}, "")
	tv.Doc("repository", "p3-cloud", map[string]any{"path": "/nowhere", "defines": "work/p3/p3-cloud", "tags": []string{"work/p3", "ts"}}, "")
	tv.Doc("topic", "Work", map[string]any{"kind": "overview", "defines": "work"}, "## Context\n\nAll work.\n")
	tv.Doc("topic", "P3", map[string]any{"kind": "overview", "defines": "work/p3", "tags": []string{"work"}}, "## Context\n\nThe p3 product.\n")
	tv.Doc("topic", "Everywhere", map[string]any{"kind": "policy", "strength": "should"}, "")
	tv.Doc("topic", "Pin Go deps", map[string]any{"kind": "policy", "strength": "must", "tags": []string{"work/p3", "go"}}, "")
	tv.Doc("topic", "P3 rule", map[string]any{"kind": "policy", "strength": "must", "tags": []string{"work/p3"}}, "")
	tv.Doc("topic", "Edge rule", map[string]any{"kind": "policy", "strength": "should", "tags": []string{"work/p3/p3-edge"}}, "")
	tv.Doc("topic", "TS rule", map[string]any{"kind": "policy", "tags": []string{"ts"}}, "")
	tv.Doc("spec", "Score boxes", map[string]any{"kind": "plan", "repositories": []string{"[[p3-edge]]"}}, "")
	tv.Doc("stub", "Idea for edge", map[string]any{"tags": []string{"work/p3/p3-edge"}}, "")
	tv.Doc("stub", "Idea for cloud", map[string]any{"tags": []string{"work/p3/p3-cloud"}}, "")
	os.WriteFile(filepath.Join(repo, "new.txt"), []byte("x"), 0o644)
	idx := tv.Index()
	b, err := brief.Of(idx, brief.Input{Path: filepath.Join(repo, "src")})
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Pages) != 2 || b.Pages[0].Tag != "work" || b.Pages[1].Body != "The p3 product." {
		t.Fatalf("pages top down: %+v", b.Pages)
	}
	var pol []string
	for _, p := range b.Policies {
		pol = append(pol, p.Ref.Title)
	}
	if len(pol) != 4 || pol[0] != "Pin Go deps" || pol[1] != "Edge rule" || pol[2] != "P3 rule" || pol[3] != "Everywhere" {
		t.Fatalf("policies %v", pol)
	}
	if len(b.Work) != 2 || len(b.Instructions) != 1 || b.Repository == nil || !b.Repository.Exists || len(b.Repository.Dirty) != 1 || len(b.Repository.Recent) != 1 {
		t.Fatalf("work %+v instructions %+v facts %+v", b.Work, b.Instructions, b.Repository)
	}
	b, err = brief.Of(idx, brief.Input{Tags: []string{"work/p3"}})
	if err != nil || len(b.Repositories) != 2 || len(b.Work) != 2 || b.Repository != nil {
		t.Fatalf("tags: %+v %v", b, err)
	}
	b, _ = brief.Of(idx, brief.Input{})
	if len(b.Tags) == 0 || b.Tags[0].Tag != "work" || len(b.Policies) != 1 {
		t.Fatalf("vault: %+v", b)
	}
	if _, err := brief.Of(idx, brief.Input{Path: "/elsewhere"}); err == nil {
		t.Fatal("a path no repository holds")
	}
}
