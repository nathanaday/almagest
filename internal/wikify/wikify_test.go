package wikify_test

import (
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/wikify"
)

const draft = "---\ntags: [draft]\n---\n# Notes on gradient descent\n\nGradient descent needs a learning rate. See `gradient descent` in code, and [[Optimizers|gradient descent tools]].\n\nThe learning rate schedule matters, and momentum helps. Gradient descent again.\n\n```\ngradient descent in a fence\n```\n"

func TestStartCopiesIntoTheScratchpad(t *testing.T) {
	tv := testvault.New(t)
	tv.Write("Notes.md", draft)
	tv.Write("journals/cs566/Week 1.md", "mine\n")
	tv.Commit()
	rel, err := wikify.Start(tv.V, "Notes.md")
	if err != nil || rel != "scratchpad/Notes · wikified.md" || tv.Read(rel) != draft || tv.Read("Notes.md") != draft {
		t.Fatalf("the copy: %s %v", rel, err)
	}
	if again, err := wikify.Start(tv.V, tv.V.Abs("Notes.md")); err != nil || again != "scratchpad/Notes · wikified (2).md" {
		t.Fatalf("a second copy: %s %v", again, err)
	}
	if j, err := wikify.Start(tv.V, "journals/cs566/Week 1.md"); err != nil || j != "scratchpad/Week 1 · wikified.md" {
		t.Fatalf("a journal note: %s %v", j, err)
	}
	for _, refused := range []string{"source-core/documents/X.md", "Atlas.md", "changes/x.md", "Nowhere.md", "image.png", "../out.md"} {
		if _, err := wikify.Start(tv.V, refused); err == nil {
			t.Errorf("%s was copied", refused)
		}
	}
}

func TestPlaceMarksTheFirstFreeMention(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("topic", "Gradient descent", map[string]any{"kind": "concept"}, "")
	tv.Doc("topic", "Learning rate", map[string]any{"kind": "concept"}, "")
	tv.Write("Notes.md", draft)
	tv.Commit()
	rel, err := wikify.Start(tv.V, "Notes.md")
	if err != nil {
		t.Fatal(err)
	}
	res, err := wikify.Place(tv.Index(), rel, []wikify.Mark{
		{Phrase: "gradient descent", Link: "Gradient descent"},
		{Phrase: "learning rate", Link: tv.ID("Learning rate")},
		{Phrase: "learning rate schedule", New: "Learning rate schedule"},
		{Phrase: "momentum", New: "Momentum"},
		{Phrase: "nesterov", New: "Nesterov momentum"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(res.Placed, ",") != "gradient descent,learning rate,learning rate schedule,momentum" || strings.Join(res.Missing, ",") != "nesterov" {
		t.Fatalf("the result: %+v", res)
	}
	got := tv.Read(rel)
	want := strings.NewReplacer(
		"Gradient descent needs a learning rate.", "{{link:Gradient descent|Gradient descent}} needs a {{link:Learning rate|learning rate}}.",
		"The learning rate schedule matters, and momentum helps.", "The {{new:Learning rate schedule|learning rate schedule}} matters, and {{new:Momentum|momentum}} helps.",
	).Replace(draft)
	if got != want {
		t.Fatalf("the note:\n%s\nwant:\n%s", got, want)
	}
	tv.Write(rel, tv.Read(rel)+"\n| Step | Note |\n|---|---|\n| 1 | nesterov in a table |\n")
	if res, err := wikify.Place(tv.Index(), rel, []wikify.Mark{{Phrase: "nesterov", New: "Nesterov momentum"}}); err != nil || len(res.Missing) != 1 {
		t.Fatalf("a phrase only in a table: %+v %v", res, err)
	}
	if _, err := wikify.Place(tv.Index(), "Notes.md", []wikify.Mark{{Phrase: "x", New: "X"}}); err == nil {
		t.Fatal("a note that is no wikified copy")
	}
	if _, err := wikify.Place(tv.Index(), rel, []wikify.Mark{{Phrase: "x", Link: "No such topic"}}); err == nil {
		t.Fatal("a link to nothing")
	}
	if _, err := wikify.Place(tv.Index(), rel, []wikify.Mark{{Phrase: "x", Link: "Learning rate", New: "X"}}); err == nil {
		t.Fatal("a mark with both")
	}
	res, err = wikify.Place(tv.Index(), rel, []wikify.Mark{{Phrase: "Gradient descent", New: "Gradient descent"}})
	if err != nil || len(res.Placed) != 1 || !strings.Contains(tv.Read(rel), "{{link:Gradient descent|Gradient descent}} again") {
		t.Fatalf("a new title the wiki holds is a link: %+v %v\n%s", res, err, tv.Read(rel))
	}
}
