// Package scope walks the context graph: the vault, its areas, and its repositories. It
// gives an agent everything it needs to work in one scope: the chain from the vault
// down, the scopes below, the policies on the chain, the open threads, and for a
// repository its own instruction files and what git says about it now.
package scope

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Bounds of what a chain carries.
const (
	MaxInstructionLines = 400
	MaxBodyLines        = 120
)

// InstructionFiles are the agent files a repository may hold, relative to its root.
var InstructionFiles = []string{"AGENTS.md", "CLAUDE.md", ".claude/CLAUDE.md"}

// Node is one scope of the chain, with its body.
type Node struct {
	Ref  vault.Ref `json:"ref"`
	Body string    `json:"body,omitempty"`
}

// Policy is one policy on the chain.
type Policy struct {
	Ref      vault.Ref `json:"ref"`
	Strength string    `json:"strength,omitempty"`
	// From is the id of the scope that holds the policy; the vault's id for the vault.
	From string `json:"from"`
}

// Instruction is one agent file of a repository.
type Instruction struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated,omitempty"`
}

// Facts are what git says about a repository now.
type Facts struct {
	Path      string `json:"path"`
	Exists    bool   `json:"exists"`
	Remote    string `json:"remote,omitempty"`
	Branch    string `json:"branch,omitempty"`
	Head      string `json:"head,omitempty"`
	Dirty     int    `json:"dirty"`
	Described string `json:"described,omitempty"`
	// Behind counts the commits from described to head; -1 when the page was never
	// described or git cannot count.
	Behind int `json:"behind"`
}

// Chain is the output of context.
type Chain struct {
	Scope        vault.Ref     `json:"scope"`
	Chain        []Node        `json:"chain"`
	Children     []vault.Ref   `json:"children"`
	Repositories []vault.Ref   `json:"repositories"`
	Policies     []Policy      `json:"policies"`
	Threads      []vault.Ref   `json:"threads"`
	Instructions []Instruction `json:"instructions,omitempty"`
	Repository   *Facts        `json:"repository,omitempty"`
}

// Context walks to a scope: an area or repository by id or title, a path inside a linked
// repository, or the vault when both are empty.
func Context(idx *vault.Index, key, path string) (*Chain, error) {
	var target *doc.Doc
	switch {
	case strings.TrimSpace(path) != "":
		d, err := RepositoryAt(idx, path)
		if err != nil {
			return nil, err
		}
		target = d
	case strings.TrimSpace(key) != "":
		d, err := idx.Resolve(key)
		if err != nil {
			return nil, err
		}
		if d.Type() != "area" && d.Type() != "repository" && d.Type() != "vault" {
			return nil, fmt.Errorf("%s is a %s, not a scope; a scope is the vault, an area, or a repository", vault.Title(d), d.Type())
		}
		if d.Type() != "vault" {
			target = d
		}
	}
	atlas := idx.ByPath(vault.Marker)
	c := &Chain{Children: []vault.Ref{}, Repositories: []vault.Ref{}, Policies: []Policy{}, Threads: []vault.Ref{}}
	// The chain, from the vault down to the scope.
	var chain []*doc.Doc
	if target != nil {
		anc := idx.Ancestors(target)
		for i := len(anc) - 1; i >= 0; i-- {
			chain = append(chain, anc[i])
		}
		chain = append(chain, target)
	}
	c.Chain = append(c.Chain, Node{Ref: idx.Ref(atlas), Body: bounded(atlas.Body, MaxBodyLines)})
	for _, d := range chain {
		c.Chain = append(c.Chain, Node{Ref: idx.Ref(d), Body: bounded(doc.StripLead(d.Body), MaxBodyLines)})
	}
	c.Scope = c.Chain[len(c.Chain)-1].Ref
	scopeID := ""
	if target != nil {
		scopeID = target.ID()
	}
	// Children and the repositories below.
	for _, d := range idx.Of("area", "repository") {
		parent := idx.Parent(d)
		if (target == nil && parent == nil) || (target != nil && parent != nil && parent.ID() == target.ID()) {
			c.Children = append(c.Children, idx.Ref(d))
		}
		if d.Type() == "repository" && idx.Under(d, scopeID) {
			c.Repositories = append(c.Repositories, idx.Ref(d))
		}
	}
	// Policies on the chain, nearest first.
	onChain := map[string]int{"": 0}
	for i, d := range chain {
		onChain[d.ID()] = i + 1
	}
	type ranked struct {
		p    Policy
		rank int
	}
	var policies []ranked
	for _, p := range idx.Of("policy") {
		ids := idx.ScopeIDs(p)
		from := ""
		if len(ids) > 0 {
			from = ids[0]
		} else if strings.TrimSpace(p.Str("scope")) != "" {
			continue // a scope that names nothing belongs to no chain
		}
		rank, ok := onChain[from]
		if !ok {
			continue
		}
		if from == "" {
			from = atlas.ID()
		}
		policies = append(policies, ranked{Policy{Ref: idx.Ref(p), Strength: p.Str("strength"), From: from}, rank})
	}
	sort.SliceStable(policies, func(i, j int) bool {
		if policies[i].rank != policies[j].rank {
			return policies[i].rank > policies[j].rank
		}
		return strengthRank(policies[i].p.Strength) < strengthRank(policies[j].p.Strength)
	})
	for _, p := range policies {
		c.Policies = append(c.Policies, p.p)
	}
	// Open threads whose scope is on the chain or below the scope.
	for _, s := range idx.Of("stub") {
		if s.Str("stage") == "closed" {
			continue
		}
		if threadMatches(idx, s, onChain, scopeID) {
			c.Threads = append(c.Threads, idx.Ref(s))
		}
	}
	if target != nil && target.Type() == "repository" {
		root := vault.Expand(target.Str("path"))
		c.Instructions = Instructions(root)
		c.Repository = RepoFacts(root, target.Str("described"))
	}
	return c, nil
}

func strengthRank(s string) int {
	switch s {
	case "must":
		return 0
	case "should":
		return 1
	}
	return 2
}

// threadMatches reports whether a stub's scope lies on the chain or below the scope. A
// stub with no scope belongs to the vault, which is on every chain.
func threadMatches(idx *vault.Index, s *doc.Doc, onChain map[string]int, scopeID string) bool {
	ids := idx.ScopeIDs(s)
	if len(ids) == 0 {
		return true
	}
	for _, id := range ids {
		if _, ok := onChain[id]; ok {
			return true
		}
		if d := idx.ByID(id); d != nil && idx.Under(d, scopeID) {
			return true
		}
	}
	return false
}

// RepositoryAt is the repository page whose path holds path: the deepest one.
func RepositoryAt(idx *vault.Index, path string) (*doc.Doc, error) {
	abs := vault.Expand(path)
	if !filepath.IsAbs(abs) {
		if a, err := filepath.Abs(abs); err == nil {
			abs = a
		}
	}
	var best *doc.Doc
	bestLen := -1
	for _, r := range idx.Of("repository") {
		p := vault.Expand(r.Str("path"))
		if p == "" || !vault.Within(abs, p) {
			continue
		}
		if len(p) > bestLen {
			best, bestLen = r, len(p)
		}
	}
	if best == nil {
		return nil, fmt.Errorf("no repository page holds %s; repo-link links it", path)
	}
	return best, nil
}

// Instructions reads a repository's agent files, each bounded.
func Instructions(root string) []Instruction {
	var out []Instruction
	for _, rel := range InstructionFiles {
		file := filepath.Join(root, rel)
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		in := Instruction{Path: vault.Shorten(file), Content: strings.Join(lines, "\n")}
		if len(lines) > MaxInstructionLines {
			in.Content = strings.Join(lines[:MaxInstructionLines], "\n")
			in.Truncated = true
		}
		out = append(out, in)
	}
	return out
}

// RepoFacts reads a repository's facts from git.
func RepoFacts(root, described string) *Facts {
	f := &Facts{Path: vault.Shorten(root), Described: described, Behind: -1}
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return f
	}
	f.Exists = true
	g := gitx.Repo{Dir: root}
	f.Remote = g.Remote()
	f.Branch = g.Branch()
	if head, err := g.Head(); err == nil {
		f.Head = head[:min(7, len(head))]
	}
	f.Dirty = g.Dirty()
	if described != "" {
		if n, err := g.Behind(described); err == nil {
			f.Behind = n
		}
	}
	return f
}

func bounded(text string, n int) string {
	text = strings.TrimSpace(text)
	lines := strings.Split(text, "\n")
	if len(lines) <= n {
		return text
	}
	return strings.Join(lines[:n], "\n") + "\n[…]"
}
