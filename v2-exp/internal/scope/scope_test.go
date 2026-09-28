package scope_test

import (
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/scope"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/testvault"
)

func TestContextChain(t *testing.T) {
	tv := testvault.New(t)
	repo := tv.Repo("p3-cloud", map[string]string{"CLAUDE.md": "Use pnpm.\n", "AGENTS.md": "Run tests.\n"})
	tv.Page("area", "work", nil, "The work area.\n")
	tv.Page("area", "p3", map[string]any{"parent": "[[work]]"}, "")
	tv.Page("repository", "p3-cloud", map[string]any{"parent": "[[p3]]", "path": repo}, "## What it is\n\nThe front end.\n")
	tv.Page("repository", "tools", map[string]any{"path": "/nowhere"}, "")
	tv.Page("policy", "Sign commits", map[string]any{"strength": "should"}, "")
	tv.Page("policy", "Use pnpm", map[string]any{"scope": "[[p3-cloud]]", "strength": "must"}, "")
	tv.Page("policy", "Review in pairs", map[string]any{"scope": "[[p3]]", "strength": "must"}, "")
	tv.Page("policy", "Home rule", map[string]any{"scope": "[[tools]]"}, "")
	tv.Write("threads/A/A.md", "---\nid: thr-aaaaaa\ntype: stub\nscope: [\"[[p3-cloud]]\"]\nstage: spec\n---\n")
	tv.Write("threads/B/B.md", "---\nid: thr-bbbbbb\ntype: stub\nscope: [\"[[tools]]\"]\nstage: stub\n---\n")
	idx := tv.Index()

	c, err := scope.Context(idx, "p3-cloud", "")
	if err != nil {
		t.Fatal(err)
	}
	var chain []string
	for _, n := range c.Chain {
		chain = append(chain, n.Ref.Title)
	}
	if strings.Join(chain, " → ") != "Work → work → p3 → p3-cloud" {
		t.Fatalf("chain %v", chain)
	}
	var policies []string
	for _, p := range c.Policies {
		policies = append(policies, p.Ref.Title)
	}
	if strings.Join(policies, ", ") != "Use pnpm, Review in pairs, Sign commits" {
		t.Fatalf("policies nearest first: %v", policies)
	}
	if len(c.Threads) != 1 || c.Threads[0].Title != "A" {
		t.Fatalf("threads %v", c.Threads)
	}
	if len(c.Instructions) != 2 || c.Repository == nil || c.Repository.Branch != "main" || c.Repository.Behind != -1 {
		t.Fatalf("repository %+v %+v", c.Instructions, c.Repository)
	}

	byPath, err := scope.Context(idx, "", repo+"/src/app")
	if err != nil || byPath.Scope.Title != "p3-cloud" {
		t.Fatalf("by path: %v %v", byPath, err)
	}
	vaultChain, _ := scope.Context(idx, "", "")
	if vaultChain.Scope.Type != "vault" || len(vaultChain.Children) != 2 || len(vaultChain.Repositories) != 2 || len(vaultChain.Threads) != 2 {
		t.Fatalf("the vault: %+v", vaultChain)
	}
	if _, err := scope.Context(idx, "Sign commits", ""); err == nil {
		t.Fatal("a policy is no scope")
	}
}
