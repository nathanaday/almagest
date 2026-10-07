// Package match joins the subjects that workers extracted from a document's chunks and
// matches each one against the topics and sources: a hit when a document holds its name,
// near when one scores above the threshold, new otherwise. Code decides all three;
// whether a near document is the same subject is the drafter's judgment.
package match

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/search"
	"github.com/nathanaday/almagest/internal/tags"
	"github.com/nathanaday/almagest/internal/vault"
)

// Threshold is the normalized score above which a page is near a subject.
const Threshold = 0.35

// Neighbor counts.
const (
	NearNeighbors = 5
	NewNeighbors  = 3
)

// Claim is one statement of a chunk, with where it stands.
type Claim struct {
	Text    string `json:"text"`
	Locator string `json:"locator"`
}

// Item is a subject one chunk says something about.
type Item struct {
	Kind        string   `json:"kind" jsonschema:"concept, entity, or policy"`
	Name        string   `json:"name"`
	Aliases     []string `json:"aliases,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty" jsonschema:"tags that exist and fit; the drafter decides"`
	Strength    string   `json:"strength,omitempty"`
	Claims      []Claim  `json:"claims,omitempty"`
	Source      string   `json:"source,omitempty"`
}

// ItemMap is what one chunk says, as a worker returns it.
type ItemMap struct {
	Doc       string   `json:"doc"`
	Chunk     int      `json:"chunk"`
	Summary   string   `json:"summary,omitempty"`
	Items     []Item   `json:"items"`
	Questions []string `json:"questions,omitempty"`
	Partial   bool     `json:"partial,omitempty"`
	Reason    string   `json:"reason,omitempty"`
}

// Neighbor is a page close to a subject, with its score.
type Neighbor struct {
	Ref   vault.Ref `json:"ref"`
	Score float64   `json:"score"`
}

// Subject is one subject joined across chunks, and its match.
type Subject struct {
	Key   string `json:"key"`
	Kind  string `json:"kind"`
	Items []Item `json:"items"`
	// Of is the topic a map compares, when the input was documents.
	Of        *vault.Ref `json:"of,omitempty"`
	Match     string     `json:"match"`
	Page      *vault.Ref `json:"page,omitempty"`
	Neighbors []Neighbor `json:"neighbors"`
}

// Map is the output of match.
type Map struct {
	Subjects []Subject `json:"subjects"`
}

// Input selects what to match.
type Input struct {
	Items  []ItemMap `json:"items,omitempty" jsonschema:"Item Maps from wiki-extract: new subjects to match against the topics"`
	Docs   []string  `json:"docs,omitempty" jsonschema:"topics (ids or titles) to match against the other topics"`
	Tags   []string  `json:"tags,omitempty" jsonschema:"limits the candidates to documents that hold every one of these tags"`
	Across bool      `json:"across,omitempty" jsonschema:"with one tag: compare each topic under it only with topics under a different child tag of it"`
}

// kindOf is how a subject compares with a document: a topic by its kind, a source as a
// source.
func kindOf(d *doc.Doc) string {
	if d.Type() == "topic" {
		return d.Str("kind")
	}
	return d.Type()
}

// Run matches.
func Run(idx *vault.Index, in Input) (*Map, error) {
	want, err := tags.NormalizeAll(in.Tags)
	if err != nil {
		return nil, err
	}
	var pages []*doc.Doc
	for _, d := range idx.Of("topic", "source") {
		if len(want) == 0 || vault.Holds(d, want...) {
			pages = append(pages, d)
		}
	}
	if in.Across && len(want) != 1 {
		return nil, errors.New("across compares the parts of one tag; give exactly one tag")
	}
	switch {
	case len(in.Items) > 0 && (len(in.Docs) > 0 || in.Across):
		return nil, errors.New("match takes items, or documents to compare, not both")
	case len(in.Items) > 0:
		return matchItems(idx, in.Items, pages)
	case len(in.Docs) > 0 || in.Across:
		top := ""
		if in.Across {
			top = want[0]
		}
		ids := in.Docs
		if len(ids) == 0 {
			for _, p := range pages {
				if p.Type() == "topic" {
					ids = append(ids, p.ID())
				}
			}
		}
		return matchDocs(idx, ids, pages, top)
	}
	return nil, errors.New("match needs items (Item Maps), or documents or a tag with across to compare")
}

// Normalize folds a name for comparison: lower case, trimmed, and every run of space
// and punctuation made one space.
func Normalize(name string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(r)
			continue
		}
		space = true
	}
	return b.String()
}

// singulars folds a trailing plural s when the singular also occurs among the names.
func singulars(names map[string]bool) map[string]string {
	out := map[string]string{}
	for n := range names {
		out[n] = n
		if len(n) > 3 && strings.HasSuffix(n, "s") && !strings.HasSuffix(n, "ss") {
			if names[n[:len(n)-1]] {
				out[n] = n[:len(n)-1]
			}
		}
	}
	return out
}

func pageNames(d *doc.Doc) []string {
	return append([]string{vault.Title(d)}, d.List("aliases")...)
}

func matchItems(idx *vault.Index, maps []ItemMap, pages []*doc.Doc) (*Map, error) {
	var items []Item
	for i, m := range maps {
		for j, it := range m.Items {
			if strings.TrimSpace(it.Kind) == "" || strings.TrimSpace(it.Name) == "" {
				return nil, fmt.Errorf("Item Map %d (doc %s, chunk %d): item %d has no kind or no name", i+1, m.Doc, m.Chunk, j+1)
			}
			if it.Source == "" {
				it.Source = m.Doc
			}
			items = append(items, it)
		}
	}
	// Every name that takes part, so a plural folds only when its singular occurs.
	all := map[string]bool{}
	for _, it := range items {
		for _, n := range append([]string{it.Name}, it.Aliases...) {
			all[Normalize(n)] = true
		}
	}
	for _, p := range pages {
		for _, n := range pageNames(p) {
			all[Normalize(n)] = true
		}
	}
	fold := singulars(all)
	key := func(n string) string { return fold[Normalize(n)] }

	// Join: two items are one subject when a name or alias is equal and the type is too.
	parent := make([]int, len(items))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	seen := map[string]int{}
	for i, it := range items {
		for _, n := range append([]string{it.Name}, it.Aliases...) {
			k := strings.ToLower(it.Kind) + "\x00" + key(n)
			if j, ok := seen[k]; ok {
				parent[find(i)] = find(j)
			} else {
				seen[k] = i
			}
		}
	}
	groups := map[int][]Item{}
	var order []int
	for i, it := range items {
		r := find(i)
		if _, ok := groups[r]; !ok {
			order = append(order, r)
		}
		groups[r] = append(groups[r], it)
	}
	byKey := map[string][]*doc.Doc{}
	for _, p := range pages {
		for _, n := range pageNames(p) {
			k := key(n)
			byKey[k] = append(byKey[k], p)
		}
	}
	corpus := corpusOf(pages)
	out := &Map{Subjects: []Subject{}}
	for _, r := range order {
		g := groups[r]
		s := Subject{Key: key(g[0].Name), Kind: strings.ToLower(g[0].Kind), Items: g, Neighbors: []Neighbor{}}
		var names []string
		var desc []string
		for _, it := range g {
			names = append(names, it.Name)
			names = append(names, it.Aliases...)
			if it.Description != "" {
				desc = append(desc, it.Description)
			}
		}
		var hit *doc.Doc
		for _, n := range names {
			for _, p := range byKey[key(n)] {
				if hit == nil || (kindOf(p) == s.Kind && kindOf(hit) != s.Kind) {
					hit = p
				}
			}
		}
		ranked := corpus.rank(strings.Join(names, " ")+" "+strings.Join(desc, " "), nil)
		if hit != nil {
			ref := idx.Ref(hit)
			s.Match, s.Page = "hit", &ref
			for _, n := range ranked {
				if n.doc.ID() != hit.ID() && n.score >= Threshold && len(s.Neighbors) < NearNeighbors {
					s.Neighbors = append(s.Neighbors, Neighbor{Ref: idx.Ref(n.doc), Score: n.score})
				}
			}
		} else {
			s.Match = "new"
			for _, n := range ranked {
				if n.score >= Threshold {
					s.Match = "near"
				}
			}
			limit := NearNeighbors
			if s.Match == "new" {
				limit = NewNeighbors
			}
			for _, n := range ranked {
				if len(s.Neighbors) == limit || n.score <= 0 {
					break
				}
				if s.Match == "near" && n.score < Threshold {
					break
				}
				s.Neighbors = append(s.Neighbors, Neighbor{Ref: idx.Ref(n.doc), Score: n.score})
			}
		}
		out.Subjects = append(out.Subjects, s)
	}
	return mergeHits(out), nil
}

// mergeHits joins the subjects of one kind that hit one document: its alias shows they
// name one thing, and one drafter must write that document.
func mergeHits(m *Map) *Map {
	var out []Subject
	at := map[string]int{}
	for _, s := range m.Subjects {
		if s.Match == "hit" && s.Page != nil {
			k := s.Kind + "\x00" + s.Page.ID
			if i, ok := at[k]; ok {
				out[i].Items = append(out[i].Items, s.Items...)
				continue
			}
			at[k] = len(out)
		}
		out = append(out, s)
	}
	m.Subjects = out
	return m
}

// matchDocs compares topics with the other candidates. With a tag, a topic is compared
// only with topics under a different child tag of it: the candidates for a bridge, or
// for widening a topic's tags to the parent.
func matchDocs(idx *vault.Index, ids []string, pages []*doc.Doc, top string) (*Map, error) {
	var inputs []*doc.Doc
	for _, id := range ids {
		d, err := idx.Resolve(id)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, d)
	}
	all := map[string]bool{}
	for _, p := range append(append([]*doc.Doc{}, pages...), inputs...) {
		for _, n := range pageNames(p) {
			all[Normalize(n)] = true
		}
	}
	fold := singulars(all)
	key := func(n string) string { return fold[Normalize(n)] }
	out := &Map{Subjects: []Subject{}}
	for _, d := range inputs {
		var others []*doc.Doc
		own := childTags(d, top)
		for _, c := range pages {
			if c.ID() == d.ID() || c.Type() != "topic" {
				continue
			}
			if top != "" {
				theirs := childTags(c, top)
				if len(own) == 0 || len(theirs) == 0 || overlap(own, theirs) {
					continue
				}
			}
			others = append(others, c)
		}
		of := idx.Ref(d)
		s := Subject{Key: key(vault.Title(d)), Kind: kindOf(d), Items: []Item{}, Of: &of, Match: "new", Neighbors: []Neighbor{}}
		names := map[string]bool{}
		for _, n := range pageNames(d) {
			names[key(n)] = true
		}
		for _, c := range others {
			for _, n := range pageNames(c) {
				if names[key(n)] && s.Page == nil {
					ref := idx.Ref(c)
					s.Match, s.Page = "hit", &ref
				}
			}
		}
		corpus := corpusOf(others)
		ranked := corpus.rank(strings.Join(pageNames(d), " ")+" "+d.Str("description"), nil)
		for _, n := range ranked {
			if s.Page != nil && n.doc.ID() == s.Page.ID {
				continue
			}
			if n.score >= Threshold {
				if s.Match == "new" {
					s.Match = "near"
				}
				if len(s.Neighbors) < NearNeighbors {
					s.Neighbors = append(s.Neighbors, Neighbor{Ref: idx.Ref(n.doc), Score: n.score})
				}
			}
		}
		out.Subjects = append(out.Subjects, s)
	}
	return out, nil
}

// childTags are the child tags of top that a document holds: for top work/p3 and a tag
// work/p3/p3-edge/ml, work/p3/p3-edge.
func childTags(d *doc.Doc, top string) []string {
	if top == "" {
		return nil
	}
	var out []string
	for _, t := range vault.DocTags(d) {
		if !tags.Under(t, top) || t == top {
			continue
		}
		child := top + "/" + strings.Split(strings.TrimPrefix(t, top+"/"), "/")[0]
		if !slicesContains(out, child) {
			out = append(out, child)
		}
	}
	return out
}

func overlap(a, b []string) bool {
	for _, x := range a {
		if slicesContains(b, x) {
			return true
		}
	}
	return false
}

func slicesContains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// corpus ranks pages by BM25 over their name, aliases, and description.
type corpus struct {
	docs   []*doc.Doc
	fields []search.Fields
}

type ranked struct {
	doc   *doc.Doc
	score float64
}

func corpusOf(pages []*doc.Doc) *corpus {
	c := &corpus{docs: pages}
	for _, p := range pages {
		c.fields = append(c.fields, search.Fields{
			Title:       search.Tokens(vault.Title(p)),
			Aliases:     search.Tokens(strings.Join(p.List("aliases"), " ")),
			Description: search.Tokens(p.Str("description")),
		})
	}
	return c
}

// rank scores every page against the text, normalized by the score a page made of the
// text itself would get, so 1 is as close as a page can be.
func (c *corpus) rank(text string, _ []string) []ranked {
	terms := search.Tokens(text)
	if len(terms) == 0 || len(c.docs) == 0 {
		return nil
	}
	self := search.Fields{Title: terms}
	withSelf := append(append([]search.Fields{}, c.fields...), self)
	scores := search.Rank(withSelf, terms)
	best := scores[len(scores)-1]
	if best <= 0 {
		return nil
	}
	out := make([]ranked, 0, len(c.docs))
	for i, d := range c.docs {
		s := math.Min(1, scores[i]/best)
		out = append(out, ranked{doc: d, score: math.Round(s*100) / 100})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
	return out
}
