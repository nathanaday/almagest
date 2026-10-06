// Package brief gives an agent everything it needs to work in a repository or under a set
// of tags: the pages of the tags from the top down, the repositories that hold them, the
// policies that apply, and for a repository its own instruction files and what git says
// about it now. (The spec calls this the context tool; Go's own context
// package takes that name.)
package brief

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/tags"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Bounds of what a brief carries.
const (
	MaxInstructionLines = 400
	MaxBodyLines        = 120
	MaxDirty            = 50
)

// InstructionFiles are the agent files a repository may hold, relative to its root.
var InstructionFiles = []string{"AGENTS.md", "CLAUDE.md", ".claude/CLAUDE.md"}

// Page is a tag page, with its body.
type Page struct {
	Tag  string    `json:"tag"`
	Ref  vault.Ref `json:"ref"`
	Body string    `json:"body,omitempty"`
}

// TagCount is a tag and the count of documents that hold it.
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
	Page  string `json:"page,omitempty"`
}

// Policy is one policy that applies.
type Policy struct {
	Ref      vault.Ref `json:"ref"`
	Strength string    `json:"strength,omitempty"`
	// Via are the policy's tags: what makes it apply.
	Via []string `json:"via"`
}

// Instruction is one agent file of a repository.
type Instruction struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated,omitempty"`
}

// Commit is one commit of a repository.
type Commit struct {
	Commit  string `json:"commit"`
	Time    string `json:"time"`
	Subject string `json:"subject"`
}

// Facts are what git says about a repository now.
type Facts struct {
	Path   string   `json:"path"`
	Exists bool     `json:"exists"`
	Remote string   `json:"remote,omitempty"`
	Branch string   `json:"branch,omitempty"`
	Head   string   `json:"head,omitempty"`
	Dirty  []string `json:"dirty"`
	// Ahead and BehindRemote count the commits against the upstream, as of the last fetch.
	Ahead        int      `json:"ahead"`
	BehindRemote int      `json:"behind_remote"`
	Upstream     bool     `json:"upstream"`
	Recent       []Commit `json:"recent"`
	Described    string   `json:"described,omitempty"`
	// Behind counts the commits from described to head; -1 when the repository was never
	// described or git cannot count.
	Behind int `json:"behind"`
}

// Brief is the output of the context tool.
type Brief struct {
	Vault        Page          `json:"vault"`
	Tags         []TagCount    `json:"tags"`
	Pages        []Page        `json:"pages"`
	Repositories []vault.Ref   `json:"repositories"`
	Policies     []Policy      `json:"policies"`
	Instructions []Instruction `json:"instructions,omitempty"`
	Repository   *Facts        `json:"repository,omitempty"`
}

// Input is what the brief is for: a repository, tags, or a path inside a linked
// repository; the vault when all are empty.
type Input struct {
	Repository string   `json:"repository,omitempty" jsonschema:"a repository document, by id or title"`
	Tags       []string `json:"tags,omitempty" jsonschema:"tags; the brief covers the documents that hold every one"`
	Path       string   `json:"path,omitempty" jsonschema:"a path inside a linked repository"`
}

// Of builds the brief.
func Of(idx *vault.Index, in Input) (*Brief, error) {
	var repo *doc.Doc
	switch {
	case strings.TrimSpace(in.Path) != "":
		d, err := RepositoryAt(idx, in.Path)
		if err != nil {
			return nil, err
		}
		repo = d
	case strings.TrimSpace(in.Repository) != "":
		d, err := idx.ResolveType(in.Repository, "repository")
		if err != nil {
			return nil, err
		}
		repo = d
	}
	want, err := tags.NormalizeAll(in.Tags)
	if err != nil {
		return nil, err
	}
	atlas := idx.ByPath(vault.Marker)
	b := &Brief{Tags: []TagCount{}, Pages: []Page{}, Repositories: []vault.Ref{}, Policies: []Policy{}}
	if atlas != nil {
		b.Vault = Page{Ref: idx.Ref(atlas), Body: bounded(doc.StripLead(atlas.Body), MaxBodyLines)}
	}
	// held are the tags the brief stands under: a repository's own and the one it defines,
	// or the tags asked for.
	var held []string
	switch {
	case repo != nil:
		held = vault.DocTags(repo)
	case len(want) > 0:
		held = want
	}
	counts := idx.TagCounts()
	add := func(t string) {
		tc := TagCount{Tag: t, Count: counts[t]}
		if p := idx.TagPage(t); p != nil {
			tc.Page = p.Title()
		}
		b.Tags = append(b.Tags, tc)
	}
	if len(held) == 0 {
		for _, t := range idx.TopTags() {
			add(t)
		}
	} else {
		for _, t := range held {
			add(t)
		}
	}
	// The pages of the tags and every tag above them, from the top down.
	var pageTags []string
	if len(held) == 0 {
		pageTags = idx.TopTags()
	} else {
		pageTags = tags.Expand(held)
		sort.SliceStable(pageTags, func(i, j int) bool { return tags.Depth(pageTags[i]) < tags.Depth(pageTags[j]) })
	}
	for _, t := range pageTags {
		if p := idx.TagPage(t); p != nil && (repo == nil || p.ID() != repo.ID()) {
			body, _ := doc.Section(p.Body, "Context")
			if body == "" {
				body = doc.StripLead(p.Body)
			}
			b.Pages = append(b.Pages, Page{Tag: t, Ref: idx.Ref(p), Body: bounded(body, MaxBodyLines)})
		}
	}
	// The repositories.
	for _, r := range idx.Of("repository") {
		if r.Front.Bool("unlinked") {
			continue
		}
		switch {
		case repo != nil:
			if r.ID() == repo.ID() {
				b.Repositories = append(b.Repositories, idx.Ref(r))
			}
		case len(want) == 0 || vault.Holds(r, want...):
			b.Repositories = append(b.Repositories, idx.Ref(r))
		}
	}
	b.Policies = Policies(idx, held)
	if repo != nil && !repo.Front.Bool("unlinked") && repo.Str("path") != "" {
		root := vault.Expand(repo.Str("path"))
		b.Instructions = Instructions(root)
		b.Repository = RepoFacts(root, repo.Str("described"))
	}
	return b, nil
}

// Policies are the policies that apply under a set of tags: a policy applies when the tags
// hold every tag the policy holds. A policy with no tags applies everywhere. The most
// specific come first: the most tags, then the deepest, then must before should.
func Policies(idx *vault.Index, held []string) []Policy {
	type ranked struct {
		p     Policy
		n     int
		depth int
	}
	var list []ranked
	for _, p := range idx.Of("topic") {
		if p.Str("kind") != "policy" || p.Str("status") == "deprecated" {
			continue
		}
		pt := p.List("tags")
		if !tags.HoldsAll(held, pt) {
			continue
		}
		depth := 0
		for _, t := range pt {
			depth = max(depth, tags.Depth(t))
		}
		list = append(list, ranked{Policy{Ref: idx.Ref(p), Strength: p.Str("strength"), Via: doc.NonNil(pt)}, len(pt), depth})
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].n != list[j].n {
			return list[i].n > list[j].n
		}
		if list[i].depth != list[j].depth {
			return list[i].depth > list[j].depth
		}
		return strengthRank(list[i].p.Strength) < strengthRank(list[j].p.Strength)
	})
	out := make([]Policy, 0, len(list))
	for _, r := range list {
		out = append(out, r.p)
	}
	return out
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

// RepositoryAt is the linked repository whose path holds path: the deepest one.
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
		if p == "" || r.Front.Bool("unlinked") || !vault.Within(abs, p) {
			continue
		}
		if len(p) > bestLen {
			best, bestLen = r, len(p)
		}
	}
	if best == nil {
		return nil, fmt.Errorf("no repository document holds %s; repo-link links it", path)
	}
	return best, nil
}

// Instructions reads a repository's agent files, each bounded. It reads through a root,
// so a file that links out of the repository is skipped.
func Instructions(root string) []Instruction {
	var out []Instruction
	r, err := os.OpenRoot(root)
	if err != nil {
		return out
	}
	defer r.Close()
	for _, rel := range InstructionFiles {
		file := filepath.Join(root, rel)
		f, err := r.Open(rel)
		if err != nil {
			continue
		}
		data, err := io.ReadAll(f)
		f.Close()
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

// RepoFacts reads a repository's facts from git, now.
func RepoFacts(root, described string) *Facts {
	f := &Facts{Path: vault.Shorten(root), Described: described, Behind: -1, Dirty: []string{}, Recent: []Commit{}}
	if st, err := os.Stat(root); err != nil || !st.IsDir() || !gitx.IsRoot(root) {
		return f
	}
	f.Exists = true
	g := gitx.Repo{Dir: root}
	f.Remote = g.Remote()
	f.Branch = g.Branch()
	if head, err := g.Head(); err == nil {
		f.Head = head[:min(7, len(head))]
	}
	if entries, err := g.Status(); err == nil {
		for i, e := range entries {
			if i == MaxDirty {
				f.Dirty = append(f.Dirty, fmt.Sprintf("… and %d more", len(entries)-MaxDirty))
				break
			}
			f.Dirty = append(f.Dirty, e.Path)
		}
	}
	f.Ahead, f.BehindRemote, f.Upstream = g.AheadBehind()
	if commits, err := g.Log(3); err == nil {
		for _, c := range commits {
			f.Recent = append(f.Recent, Commit{Commit: c.SHA[:min(7, len(c.SHA))], Time: vault.Stamp(c.Date.Local()), Subject: c.Subject})
		}
	}
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
