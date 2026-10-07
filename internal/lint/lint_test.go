package lint_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/almagest/internal/lint"
	"github.com/nathanaday/almagest/internal/testvault"
)

func run(t *testing.T, tv *testvault.T, opts lint.Options) *lint.Findings {
	t.Helper()
	opts.Now = testvault.Now
	f, err := lint.Run(tv.Index(), opts)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func has(f *lint.Findings, check, title, text string) bool {
	for _, x := range f.Findings {
		if x.Check == check && x.Doc.Title == title && strings.Contains(x.Message, text) {
			return true
		}
	}
	return false
}

func TestNewVaultLintsClean(t *testing.T) {
	tv := testvault.New(t)
	f := run(t, tv, lint.Options{})
	if len(f.Findings) != 0 {
		t.Fatalf("a new vault has findings: %+v", f.Findings)
	}
	if f.Checked != 1 {
		t.Fatalf("checked %d; Almagest.md is the one document", f.Checked)
	}
	tv.Write("wiki-view/View · Home.md", "[[Nothing]]\n")
	if f := run(t, tv, lint.Options{}); len(f.Findings) != 0 {
		t.Fatalf("lint never reads the views: %+v", f.Findings)
	}
}

func TestChecks(t *testing.T) {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", nil)
	tv.Doc("repository", "p3-edge", map[string]any{"path": repo, "defines": "work/p3/p3-edge", "tags": []string{"work/p3"}}, "")
	tv.Doc("repository", "gone", map[string]any{"path": tv.Dir + "/nowhere"}, "")
	tv.Doc("topic", "Motion scoring", map[string]any{"kind": "concept", "status": "shaky", "sources": []string{"[[Nowhere]]"}}, "Links [[Missing page]] and `[[not a link]]`.\n")
	tv.Doc("topic", "Lonely", map[string]any{"kind": "concept", "sources": []string{"[[Motion scoring]]"}}, "")
	tv.Doc("topic", "Motion", map[string]any{"kind": "entity"}, "see [[Motion scoring]]\n")
	tv.Doc("topic", "Also p3-edge", map[string]any{"kind": "overview", "defines": "work/p3/p3-edge"}, "")
	tv.Doc("topic", "Defines wrong", map[string]any{"kind": "concept", "defines": "x"}, "")
	tv.Doc("topic", "View · Mine", map[string]any{"kind": "concept"}, "")
	tv.Write("scratchpad/Motion scoring.md", "a scratch note with the same title\n")
	tv.Write("tool/source-core/documents/Loose note.md", "no frontmatter\n")
	tv.Write("notes/Stray.md", "---\nid: doc-stray1\ntype: topic\nkind: concept\ndescription: x\n---\n")
	tv.Doc("topic", "Papers", map[string]any{"kind": "concept", "tags": []string{"paper", "papers"}}, "")
	f := run(t, tv, lint.Options{})
	for _, want := range []struct{ check, title, text string }{
		{"repository-path", "gone", "is gone"},
		{"dead-link", "Motion scoring", "[[Nowhere]] names no document"},
		{"schema", "Motion scoring", `status: is "shaky"`},
		{"dead-link", "Motion scoring", "[[Missing page]]"},
		{"duplicate-title", "Motion scoring", "scratchpad/Motion scoring.md"},
		{"duplicate-title", "View · Mine", "view's title"},
		{"uncited", "Motion", "sources are empty"},
		{"orphan", "Lonely", "no other document"},
		{"tag", "p3-edge", "2 documents define work/p3/p3-edge"},
		{"tag", "Defines wrong", "only an overview or a repository"},
		{"misplaced", "Stray", "tools do not see it"},
		{"untyped", "Loose note", "no type"},
		{"tag-near", "Work", "paper, papers"},
	} {
		if !has(f, want.check, want.title, want.text) {
			t.Errorf("no %s finding on %s with %q", want.check, want.title, want.text)
		}
	}
	if has(f, "dead-link", "Motion scoring", "not a link") {
		t.Error("a link in code is not a link")
	}
	if f.Findings[0].Severity != lint.Error {
		t.Error("errors come first")
	}
	quick := run(t, tv, lint.Options{Quick: true})
	for _, x := range quick.Findings {
		if x.Severity != lint.Error || x.Check == "dead-link" && strings.Contains(x.Message, "line") {
			t.Errorf("a quick run makes only frontmatter error checks: %+v", x)
		}
	}
}

func TestTagsLimitTheRun(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("topic", "In p3", map[string]any{"kind": "concept", "tags": []string{"work/p3"}}, "")
	tv.Doc("topic", "At home", map[string]any{"kind": "concept", "tags": []string{"home"}}, "")
	f := run(t, tv, lint.Options{Tags: []string{"work"}})
	for _, x := range f.Findings {
		if x.Doc.Title == "At home" {
			t.Fatalf("a run with tags checks only what holds them: %+v", x)
		}
	}
	if !has(f, "uncited", "In p3", "") {
		t.Fatal("the documents under the tag are checked")
	}
}

func TestPendingAndStale(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("source", "Old paper", map[string]any{"sha256": "abcdef0123456789", "file": "[[x.pdf]]", "refreshed": "2026-09-28T10:00:00"}, "")
	tv.Doc("topic", "Idea", map[string]any{"kind": "concept", "sources": []string{"[[Old paper]]"}, "refreshed": "2026-09-20T10:00:00"}, "")
	tv.Write("changes/2026-09/2026-09-25 Waiting.md", "---\nid: chg-bbbbbb\ntype: change\ncreated: 2026-09-25\nupdated: 2026-09-25\nstatus: proposed\nproposed: 2026-09-25T10:00:00\n---\n")
	f := run(t, tv, lint.Options{Now: testvault.Now.Add(time.Hour)})
	if !has(f, "change-stale", "2026-09-25 Waiting", "proposed") {
		t.Error("a change proposed two days ago is stale")
	}
	if !has(f, "stale", "Idea", "changed after it was refreshed") {
		t.Error("a topic older than what it cites is stale")
	}
}

func TestThreadsArchiveResolvesLinks(t *testing.T) {
	tv := testvault.New(t)
	tv.Write("threads/Fix alarms.md", "---\nid: doc-aaaaaa\ntype: stub\ndescription: x\n---\n\n## Idea\n\n[[Nowhere]]\n")
	tv.Doc("topic", "Alarms", map[string]any{"kind": "concept"}, "Came from [[Fix alarms]].\n")
	f := run(t, tv, lint.Options{})
	for _, x := range f.Findings {
		if x.Check == "dead-link" || x.Doc.Title == "Fix alarms" {
			t.Errorf("lint reads nothing in threads/ and a link to it resolves: %+v", x)
		}
	}
}
