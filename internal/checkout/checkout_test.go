package checkout_test

import (
	"path"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/almagest/internal/change"
	"github.com/nathanaday/almagest/internal/checkout"
	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/lint"
	"github.com/nathanaday/almagest/internal/testvault"
	"github.com/nathanaday/almagest/internal/vault"
)

func library(t *testing.T) *testvault.T {
	tv := testvault.New(t)
	tv.Doc("topic", "Reinforcement learning", map[string]any{"kind": "overview", "description": "Learning by reward."}, "## Summary\n\nAn agent learns from reward. See [[Q-learning]] and [[Policy gradient]].\n")
	tv.Doc("topic", "Q-learning", map[string]any{"kind": "concept", "description": "Learning action values.", "aliases": []string{"Q learning"}}, "## Definition\n\nA reinforcement learning method that learns values. It builds on [[Bellman equation]].\n")
	tv.Doc("topic", "Policy gradient", map[string]any{"kind": "concept", "description": "Learning a policy by gradient."}, "## Definition\n\nA reinforcement learning method on the policy, unlike [[Q-learning|Q learning]].\n")
	tv.Doc("topic", "Bellman equation", map[string]any{"kind": "concept", "description": "A recursion for values."}, "## Definition\n\nValues by recursion.\n")
	tv.Doc("topic", "Sourdough", map[string]any{"kind": "concept", "description": "Bread."}, "## Definition\n\nBread.\n")
	tv.Commit()
	return tv
}

func TestCandidatesFollowLinksFromTheBestHits(t *testing.T) {
	tv := library(t)
	cands, err := checkout.Candidates(tv.Index(), "reinforcement learning", nil, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]checkout.Candidate{}
	for _, c := range cands {
		got[c.Ref.Title] = c
	}
	if _, ok := got["Sourdough"]; ok {
		t.Fatalf("an unrelated topic is a candidate: %+v", cands)
	}
	if c, ok := got["Bellman equation"]; !ok || c.Distance == 0 || c.Via == "" {
		t.Fatalf("a document reached by a link: %+v", cands)
	}
	if cands[0].Distance != 0 || cands[0].Score < got["Bellman equation"].Score {
		t.Fatalf("the order: %+v", cands)
	}
	if _, err := checkout.Candidates(tv.Index(), " ", nil, nil, 0); err == nil {
		t.Fatal("an empty request")
	}
}

func TestMakeAndReturn(t *testing.T) {
	tv := library(t)
	now := tv.Tick(time.Hour)
	m, err := checkout.Make(tv.V, checkout.Order{
		Request: "check out the material on reinforcement learning",
		Name:    "RL",
		Documents: []checkout.Pick{
			{ID: "Reinforcement learning", Why: "the map"},
			{ID: "Q-learning", Why: "values"},
			{ID: "Policy gradient", Why: "policies"},
		},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	tv.Clean()
	folder := "checkout/" + vault.Date(now) + " RL"
	if m.Folder != folder || len(m.Copies) != 3 {
		t.Fatalf("made %+v", m)
	}
	pg := tv.Read(folder + "/Policy gradient (checkout).md")
	for _, want := range []string{"checkout_of: \"[[Policy gradient]]\"", "> [!almagest] A copy of [[Policy gradient]]", "unlike [[" + folder + "/Q-learning (checkout)|Q learning]]."} {
		if !strings.Contains(pg, want) {
			t.Fatalf("the copy lacks %q:\n%s", want, pg)
		}
	}
	if q := tv.Read(folder + "/Q-learning (checkout).md"); !strings.Contains(q, "It builds on [[Bellman equation]].") {
		t.Fatalf("a link out of the checkout keeps naming the wiki:\n%s", q)
	}
	index := tv.Read(m.Index)
	if m.Index != folder+"/_index.md" || !strings.Contains(index, "1. [["+folder+"/Reinforcement learning (checkout)|Reinforcement learning]] · the map\n2. ") || !strings.Contains(index, "\n# RL\n") {
		t.Fatalf("the index:\n%s", index)
	}
	if fm := doc.Parse("", []byte(index)); fm.Str("name") != "RL" || fm.Str("status") != "out" || fm.Str("documents") != "3" {
		t.Fatalf("the index's fields:\n%s", index)
	}
	if ledger := tv.Read("checkout/Checkout · Ledger.md"); ledger != checkout.LedgerNote || !strings.Contains(ledger, `file.inFolder("tool/returned")`) {
		t.Fatalf("the ledger:\n%s", ledger)
	}
	if _, err := checkout.Make(tv.V, checkout.Order{Request: "x", Name: "Y", Documents: []checkout.Pick{{ID: "Q-learning"}, {ID: "Q learning"}}}, now); err == nil {
		t.Fatal("a document named twice")
	}
	f, err := lint.Run(tv.Index(), lint.Options{Now: now})
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range f.Findings {
		if x.Severity == lint.Error {
			t.Errorf("lint after a checkout: %s %s", x.Check, x.Message)
		}
	}

	pgPath := folder + "/Policy gradient (checkout).md"
	tv.Write(pgPath, strings.Replace(pg, "on the policy, unlike", "on the policy itself, unlike", 1))
	qPath := folder + "/Q-learning (checkout).md"
	tv.Write(qPath, tv.Read(qPath)+"\nMy note on Q-learning.\n")
	tv.Write(pgPath, strings.Replace(tv.Read(pgPath), "on the policy itself, unlike", "on the policy itself (see [[Reinforcement learning (checkout)|RL]]), unlike", 1))
	tv.Write(vault.DocPath("Q-learning"), strings.Replace(tv.Read(vault.DocPath("Q-learning")), "learns values", "learns action values", 1))
	tv.Commit()
	if e := checkout.List(tv.V)[0]; e.Edited != 2 {
		t.Fatalf("the edited count: %+v", e)
	}
	if _, err := checkout.Return(tv.V, "RL", tv.Tick(time.Minute)); err == nil {
		t.Fatal("a checkout's folder named without its date")
	}
	r, err := checkout.Return(tv.V, folder, tv.Tick(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Skipped) != 1 || !strings.Contains(r.Skipped[0], "Q-learning changed since the checkout") || r.Change.Counts.Modify != 1 {
		t.Fatalf("the return: %+v", r)
	}
	// The checkout moves to tool/returned/, every file of it, with its links and its index marked.
	returned := "tool/returned/" + vault.Date(now) + " RL"
	if r.Folder != returned || r.Index != returned+"/_index.md" || tv.V.Exists(folder) {
		t.Fatalf("the return's place: %+v", r)
	}
	index = tv.Read(r.Index)
	if fm := doc.Parse("", []byte(index)); fm.Str("status") != "returned" || fm.Str("returned") == "" || fm.Str("return_change") != r.Change.Ref.ID {
		t.Fatalf("the returned index:\n%s", index)
	}
	if !strings.Contains(index, "1. [["+returned+"/Reinforcement learning (checkout)|Reinforcement learning]]") || strings.Contains(index, "[["+folder+"/") {
		t.Fatalf("the returned index's links:\n%s", index)
	}
	// A copy keeps the user's edits, and its links follow it.
	if got := tv.Read(returned + "/Policy gradient (checkout).md"); !strings.Contains(got, "on the policy itself") || !strings.Contains(got, "[["+returned+"/Q-learning (checkout)|Q learning]]") {
		t.Fatalf("the returned copy:\n%s", got)
	}
	if !strings.Contains(tv.Read(returned+"/Q-learning (checkout).md"), "My note on Q-learning.") {
		t.Fatal("the left-out copy lost its edit")
	}
	if l := checkout.List(tv.V); len(l) != 1 || l[0].Status != "returned" || l[0].Folder != returned || l[0].Name != "RL" {
		t.Fatalf("the list after the return: %+v", l)
	}
	if !strings.Contains(tv.Read(r.Change.Ref.Path), "[["+returned+"/_index|") {
		t.Fatalf("the change's link to the checkout:\n%s", tv.Read(r.Change.Ref.Path))
	}
	tv.Clean()
	if f, err := lint.Run(tv.Index(), lint.Options{Now: now}); err != nil || f.Counts[lint.Error] != 0 {
		t.Fatalf("lint after a return: %+v %v", f, err)
	}
	if _, err := change.Apply(tv.V, r.Change.Ref.ID, tv.Tick(time.Minute), nil); err != nil {
		t.Fatal(err)
	}
	got := tv.Read(vault.DocPath("Policy gradient"))
	if !strings.Contains(got, "on the policy itself (see [[Reinforcement learning|RL]]), unlike [[Q-learning|Q learning]].") || strings.Contains(got, "checkout") {
		t.Fatalf("the returned topic:\n%s", got)
	}
	if _, err := checkout.Return(tv.V, folder, tv.Clock); err == nil || !strings.Contains(err.Error(), "was returned already") {
		t.Fatalf("a second return: %v", err)
	}
	if !strings.HasSuffix(r.Change.Ref.Title, " Return RL") {
		t.Fatalf("the change's title: %s", r.Change.Ref.Title)
	}
	tv.Clean()
}

// A checkout with no edit returns too: nothing to propose, and the checkout moves; a
// second one of the same folder name takes a number.
func TestAReturnWithNoEditMovesTheCheckout(t *testing.T) {
	tv := library(t)
	now := tv.Tick(time.Hour)
	order := checkout.Order{Request: "Bellman", Name: "Values", Documents: []checkout.Pick{{ID: "Bellman equation"}}}
	m, err := checkout.Make(tv.V, order, now)
	if err != nil {
		t.Fatal(err)
	}
	// A file of the user's in the checkout goes with it.
	tv.Write(m.Folder+"/My notes.md", "Mine.\n")
	tv.Commit()
	r, err := checkout.Return(tv.V, m.Folder, tv.Tick(time.Minute))
	if err != nil || r.Change != nil || r.Warning != "" {
		t.Fatalf("a return with no edit: %+v %v", r, err)
	}
	if tv.Read(r.Folder+"/My notes.md") != "Mine.\n" || tv.V.Exists(m.Folder) {
		t.Fatalf("the returned checkout: %+v", r)
	}
	if fm := doc.Parse("", []byte(tv.Read(r.Index))); fm.Str("status") != "returned" || fm.Str("return_change") != "" {
		t.Fatalf("the index:\n%s", tv.Read(r.Index))
	}
	if log := tv.Log(); log[0] != "checkout: return "+path.Base(m.Folder) {
		t.Fatalf("the log: %v", log)
	}
	tv.Clean()

	m2, err := checkout.Make(tv.V, order, now)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := checkout.Return(tv.V, m2.Folder, tv.Tick(time.Minute))
	if err != nil || r2.Folder != r.Folder+" (2)" {
		t.Fatalf("a second return of the same name: %+v %v", r2, err)
	}
	if l := checkout.List(tv.V); len(l) != 2 || l[0].Status != "returned" || l[1].Status != "returned" {
		t.Fatalf("the list: %+v", l)
	}
}
