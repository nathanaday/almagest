package change_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/almagest/internal/change"
	"github.com/nathanaday/almagest/internal/testvault"
)

// upVault holds a page of the tag ml, a hub below it, and a topic below the hub.
func upVault(t *testing.T) *testvault.T {
	tv := testvault.New(t)
	tv.Doc("topic", "Machine Learning", map[string]any{"kind": "overview", "defines": "ml"}, "")
	tv.Doc("topic", "Training", map[string]any{"kind": "concept", "tags": []string{"ml"}, "up": []string{"[[Machine Learning]]"}}, "")
	tv.Doc("topic", "Fine-Tuning", map[string]any{"kind": "concept", "tags": []string{"ml"}, "up": []string{"[[Training]]"}}, "")
	tv.Doc("source", "Block 7 Slides", map[string]any{"tags": []string{"ml"}}, "")
	tv.Commit()
	return tv
}

func TestUpTakesTopicsAndRepositories(t *testing.T) {
	tv := upVault(t)
	pv := propose(t, tv, change.Plan{Title: "Add a hub", Writes: []change.Write{
		{Op: "create", Type: "topic", Kind: "concept", Title: "Transfer Learning", Fields: map[string]any{"description": "Reusing a trained model.", "tags": []any{"ml"}, "up": []any{"Training"}}},
		{Op: "modify", ID: "Fine-Tuning", Fields: map[string]any{"up": []any{"[[Transfer Learning]]", "Training"}}},
	}})
	content := tv.Read(pv.Ref.Path)
	for _, want := range []string{`up: ["[[Training]]"]`, `up: ["[[Transfer Learning]]", "[[Training]]"]`} {
		if !strings.Contains(content, want) {
			t.Errorf("the writes lack %q:\n%s", want, content)
		}
	}
}

func TestUpRefusals(t *testing.T) {
	tv := upVault(t)
	modify := func(title string, up ...any) change.Plan {
		return change.Plan{Title: "Change up", Writes: []change.Write{{Op: "modify", ID: title, Fields: map[string]any{"up": up}}}}
	}
	refused(t, tv, modify("Fine-Tuning", "[[Nothing]]"), "names no document; link a document that exists or that this change creates, or drop the link")
	refused(t, tv, modify("Fine-Tuning", "[[Block 7 Slides]]"), "is a source; it must be a topic or repository")
	refused(t, tv, modify("Fine-Tuning", "[[Fine-Tuning]]"), "up names Fine-Tuning itself; drop that link")
	refused(t, tv, modify("Machine Learning", "[[Fine-Tuning]]"), "up: Fine-Tuning, Machine Learning, Training reach themselves through their up links")
	// A remove leaves the up of the topic below it naming nothing.
	refused(t, tv, change.Plan{Title: "Drop the hub", Writes: []change.Write{{Op: "remove", ID: "Training"}}},
		"Fine-Tuning: up names Training, which this change removes; give the remove a redirect, or modify the up of Fine-Tuning")
	// A new page whose tag the hub holds puts the hub below it; the page's own up closes the cycle.
	refused(t, tv, change.Plan{Title: "Page", Writes: []change.Write{
		{Op: "create", Type: "topic", Kind: "overview", Title: "ML Basics", Fields: map[string]any{"description": "The basics.", "defines": "basics", "up": []any{"Fine-Tuning"}}},
		{Op: "modify", ID: "Training", Fields: map[string]any{"up": nil, "tags": []any{"basics"}}},
	}}, "reach themselves through their up links")
}

func TestUpFollowsARedirect(t *testing.T) {
	tv := upVault(t)
	pv := propose(t, tv, change.Plan{Title: "Merge the hub", Writes: []change.Write{{Op: "remove", ID: "Training", Redirect: "Machine Learning"}}})
	content := tv.Read(pv.Ref.Path)
	if !strings.Contains(content, "Fine-Tuning") || !strings.Contains(content, `up: ["[[Machine Learning]]"]`) {
		t.Fatalf("Fine-Tuning's up does not follow the redirect:\n%s", content)
	}
}

func TestAnOldCycleBlocksNoOtherChange(t *testing.T) {
	tv := upVault(t)
	tv.Doc("topic", "A", map[string]any{"kind": "concept", "up": []string{"[[B]]"}}, "")
	tv.Doc("topic", "B", map[string]any{"kind": "concept", "up": []string{"[[A]]"}}, "")
	tv.Commit()
	propose(t, tv, change.Plan{Title: "Unrelated", Writes: []change.Write{{Op: "modify", ID: "Fine-Tuning", Fields: map[string]any{"description": "Training a trained model further."}}}})
}

func TestARemovedTargetGetsOneRefusal(t *testing.T) {
	tv := upVault(t)
	_, err := change.Propose(tv.V, change.Plan{Title: "Drop the hub", Writes: []change.Write{
		{Op: "remove", ID: "Training"},
		{Op: "modify", ID: "Fine-Tuning", Fields: map[string]any{"description": "Training a trained model further."}},
	}}, tv.Tick(time.Minute))
	if err == nil || strings.Count(err.Error(), "\n- ") != 1 || !strings.Contains(err.Error(), "up: [[Training]] names Training, which this change removes; give the remove a redirect, or change the link") {
		t.Fatalf("want one refusal that names the removal, got %v", err)
	}
	refused(t, tv, change.Plan{Title: "Drop the hub", Writes: []change.Write{
		{Op: "remove", ID: "Training"},
		{Op: "modify", ID: "Fine-Tuning", Fields: map[string]any{"up": []any{"Training"}}},
	}}, "up: Training names Training, which this change removes; link a document that stays, or drop the link")
	// With a redirect, the rewrite moves the link, and the modify passes.
	propose(t, tv, change.Plan{Title: "Merge the hub", Writes: []change.Write{
		{Op: "remove", ID: "Training", Redirect: "Machine Learning"},
		{Op: "modify", ID: "Fine-Tuning", Fields: map[string]any{"description": "Training a trained model further."}},
	}})
}
