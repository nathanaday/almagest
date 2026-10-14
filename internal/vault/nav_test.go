package vault_test

import (
	"reflect"
	"testing"

	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/testvault"
	"github.com/nathanaday/almagest/internal/vault"
)

func titles(ds []*doc.Doc) []string {
	out := []string{}
	for _, d := range ds {
		out = append(out, d.Title())
	}
	return out
}

// navVault is a small map: two tag pages, a hub chain with explicit up, documents that
// fall back to their tag's page, and a source.
func navVault(t *testing.T) (*testvault.T, *vault.Index) {
	tv := testvault.New(t)
	topic := func(title string, f map[string]any) {
		f["kind"] = "concept"
		tv.Doc("topic", title, f, "")
	}
	tv.Doc("topic", "Machine Learning", map[string]any{"kind": "overview", "defines": "ml"}, "")
	tv.Doc("topic", "CS566", map[string]any{"kind": "overview", "defines": "ml/cs566", "tags": []string{"ml"}}, "")
	topic("Training", map[string]any{"tags": []string{"ml"}, "up": []string{"[[Machine Learning]]"}})
	topic("Transfer Learning", map[string]any{"tags": []string{"ml/cs566"}, "up": []string{"[[Training]]"}})
	topic("Optimizers", map[string]any{"tags": []string{"ml/cs566"}, "up": []string{"[[Training]]", "[[CS566]]"}})
	topic("Fine-Tuning", map[string]any{"tags": []string{"ml/cs566"}, "up": []string{"[[Transfer Learning]]", "[[Optimizers]]"}, "sources": []string{"[[Block 7 Slides]]"}})
	topic("Backprop", map[string]any{"tags": []string{"ml/cs566", "tool"}})
	topic("Dead Up", map[string]any{"tags": []string{"ml/cs566"}, "up": []string{"[[Nothing]]"}})
	topic("Orphan", map[string]any{"tags": []string{"tool"}})
	tv.Doc("source", "Block 7 Slides", map[string]any{"tags": []string{"ml/cs566"}, "up": []string{"[[Training]]"}}, "")
	return tv, tv.Index()
}

func TestUpTakesUpOrTheTagPage(t *testing.T) {
	_, idx := navVault(t)
	up := func(title string) []string {
		d, err := idx.Resolve(title)
		if err != nil {
			t.Fatal(err)
		}
		return titles(idx.Up(d))
	}
	for title, want := range map[string][]string{
		"Machine Learning":  {},
		"CS566":             {"Machine Learning"},
		"Training":          {"Machine Learning"},
		"Fine-Tuning":       {"Transfer Learning", "Optimizers"},
		"Optimizers":        {"Training", "CS566"},
		"Backprop":          {"CS566"},
		"Block 7 Slides":    {"CS566"},
		"Dead Up":           {},
		"Orphan":            {},
		"Transfer Learning": {"Training"},
	} {
		if got := up(title); !reflect.DeepEqual(got, want) {
			t.Errorf("Up(%s) = %v, want %v", title, got, want)
		}
	}
}

func TestBelowAndUnder(t *testing.T) {
	_, idx := navVault(t)
	get := func(title string) *doc.Doc {
		d, err := idx.Resolve(title)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	if got := titles(idx.Below(get("CS566"))); !reflect.DeepEqual(got, []string{"Backprop", "Block 7 Slides", "Optimizers"}) {
		t.Errorf("Below(CS566) = %v", got)
	}
	if got := titles(idx.Below(get("Training"))); !reflect.DeepEqual(got, []string{"Optimizers", "Transfer Learning"}) {
		t.Errorf("Below(Training) = %v", got)
	}
	// Fine-Tuning sits below two documents under Training, and counts once.
	if n := idx.Under(get("Training")); n != 3 {
		t.Errorf("Under(Training) = %d, want 3", n)
	}
	if n := idx.Under(get("Machine Learning")); n != 7 {
		t.Errorf("Under(Machine Learning) = %d, want 7", n)
	}
	if len(idx.UpCycles()) != 0 {
		t.Errorf("cycles: %v", idx.UpCycles())
	}
}

func TestUpCycles(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("topic", "A", map[string]any{"kind": "concept", "up": []string{"[[B]]"}}, "")
	tv.Doc("topic", "B", map[string]any{"kind": "concept", "up": []string{"[[A]]"}}, "")
	tv.Doc("topic", "C", map[string]any{"kind": "concept", "up": []string{"[[A]]"}}, "")
	tv.Doc("topic", "Page", map[string]any{"kind": "overview", "defines": "x", "up": []string{"[[Child]]"}}, "")
	tv.Doc("topic", "Child", map[string]any{"kind": "concept", "tags": []string{"x"}}, "")
	cycles := tv.Index().UpCycles()
	var got [][]string
	for _, c := range cycles {
		got = append(got, titles(c))
	}
	if len(got) != 2 {
		t.Fatalf("cycles = %v, want A-B and Page-Child", got)
	}
}

func TestCyclesFindsSelfEdges(t *testing.T) {
	got := vault.Cycles([]string{"a", "b"}, map[string][]string{"a": {"a"}, "b": {"a"}})
	if !reflect.DeepEqual(got, [][]string{{"a"}}) {
		t.Fatalf("cycles = %v", got)
	}
}

func TestUpIgnoresAnIdOrAnAlias(t *testing.T) {
	tv := testvault.New(t)
	ml := tv.Doc("topic", "Machine Learning", map[string]any{"kind": "overview", "defines": "ml", "aliases": []string{"ML"}}, "")
	tv.Doc("topic", "By id", map[string]any{"kind": "concept", "tags": []string{"ml"}, "up": []string{"[[" + ml + "]]"}}, "")
	tv.Doc("topic", "By alias", map[string]any{"kind": "concept", "tags": []string{"ml"}, "up": []string{"[[ML]]"}}, "")
	idx := tv.Index()
	for _, title := range []string{"By id", "By alias"} {
		d, _ := idx.Resolve(title)
		if got := titles(idx.Up(d)); len(got) != 0 {
			t.Errorf("Up(%s) = %v; the navigator matches neither an id nor an alias, and a value in up stops the fallback", title, got)
		}
	}
	tv.Doc("topic", "As text", map[string]any{"kind": "concept", "tags": []string{"ml"}, "up": []string{"Machine Learning"}}, "")
	idx = tv.Index()
	if d, _ := idx.Resolve("As text"); len(idx.Up(d)) != 0 {
		t.Errorf("Up(As text) = %v; plain text is no link", titles(idx.Up(d)))
	}
	for link, want := range map[string]string{"[[" + ml + "]]": "by its id", "[[ML]]": "by an alias", "Machine Learning": "as plain text", "[[Machine Learning]]": "", "[[Nothing]]": "", "Nothing": ""} {
		if _, by := idx.Misnamed(link); by != want {
			t.Errorf("Misnamed(%s) = %q, want %q", link, by, want)
		}
	}
}

func TestNeighbors(t *testing.T) {
	_, idx := navVault(t)
	refTitles := func(rs []vault.Ref) []string {
		out := []string{}
		for _, r := range rs {
			out = append(out, r.Title)
		}
		return out
	}
	type groups struct{ up, beside, below, sourcesBelow, sources, citedBy []string }
	for title, want := range map[string]groups{
		"Machine Learning":  {below: []string{"CS566", "Training"}},
		"CS566":             {up: []string{"Machine Learning"}, below: []string{"Backprop", "Optimizers"}, sourcesBelow: []string{"Block 7 Slides"}},
		"Transfer Learning": {up: []string{"Training"}, beside: []string{"Optimizers"}, below: []string{"Fine-Tuning"}},
		// Fine-Tuning sits below Optimizers and shares Training with it: it stands in Below only.
		"Optimizers":     {up: []string{"CS566", "Training"}, beside: []string{"Transfer Learning"}, below: []string{"Fine-Tuning"}},
		"Fine-Tuning":    {up: []string{"Optimizers", "Transfer Learning"}, sources: []string{"Block 7 Slides"}},
		"Block 7 Slides": {up: []string{"CS566"}, citedBy: []string{"Fine-Tuning"}},
		// The fallback puts nothing beside: Backprop's siblings would be a whole tag.
		"Backprop": {up: []string{"CS566"}},
	} {
		d, err := idx.Resolve(title)
		if err != nil {
			t.Fatal(err)
		}
		n := idx.Neighbors(d)
		got := groups{refTitles(n.Up), refTitles(n.Beside), refTitles(n.Below), refTitles(n.SourcesBelow), refTitles(n.Sources), refTitles(n.CitedBy)}
		for _, g := range []*[]string{&want.up, &want.beside, &want.below, &want.sourcesBelow, &want.sources, &want.citedBy} {
			if *g == nil {
				*g = []string{}
			}
		}
		if !reflect.DeepEqual(got, want) || n.Document.Title != title {
			t.Errorf("Neighbors(%s) = %+v, want %+v", title, got, want)
		}
	}
}
