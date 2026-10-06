package source_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/source"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
)

var at = testvault.Now

func TestCaptureFromTheInbox(t *testing.T) {
	tv := testvault.New(t)
	tv.Write("inbox/DINOv2.pdf", "%PDF-1.4\n1 0 obj << /Type /Pages /Count 31 >> endobj\n")
	tv.Write("inbox/meeting notes.md", "# Notes\n\nWe met.\n")
	res, err := source.Capture(tv.V, source.Request{Inbox: []string{"DINOv2.pdf", "meeting notes.md"}, Tags: []string{"ML", "paper"}}, at)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Captured) != 2 || tv.V.Exists("inbox/DINOv2.pdf") {
		t.Fatalf("captured %+v", res)
	}
	pdf := res.Captured[0]
	if pdf.Measure != "31 pages" || len(pdf.Chunks) != 2 || pdf.Chunks[1].Locator != "pages 21-31" {
		t.Fatalf("pdf %+v", pdf)
	}
	if pdf.Ref.Path != "wiki/documents/DINOv2.md" || !tv.V.Exists("wiki/assets/"+pdf.Ref.ID+".pdf") {
		t.Fatalf("the source goes in wiki/documents: %s", pdf.Ref.Path)
	}
	page := tv.Read(pdf.Ref.Path)
	for _, want := range []string{"origin: inbox", "authority: unknown", "media: pdf", "tags: [ml, paper]", "status: pending", "locator: DINOv2.pdf", "> [!source] PDF · 31 pages · unknown", "![[" + pdf.Ref.ID + ".pdf]]"} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q:\n%s", want, page)
		}
	}
	if strings.Contains(page, "from:") {
		t.Errorf("a source has no from field:\n%s", page)
	}
	if tv.Log()[0] != "capture: DINOv2, meeting notes" {
		t.Fatalf("log %v", tv.Log())
	}
	tv.Clean()
	idx := tv.Index()
	if !idx.Pending(idx.ByID(pdf.Ref.ID)) {
		t.Fatal("a capture is pending")
	}
	blob, err := source.Read(idx, pdf.Ref.ID, 2)
	if err != nil || blob.Pages != "21-31" || !strings.HasSuffix(blob.File, pdf.Ref.ID+".pdf") || blob.Content != "" {
		t.Fatalf("a PDF is named, not read: %+v %v", blob, err)
	}
	// The same file again is a duplicate, and still leaves the inbox.
	tv.Write("inbox/copy.md", "# Notes\n\nWe met.\n")
	res, err = source.Capture(tv.V, source.Request{Inbox: []string{"copy.md"}}, at)
	if err != nil || res.Captured[0].Duplicate == "" || tv.V.Exists("inbox/copy.md") {
		t.Fatalf("duplicate %+v %v", res, err)
	}
	if _, err := source.Capture(tv.V, source.Request{Inbox: []string{"../Atlas.md"}}, at); err == nil {
		t.Fatal("a name outside the inbox is refused")
	}
}

func TestCaptureTextAndChunkMarkdown(t *testing.T) {
	tv := testvault.New(t)
	var b strings.Builder
	for s := 1; s <= 3; s++ {
		fmt.Fprintf(&b, "# Part %d\n\n", s)
		for i := 0; i < 500; i++ {
			fmt.Fprintf(&b, "line %d of part %d\n", i, s)
		}
	}
	res, err := source.Capture(tv.V, source.Request{Text: b.String(), Title: "Long notes", Locator: "[[2026-09-27 1432 a1b2c3]]"}, at)
	if err != nil {
		t.Fatal(err)
	}
	c := res.Captured[0]
	if c.Measure != "1506 lines" || len(c.Chunks) != 3 || !strings.Contains(c.Chunks[1].Locator, "§ Part 2") {
		t.Fatalf("chunks %+v", c.Chunks)
	}
	blob, err := source.Read(tv.Index(), c.Ref.ID, 2)
	if err != nil || !strings.HasPrefix(blob.Content, "# Part 2") {
		t.Fatalf("read %v %q", err, blob.Content[:40])
	}
	if !strings.Contains(tv.Read(c.Ref.Path), "origin: pasted") {
		t.Fatal("pasted")
	}
}

func TestCaptureARepositorySnapshot(t *testing.T) {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", map[string]string{"go.mod": "module p3\n", "CLAUDE.md": "Use gofmt.\n", "main.go": "package main\n// TODO: score boxes\n", "docs/design.md": "# Design\n"})
	tv.Doc("repository", "p3-edge", map[string]any{"path": repo, "defines": "work/p3/p3-edge"}, "")
	tv.Commit()
	res, err := source.Capture(tv.V, source.Request{Repository: "p3-edge"}, at)
	if err != nil {
		t.Fatal(err)
	}
	c := res.Captured[0]
	if !strings.HasPrefix(c.Ref.Title, "p3-edge @ ") {
		t.Fatalf("title %s", c.Ref.Title)
	}
	page := tv.Read(c.Ref.Path)
	if !strings.Contains(page, "origin: repository") || !strings.Contains(page, "tags: [work/p3/p3-edge]") || !strings.Contains(page, "[["+c.Ref.ID+".md|Open the original (md)]]") {
		t.Fatalf("page:\n%s", page)
	}
	report := tv.Read("wiki/assets/" + c.Ref.ID + ".md")
	for _, want := range []string{"## Tree", "### go.mod", "### CLAUDE.md", "`main.go:2` // TODO: score boxes", "`docs/design.md`"} {
		if !strings.Contains(report, want) {
			t.Errorf("report lacks %q:\n%s", want, report)
		}
	}
	again, err := source.Capture(tv.V, source.Request{Repository: "p3-edge"}, at)
	if err != nil || again.Captured[0].Duplicate != c.Ref.ID {
		t.Fatalf("a snapshot at the same commit is a duplicate: %+v %v", again, err)
	}
}
