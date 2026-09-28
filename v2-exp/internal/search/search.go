// Package search ranks the typed documents of a vault against a query with BM25 over
// four fields: the title, the aliases, the description, and the body. It reads the vault
// on each call, so no index file exists to go stale.
package search

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/doc"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/lint"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/schema"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/vault"
)

// Weights of the fields.
const (
	WeightTitle       = 3.0
	WeightAliases     = 3.0
	WeightDescription = 2.0
	WeightBody        = 1.0
	k1                = 1.2
	b                 = 0.75
	DefaultLimit      = 20
)

// Query is what to search for.
type Query struct {
	Text  string              `json:"text,omitempty" jsonschema:"free text; may be empty when the filters say enough"`
	Types []string            `json:"types,omitempty" jsonschema:"document types; empty means every type but change"`
	Scope string              `json:"scope,omitempty" jsonschema:"an area or repository (id or title): this scope and every scope below it; empty is the whole vault"`
	State map[string][]string `json:"state,omitempty" jsonschema:"filters on a Doc Ref's state, such as {stage: [stub, spec, tasks]} or {status: [proposed]}"`
	Limit int                 `json:"limit,omitempty" jsonschema:"at most this many hits; 20 when 0"`
}

// Hit is one ranked document.
type Hit struct {
	Ref     vault.Ref `json:"ref"`
	Score   float64   `json:"score"`
	Snippet string    `json:"snippet,omitempty"`
}

// Hits is the output of a search.
type Hits struct {
	Hits  []Hit `json:"hits"`
	Total int   `json:"total"`
}

// Fields are a document's text, as search ranks it.
type Fields struct {
	Title       []string
	Aliases     []string
	Description []string
	Body        []string
	Lines       []string
}

// FieldsOf reads the four fields of a document. A change's Writes, which hold copies of
// pages, and a lead callout are never ranked.
func FieldsOf(d *doc.Doc) Fields {
	body := doc.StripLead(lint.Checked(d))
	return Fields{
		Title:       Tokens(vault.Title(d)),
		Aliases:     Tokens(strings.Join(d.List("aliases"), " ")),
		Description: Tokens(d.Str("description")),
		Body:        Tokens(body),
		Lines:       strings.Split(body, "\n"),
	}
}

// Search runs a query.
func Search(idx *vault.Index, q Query) (*Hits, error) {
	for _, t := range q.Types {
		if !schema.Is(t) {
			return nil, fmt.Errorf("type %q is not a document type; the types are %s", t, strings.Join(schema.Names(), ", "))
		}
	}
	scope := ""
	if strings.TrimSpace(q.Scope) != "" {
		s, err := idx.ResolveType(q.Scope, "area", "repository")
		if err != nil {
			return nil, fmt.Errorf("scope: %w", err)
		}
		scope = s.ID()
	}
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	var cands []*doc.Doc
	for _, d := range idx.Docs {
		if !wanted(d, q.Types) {
			continue
		}
		if scope != "" && !inScope(idx, d, scope) {
			continue
		}
		cands = append(cands, d)
	}
	var refs []vault.Ref
	var kept []*doc.Doc
	for _, d := range cands {
		ref := idx.Ref(d)
		if !stateMatches(ref.State, q.State) {
			continue
		}
		refs = append(refs, ref)
		kept = append(kept, d)
	}
	terms := Tokens(q.Text)
	out := &Hits{Hits: []Hit{}}
	if len(terms) == 0 {
		order := make([]int, len(kept))
		for i := range order {
			order[i] = i
		}
		sort.SliceStable(order, func(i, j int) bool {
			return kept[order[i]].Str("updated") > kept[order[j]].Str("updated")
		})
		out.Total = len(kept)
		for _, i := range order {
			if len(out.Hits) == limit {
				break
			}
			out.Hits = append(out.Hits, Hit{Ref: refs[i]})
		}
		return out, nil
	}
	fields := make([]Fields, len(kept))
	for i, d := range kept {
		fields[i] = FieldsOf(d)
	}
	scores := Rank(fields, terms)
	order := make([]int, 0, len(kept))
	for i, s := range scores {
		if s > 0 {
			order = append(order, i)
		}
	}
	sort.SliceStable(order, func(i, j int) bool { return scores[order[i]] > scores[order[j]] })
	out.Total = len(order)
	for _, i := range order {
		if len(out.Hits) == limit {
			break
		}
		out.Hits = append(out.Hits, Hit{Ref: refs[i], Score: math.Round(scores[i]*100) / 100, Snippet: snippet(fields[i].Lines, terms)})
	}
	return out, nil
}

func wanted(d *doc.Doc, types []string) bool {
	if len(types) == 0 {
		return d.Type() != "change"
	}
	for _, t := range types {
		if d.Type() == t {
			return true
		}
	}
	return false
}

// inScope reports whether a document belongs to scope: a scope page by its place in the
// graph, any other by its scope.
func inScope(idx *vault.Index, d *doc.Doc, scope string) bool {
	switch d.Type() {
	case "area", "repository":
		return idx.Under(d, scope)
	}
	return idx.InScope(d, scope)
}

// stateMatches reports whether a Doc Ref's state holds one of the wanted values for each
// key. A state value that is a list matches when it holds one of them.
func stateMatches(state map[string]any, want map[string][]string) bool {
	for k, values := range want {
		if len(values) == 0 {
			continue
		}
		have, ok := state[k]
		if !ok {
			return false
		}
		if !valueIn(have, values) {
			return false
		}
	}
	return true
}

func valueIn(have any, values []string) bool {
	switch x := have.(type) {
	case []string:
		for _, h := range x {
			if valueIn(h, values) {
				return true
			}
		}
		return false
	case bool:
		have = strconv.FormatBool(x)
	case int:
		have = strconv.Itoa(x)
	}
	s := fmt.Sprint(have)
	for _, v := range values {
		if strings.EqualFold(s, v) {
			return true
		}
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
		lengths[i] = WeightTitle*float64(len(f.Title)) + WeightAliases*float64(len(f.Aliases)) + WeightDescription*float64(len(f.Description)) + WeightBody*float64(len(f.Body))
		total += lengths[i]
		seen := map[string]bool{}
		for _, list := range [][]string{f.Title, f.Aliases, f.Description, f.Body} {
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
