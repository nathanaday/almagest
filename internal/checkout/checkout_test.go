package checkout_test

import (
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
	list := tv.Read(m.ReadingList)
	if !strings.Contains(list, "1. [["+folder+"/Reinforcement learning (checkout)|Reinforcement learning]] · the map\n2. ") {
		t.Fatalf("the reading list:\n%s", list)
	}
	if ledger := tv.Read("checkout/Checkout · Ledger.md"); !strings.Contains(ledger, "| 3 | 0 | no |") {
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

	if _, err := checkout.Return(tv.V, folder, now); err == nil || !strings.Contains(err.Error(), "nothing to return") {
		t.Fatalf("a return with no edit: %v", err)
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
	r, err := checkout.Return(tv.V, "RL", tv.Tick(time.Minute))
	if err == nil {
		t.Fatal("a checkout's folder named without its date")
	}
	r, err = checkout.Return(tv.V, folder, tv.Tick(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Skipped) != 1 || !strings.Contains(r.Skipped[0], "Q-learning changed since the checkout") || r.Change.Counts.Modify != 1 {
		t.Fatalf("the return: %+v", r)
	}
	if _, err := change.Apply(tv.V, r.Change.Ref.ID, tv.Tick(time.Minute), nil); err != nil {
		t.Fatal(err)
	}
	got := tv.Read(vault.DocPath("Policy gradient"))
	if !strings.Contains(got, "on the policy itself (see [[Reinforcement learning|RL]]), unlike [[Q-learning|Q learning]].") || strings.Contains(got, "checkout") {
		t.Fatalf("the returned topic:\n%s", got)
	}
	if rl := doc.Parse("", []byte(tv.Read(m.ReadingList))); rl.Str("returned") == "" {
		t.Fatal("the reading list does not record the return")
	}
	if _, err := checkout.Return(tv.V, folder, tv.Clock); err == nil || !strings.Contains(err.Error(), "was returned") {
		t.Fatalf("a second return: %v", err)
	}
	if !strings.HasSuffix(r.Change.Ref.Title, " Return RL") {
		t.Fatalf("the change's title: %s", r.Change.Ref.Title)
	}
	tv.Clean()
}

// Cancel of a return's change frees the checkout to return again.
func TestACancelledReturnFreesTheCheckout(t *testing.T) {
	tv := library(t)
	now := tv.Tick(time.Hour)
	m, err := checkout.Make(tv.V, checkout.Order{Request: "Bellman", Name: "Values", Documents: []checkout.Pick{{ID: "Bellman equation"}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	cp := m.Copies[0]
	tv.Write(cp, tv.Read(cp)+"\nA note.\n")
	tv.Commit()
	r, err := checkout.Return(tv.V, m.Folder, tv.Tick(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if e := checkout.List(tv.V)[0]; e.Returned == "" {
		t.Fatalf("a return with its change proposed: %+v", e)
	}
	if _, err := change.Reject(tv.V, r.Change.Ref.ID, "not yet", tv.Tick(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if e := checkout.List(tv.V)[0]; e.Returned != "" {
		t.Fatalf("a return whose change was cancelled: %+v", e)
	}
	if _, err := checkout.Return(tv.V, m.Folder, tv.Tick(time.Minute)); err != nil {
		t.Fatalf("the second return: %v", err)
	}
}
