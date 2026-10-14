package change_test

import (
	"fmt"
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

func TestAMisnamedLinkIsRefusedWithItsTitle(t *testing.T) {
	tv := upVault(t)
	tv.Doc("topic", "Machine Learning", map[string]any{"id": tv.ID("Machine Learning"), "kind": "overview", "defines": "ml", "aliases": []string{"ML"}}, "")
	tv.Doc("topic", "Fine-Tuning", map[string]any{"id": tv.ID("Fine-Tuning"), "kind": "concept", "tags": []string{"ml"}, "up": []string{"[[ML]]"}}, "")
	tv.Commit()
	_, err := change.Propose(tv.V, change.Plan{Title: "Describe", Writes: []change.Write{
		{Op: "modify", ID: "Fine-Tuning", Fields: map[string]any{"description": "Training a trained model further."}},
	}}, tv.Tick(time.Minute))
	if err == nil || strings.Count(err.Error(), "\n- ") != 1 || !strings.Contains(err.Error(), "up: [[ML]] names Machine Learning by an alias; Obsidian links only a [[title]], so link [[Machine Learning]]") {
		t.Fatalf("want one refusal that names the title, got %v", err)
	}
}

func TestALinkChecksAfterItsRewrite(t *testing.T) {
	tv := upVault(t)
	// A rename moves the up of the document below it, which the same change modifies.
	pv := propose(t, tv, change.Plan{Title: "Rename the page", Writes: []change.Write{
		{Op: "modify", ID: "Training", Fields: map[string]any{"description": "How a model learns."}},
		{Op: "rename", ID: "Machine Learning", Title: "ML Field"},
	}})
	if content := tv.Read(pv.Ref.Path); !strings.Contains(content, `up: ["[[ML Field]]"]`) {
		t.Fatalf("Training's up does not follow the rename:\n%s", content)
	}
	// A link by id to a document the change removes is named for the removal, not its title.
	tv.Doc("topic", "Training", map[string]any{"id": tv.ID("Training"), "kind": "concept", "tags": []string{"ml"}, "up": []string{"[[" + tv.ID("Machine Learning") + "]]"}}, "")
	tv.Commit()
	_, err := change.Propose(tv.V, change.Plan{Title: "Drop the page", Writes: []change.Write{
		{Op: "remove", ID: "Machine Learning"},
		{Op: "modify", ID: "Training", Fields: map[string]any{"description": "How a model learns."}},
	}}, tv.Tick(time.Minute))
	want := "names Machine Learning by its id, and this change removes Machine Learning; change the link"
	if err == nil || !strings.Contains(err.Error(), want) || strings.Contains(err.Error(), "so link [[Machine Learning]]") {
		t.Fatalf("want %q and no advice to link it, got %v", want, err)
	}
}

func TestLinksCheckAsTheChangeLeavesThem(t *testing.T) {
	tv := upVault(t)
	// A rename keeps the type check: Training's up names a source, renamed or not.
	tv.Doc("topic", "Training", map[string]any{"id": tv.ID("Training"), "kind": "concept", "tags": []string{"ml"}, "up": []string{"[[Block 7 Slides]]"}}, "")
	tv.Commit()
	refused(t, tv, change.Plan{Title: "Rename the slides", Writes: []change.Write{
		{Op: "modify", ID: "Training", Fields: map[string]any{"description": "How a model learns."}},
		{Op: "rename", ID: "Block 7 Slides", Title: "B7S"},
	}}, "up: [[B7S]] is a source; it must be a topic or repository")
	// A plan may name a title its own rename frees: the link names the new title.
	pv := propose(t, tv, change.Plan{Title: "Rename and add", Writes: []change.Write{
		{Op: "rename", ID: "Machine Learning", Title: "ML Field"},
		{Op: "create", Type: "topic", Kind: "concept", Title: "Deep Learning", Fields: map[string]any{"description": "Many layers.", "tags": []any{"ml"}, "up": []any{"Machine Learning"}}},
	}})
	if content := tv.Read(pv.Ref.Path); !strings.Contains(content, `up: ["[[ML Field]]"]`) {
		t.Fatalf("the create's up does not name the new title:\n%s", content)
	}
	// A redirect that up cannot take says so.
	tv.Doc("topic", "Training", map[string]any{"id": tv.ID("Training"), "kind": "concept", "tags": []string{"ml"}, "up": []string{"[[Machine Learning]]"}}, "")
	tv.Commit()
	refused(t, tv, change.Plan{Title: "Merge into the slides", Writes: []change.Write{{Op: "remove", ID: "Training", Redirect: "Block 7 Slides"}}},
		"Fine-Tuning: up names Training, which this change removes, and its redirect is no topic or repository")
}

func TestApplyChecksLinksAfterTheRewritesToo(t *testing.T) {
	tv := upVault(t)
	pv := propose(t, tv, change.Plan{Title: "Rename the hub", Writes: []change.Write{
		{Op: "modify", ID: "Fine-Tuning", Fields: map[string]any{"description": "Training a trained model further."}},
		{Op: "rename", ID: "Training", Title: "Model Training"},
	}})
	apply(t, tv, pv.Ref.ID)
	if got := tv.Read("tool/source-core/documents/Fine-Tuning.md"); !strings.Contains(got, `up: ["[[Model Training]]"]`) || !strings.Contains(got, "> Up: [[Model Training]]") {
		t.Fatalf("Fine-Tuning after the apply:\n%s", got)
	}
	pv = propose(t, tv, change.Plan{Title: "Merge the hub", Writes: []change.Write{
		{Op: "modify", ID: "Fine-Tuning", Fields: map[string]any{"description": "Tuning a trained model."}},
		{Op: "remove", ID: "Model Training", Redirect: "Machine Learning"},
	}})
	apply(t, tv, pv.Ref.ID)
	if got := tv.Read("tool/source-core/documents/Fine-Tuning.md"); !strings.Contains(got, `up: ["[[Machine Learning]]"]`) {
		t.Fatalf("Fine-Tuning after the merge:\n%s", got)
	}
}

func TestApplyRefusesABadLinkEditedIntoTheChange(t *testing.T) {
	tv := upVault(t)
	pv := propose(t, tv, change.Plan{Title: "Describe", Writes: []change.Write{
		{Op: "modify", ID: "Fine-Tuning", Fields: map[string]any{"description": "Training a trained model further."}},
	}})
	record := tv.Read(pv.Ref.Path)
	if !strings.Contains(record, `up: ["[[Training]]"]`) {
		t.Fatalf("the change document holds no up to edit:\n%s", record)
	}
	before := tv.Read("tool/source-core/documents/Fine-Tuning.md")
	for edit, want := range map[string]string{
		`up: ["[[Block 7 Slides]]"]`:                    "is a source; it must be a topic or repository",
		`up: ["[[Nowhere]]"]`:                           "[[Nowhere]] names no document",
		`up: ["[[Fine-Tuning]]"]`:                       "up names Fine-Tuning itself",
		`up: ["[[` + tv.ID("Machine Learning") + `]]"]`: "names Machine Learning by its id",
	} {
		tv.Write(pv.Ref.Path, strings.Replace(record, `up: ["[[Training]]"]`, edit, 1))
		_, err := change.Apply(tv.V, pv.Ref.ID, tv.Tick(time.Minute), nil)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("apply with %s: want %q, got %v", edit, want, err)
		}
		if got := tv.Read("tool/source-core/documents/Fine-Tuning.md"); got != before {
			t.Errorf("a refused apply changed Fine-Tuning:\n%s", got)
		}
	}
	// A cycle edited into the change: Machine Learning under Fine-Tuning, which sits under it.
	pv = propose(t, tv, change.Plan{Title: "Describe the page", Writes: []change.Write{
		{Op: "modify", ID: "Machine Learning", Fields: map[string]any{"description": "Learning from data."}},
	}})
	record = tv.Read(pv.Ref.Path)
	tv.Write(pv.Ref.Path, strings.Replace(record, "defines: ml\n", "defines: ml\nup: [\"[[Fine-Tuning]]\"]\n", 1))
	if _, err := change.Apply(tv.V, pv.Ref.ID, tv.Tick(time.Minute), nil); err == nil || !strings.Contains(err.Error(), "reach themselves through their up links") {
		t.Errorf("apply with a cycle: got %v", err)
	}
}

func TestAnOldSelfLinkBlocksNoOtherChange(t *testing.T) {
	tv := upVault(t)
	tv.Doc("topic", "Loner", map[string]any{"kind": "concept", "up": []string{"[[Loner]]"}}, "")
	tv.Commit()
	propose(t, tv, change.Plan{Title: "Unrelated", Writes: []change.Write{{Op: "modify", ID: "Fine-Tuning", Fields: map[string]any{"description": "Tuning a trained model."}}}})
	refused(t, tv, change.Plan{Title: "Self", Writes: []change.Write{{Op: "modify", ID: "Training", Fields: map[string]any{"up": []any{"Training"}}}}}, "up names Training itself")
}

func TestPlainTextIsNoLink(t *testing.T) {
	tv := upVault(t)
	tv.Doc("topic", "Fine-Tuning", map[string]any{"id": tv.ID("Fine-Tuning"), "kind": "concept", "tags": []string{"ml"}, "up": []string{"Training"}, "sources": []string{"Block 7 Slides"}}, "")
	tv.Commit()
	_, err := change.Propose(tv.V, change.Plan{Title: "Merge the hub", Writes: []change.Write{
		{Op: "remove", ID: "Training", Redirect: "Machine Learning"},
		{Op: "modify", ID: "Fine-Tuning", Fields: map[string]any{"description": "Tuning a trained model."}},
	}}, tv.Tick(time.Minute))
	msg := fmt.Sprint(err)
	for _, want := range []string{
		"up: Training names Training as plain text, and this change removes Training; change the link",
		"sources: Block 7 Slides names Block 7 Slides as plain text; Obsidian links only a [[title]], so link [[Block 7 Slides]]",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("want %q in %s", want, msg)
		}
	}
	if strings.Contains(msg, "redirect is no") || strings.Contains(msg, "a , or") {
		t.Errorf("the refusal blames the redirect: %s", msg)
	}
}

func TestARewriteThatCannotMoveAnUpSaysWhy(t *testing.T) {
	tv := upVault(t)
	for _, up := range []string{"Training", "[[TR]]", "[[" + tv.ID("Training") + "]]"} {
		tv.Doc("topic", "Training", map[string]any{"id": tv.ID("Training"), "kind": "concept", "tags": []string{"ml"}, "aliases": []string{"TR"}, "up": []string{"[[Machine Learning]]"}}, "")
		tv.Doc("topic", "Fine-Tuning", map[string]any{"id": tv.ID("Fine-Tuning"), "kind": "concept", "tags": []string{"ml"}, "up": []string{up}}, "")
		tv.Commit()
		_, err := change.Propose(tv.V, change.Plan{Title: "Merge the hub", Writes: []change.Write{{Op: "remove", ID: "Training", Redirect: "Machine Learning"}}}, tv.Tick(time.Minute))
		msg := fmt.Sprint(err)
		if !strings.Contains(msg, "Fine-Tuning: up: "+up+" names Training ") || !strings.Contains(msg, "a rewrite moves only a [[title]], so modify the up of Fine-Tuning") || strings.Contains(msg, "redirect is no") {
			t.Errorf("up %s: %s", up, msg)
		}
	}
}

func TestPlainTextIsNoLinkToACreateEither(t *testing.T) {
	tv := upVault(t)
	tv.Doc("topic", "Fine-Tuning", map[string]any{"id": tv.ID("Fine-Tuning"), "kind": "concept", "tags": []string{"ml"}, "up": []string{"Hub"}}, "")
	tv.Commit()
	refused(t, tv, change.Plan{Title: "Add the hub", Writes: []change.Write{
		{Op: "create", Type: "topic", Kind: "concept", Title: "Hub", Fields: map[string]any{"description": "A hub.", "tags": []any{"ml"}}},
		{Op: "modify", ID: "Fine-Tuning", Fields: map[string]any{"description": "Tuning a trained model."}},
	}}, "up: Hub names Hub as plain text; Obsidian links only a [[title]], so link [[Hub]]")
	tv.Doc("topic", "Fine-Tuning", map[string]any{"id": tv.ID("Fine-Tuning"), "kind": "concept", "tags": []string{"ml"}, "up": []string{"Training"}}, "")
	tv.Commit()
	refused(t, tv, change.Plan{Title: "Make the hub again", Writes: []change.Write{
		{Op: "remove", ID: "Training"},
		{Op: "create", Type: "topic", Kind: "concept", Title: "Training", Fields: map[string]any{"description": "How a model learns.", "tags": []any{"ml"}}},
		{Op: "modify", ID: "Fine-Tuning", Fields: map[string]any{"description": "Tuning a trained model."}},
	}}, "up: Training names Training as plain text; Obsidian links only a [[title]], so link [[Training]]")
}
