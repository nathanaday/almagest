// Package search ranks the documents of a vault against a query with BM25 over five
// fields: the title, the aliases, the tags, the description, and the body. It filters on
// the fields every document has: type, kind, tags, and status. It reads the vault on each
// call, so no index file exists to go stale.
package search

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/tags"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Weights of the fields.
const (
	WeightTitle       = 3.0
	WeightAliases     = 3.0
	WeightTags        = 2.0
	WeightDescription = 2.0
	WeightBody        = 1.0
	k1                = 1.2
	b                 = 0.75
	DefaultLimit      = 20
	// MaxFacetTags bounds the tags a facet lists.
	MaxFacetTags = 30
)

// Query is what to search for.
type Query struct {
	Text       string   `json:"text,omitempty" jsonschema:"free text; may be empty when the filters say enough"`
	Types      []string `json:"types,omitempty" jsonschema:"source, repository, topic, stub, spec, event (and session or change); empty: the six document types"`
	Kinds      []string `json:"kinds,omitempty" jsonschema:"topic kinds (concept, entity, policy, overview), spec kinds (plan, design), or event kinds"`
	Tags       []string `json:"tags,omitempty" jsonschema:"a document must hold every one of these tags, or a tag below it"`
	Status     []string `json:"status,omitempty" jsonschema:"the statuses to keep: open, started, done, dropped, resolved, current, superseded, pending, absorbed, draft, stable, contested, deprecated"`
	Repository string   `json:"repository,omitempty" jsonschema:"only documents that name this repository or hold its tag, by id or title"`
	Limit      int      `json:"limit,omitempty" jsonschema:"at most this many hits; 20 when 0"`
}

// Hit is one ranked document.
type Hit struct {
	Ref     vault.Ref `json:"ref"`
	Score   float64   `json:"score,omitempty"`
	Snippet string    `json:"snippet,omitempty"`
}

// Facets count the tags, types, and statuses among every match, before the limit.
type Facets struct {
	Tags   map[string]int `json:"tags"`
	Types  map[string]int `json:"types"`
	Status map[string]int `json:"status"`
}

// Hits is the output of a search.
type Hits struct {
	Hits   []Hit  `json:"hits"`
	Total  int    `json:"total"`
	Facets Facets `json:"facets"`
}

// Fields are a document's text, as search ranks it.
type Fields struct {
	Title       []string
	Aliases     []string
	Tags        []string
	Description []string
	Body        []string
	Lines       []string
}

// FieldsOf reads the five fields of a document. A change's Writes, which hold copies of
// documents, and a lead callout are never ranked.
func FieldsOf(d *doc.Doc) Fields {
	body := doc.StripLead(vault.Readable(d))
	return Fields{
		Title:       Tokens(vault.Title(d)),
		Aliases:     Tokens(strings.Join(d.List("aliases"), " ")),
		Tags:        Tokens(strings.Join(vault.DocTags(d), " ")),
		Description: Tokens(d.Str("description")),
		Body:        Tokens(body),
		Lines:       strings.Split(body, "\n"),
	}
}

// Search runs a query.
func Search(idx *vault.Index, q Query) (*Hits, error) {
	for _, t := range q.Types {
		if !schema.Is(t) || t == "vault" {
			return nil, fmt.Errorf("type %q is not a document type; the types are %s, and session or change", t, strings.Join(schema.DocumentTypes, ", "))
		}
	}
	for _, k := range q.Kinds {
		if !slices.Contains(schema.AllKinds(), k) {
			return nil, fmt.Errorf("kind %q is no kind; the kinds are %s", k, strings.Join(schema.AllKinds(), ", "))
		}
	}
	for _, s := range q.Status {
		if !slices.Contains(schema.Statuses(), s) {
			return nil, fmt.Errorf("status %q is no status; the statuses are %s", s, strings.Join(schema.Statuses(), ", "))
		}
	}
	want, err := tags.NormalizeAll(q.Tags)
	if err != nil {
		return nil, err
	}
	var repo *doc.Doc
	if strings.TrimSpace(q.Repository) != "" {
		if repo, err = idx.ResolveType(q.Repository, "repository"); err != nil {
			return nil, fmt.Errorf("repository: %w", err)
		}
	}
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	var kept []*doc.Doc
	var refs []vault.Ref
	for _, d := range idx.Docs {
		if !wanted(d, q.Types) || (len(q.Kinds) > 0 && !slices.Contains(q.Kinds, d.Str("kind"))) {
			continue
		}
		if len(want) > 0 && !vault.Holds(d, want...) {
			continue
		}
		if repo != nil && !names(idx, d, repo) {
			continue
		}
		ref := idx.Ref(d)
		if len(q.Status) > 0 && !slices.Contains(q.Status, ref.Status) {
			continue
		}
		kept = append(kept, d)
		refs = append(refs, ref)
	}
	terms := Tokens(q.Text)
	out := &Hits{Hits: []Hit{}}
	var order []int
	var scores []float64
	var fields []Fields
	if len(terms) == 0 {
		for i := range kept {
			order = append(order, i)
		}
		sort.SliceStable(order, func(i, j int) bool {
			return kept[order[i]].Str("updated") > kept[order[j]].Str("updated")
		})
	} else {
		fields = make([]Fields, len(kept))
		for i, d := range kept {
			fields[i] = FieldsOf(d)
		}
		scores = Rank(fields, terms)
		for i, s := range scores {
			if s > 0 {
				order = append(order, i)
			}
		}
		sort.SliceStable(order, func(i, j int) bool { return scores[order[i]] > scores[order[j]] })
	}
	out.Total = len(order)
	out.Facets = facets(kept, refs, order, want)
	for _, i := range order {
		if len(out.Hits) == limit {
			break
		}
		h := Hit{Ref: refs[i]}
		if scores != nil {
			h.Score = math.Round(scores[i]*100) / 100
			h.Snippet = snippet(fields[i].Lines, terms)
		}
		out.Hits = append(out.Hits, h)
	}
	return out, nil
}

// facets counts the tags beyond those asked for, the types, and the statuses of the
// matches. The tags keep the most used, and a tag above one asked for is left out.
func facets(docs []*doc.Doc, refs []vault.Ref, order []int, asked []string) Facets {
	f := Facets{Tags: map[string]int{}, Types: map[string]int{}, Status: map[string]int{}}
	skip := map[string]bool{}
	for _, t := range asked {
		skip[t] = true
		for _, a := range tags.Ancestors(t) {
			skip[a] = true
		}
	}
	all := map[string]int{}
	for _, i := range order {
		f.Types[refs[i].Type]++
		if refs[i].Status != "" {
			f.Status[refs[i].Status]++
		}
		for _, t := range tags.Expand(vault.DocTags(docs[i])) {
			if !skip[t] {
				all[t]++
			}
		}
	}
	list := make([]string, 0, len(all))
	for t := range all {
		list = append(list, t)
	}
	sort.Slice(list, func(i, j int) bool {
		if all[list[i]] != all[list[j]] {
			return all[list[i]] > all[list[j]]
		}
		return list[i] < list[j]
	})
	for i, t := range list {
		if i == MaxFacetTags {
			break
		}
		f.Tags[t] = all[t]
	}
	return f
}

func wanted(d *doc.Doc, types []string) bool {
	if len(types) == 0 {
		return schema.IsDocument(d.Type())
	}
	return slices.Contains(types, d.Type())
}

// names reports whether a document belongs to a repository: the repository itself, a
// spec that names it, an event about such a spec, or a document that holds its tag.
func names(idx *vault.Index, d, repo *doc.Doc) bool {
	if d.ID() == repo.ID() {
		return true
	}
	lists := func(x *doc.Doc) bool {
		return slices.ContainsFunc(x.List("repositories"), func(l string) bool { return strings.EqualFold(doc.LinkTarget(l), repo.Title()) })
	}
	if lists(d) {
		return true
	}
	if d.Type() == "event" {
		if s := idx.Linked(d.Str("subject")); s != nil && lists(s) {
			return true
		}
	}
	if def := repo.Str("defines"); def != "" && vault.Holds(d, def) {
		return true
	}
	return false
}

// Rank scores each document's fields against the terms with BM25F.
func Rank(docs []Fields, terms []string) []float64 {
	n := float64(len(docs))
	lengths := make([]float64, len(docs))
	df := map[string]int{}
	total := 0.0
	for i, f := range docs {
		lengths[i] = WeightTitle*float64(len(f.Title)) + WeightAliases*float64(len(f.Aliases)) + WeightTags*float64(len(f.Tags)) + WeightDescription*float64(len(f.Description)) + WeightBody*float64(len(f.Body))
		total += lengths[i]
		seen := map[string]bool{}
		for _, list := range [][]string{f.Title, f.Aliases, f.Tags, f.Description, f.Body} {
			for _, t := range list {
				if !seen[t] {
					seen[t] = true
					df[t]++
				}
			}
		}
	}
	avg := 1.0
	if len(docs) > 0 && total > 0 {
		avg = total / n
	}
	unique := map[string]bool{}
	var query []string
	for _, t := range terms {
		if !unique[t] {
			unique[t] = true
			query = append(query, t)
		}
	}
	scores := make([]float64, len(docs))
	for i, f := range docs {
		tf := map[string]float64{}
		for _, t := range f.Title {
			tf[t] += WeightTitle
		}
		for _, t := range f.Aliases {
			tf[t] += WeightAliases
		}
		for _, t := range f.Tags {
			tf[t] += WeightTags
		}
		for _, t := range f.Description {
			tf[t] += WeightDescription
		}
		for _, t := range f.Body {
			tf[t] += WeightBody
		}
		for _, t := range query {
			w := tf[t]
			if w == 0 {
				continue
			}
			idf := math.Log(1 + (n-float64(df[t])+0.5)/(float64(df[t])+0.5))
			scores[i] += idf * (w * (k1 + 1)) / (w + k1*(1-b+b*lengths[i]/avg))
		}
	}
	return scores
}

// snippet is the body line that holds the most query terms, cut to 200 characters.
func snippet(lines []string, terms []string) string {
	want := map[string]bool{}
	for _, t := range terms {
		want[t] = true
	}
	best, bestN := "", 0
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "|---") {
			continue
		}
		n := 0
		seen := map[string]bool{}
		for _, t := range Tokens(l) {
			if want[t] && !seen[t] {
				seen[t] = true
				n++
			}
		}
		if n > bestN {
			best, bestN = l, n
		}
	}
	if len([]rune(best)) > 200 {
		best = string([]rune(best)[:197]) + "…"
	}
	return best
}

var stop = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true, "be": true, "by": true, "for": true, "from": true,
	"has": true, "have": true, "in": true, "is": true, "it": true, "its": true, "of": true, "on": true, "or": true, "that": true,
	"the": true, "this": true, "to": true, "was": true, "were": true, "will": true, "with": true, "what": true, "which": true,
	"does": true, "do": true, "how": true, "we": true, "our": true, "i": true, "my": true, "about": true,
}

// Tokens splits text into search terms: lower case, letters and digits, without the
// commonest words, and a plural folded to its singular.
func Tokens(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if stop[f] || len(f) < 2 {
			continue
		}
		out = append(out, Stem(f))
	}
	return out
}

// Stem folds a plural to its singular: "ies" to "y", and a trailing "s" that is not "ss".
func Stem(t string) string {
	switch {
	case len(t) > 4 && strings.HasSuffix(t, "ies"):
		return t[:len(t)-3] + "y"
	case len(t) > 3 && strings.HasSuffix(t, "s") && !strings.HasSuffix(t, "ss") && !strings.HasSuffix(t, "us") && !strings.HasSuffix(t, "is"):
		return t[:len(t)-1]
	}
	return t
}
