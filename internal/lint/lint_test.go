package lint_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/lint"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
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
		t.Fatalf("checked %d; Atlas.md is the one document", f.Checked)
	}
}

func TestChecks(t *testing.T) {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", nil)
	tv.Page("area", "p3", nil, "")
	tv.Write("wiki/concepts/Stray.md", "---\nid: are-stray1\ntype: area\ncreated: 2026-09-27\nupdated: 2026-09-27\ndescription: x\n---\n")
	tv.Write("wiki/p3/Loose.md", "---\nid: con-loose1\ntype: concept\ncreated: 2026-09-27\nupdated: 2026-09-27\ndescription: x\n---\n")
	tv.Write("wiki/concepts/Sources/Sources.md", "---\nid: are-rsrvd1\ntype: area\ncreated: 2026-09-27\nupdated: 2026-09-27\ndescription: x\n---\n")
	tv.Page("repository", "p3-edge", map[string]any{"path": repo, "parent": "[[p3]]"}, "")
	tv.Page("repository", "gone", map[string]any{"path": tv.Dir + "/nowhere"}, "")
	tv.Page("concept", "Motion scoring", map[string]any{"scope": "[[Nowhere]]", "status": "shaky"}, "Links [[Missing page]] and `[[not a link]]`.\n")
	tv.Page("concept", "Lonely", map[string]any{"sources": []string{"[[Motion scoring]]"}}, "")
	tv.Page("entity", "Motion", nil, "see [[Motion scoring]]\n")
	tv.Write("scratchpad/Motion scoring.md", "a scratch note with the same title\n")
	tv.Write("threads/X/X — Spec.md", "---\nid: spc-aaaaaa\ntype: spec\nthread: \"[[X]]\"\nthread_id: thr-zzzzzz\ncreated: 2026-09-27\nupdated: 2026-09-27\n---\n")
	f := run(t, tv, lint.Options{})
	for _, want := range []struct{ check, title, text string }{
		{"layout", "Stray", "outside a folder of its own"},
		{"layout", "Loose", "outside the concepts folder"},
		{"layout", "Sources", "name of a type folder"},
		{"repository-path", "gone", "is gone"},
		{"scope", "Motion scoring", "[[Nowhere]] names no document"},
		{"schema", "Motion scoring", `status: is "shaky"`},
		{"dead-link", "Motion scoring", "[[Missing page]]"},
		{"duplicate-title", "Motion scoring", "scratchpad/Motion scoring.md"},
		{"uncited", "Motion", "sources are empty"},
		{"orphan", "Lonely", "no other document"},
		{"thread", "X — Spec", "not a stub"},
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

func TestScopeLimitsTheRun(t *testing.T) {
	tv := testvault.New(t)
	tv.Page("area", "p3", nil, "")
	tv.Page("area", "home", nil, "")
	tv.Page("concept", "In p3", map[string]any{"scope": "[[p3]]"}, "")
	tv.Page("concept", "At home", map[string]any{"scope": "[[home]]"}, "")
	f := run(t, tv, lint.Options{Scope: "p3"})
	for _, x := range f.Findings {
		if x.Doc.Title == "At home" || x.Doc.Title == "home" {
			t.Fatalf("a scoped run checks only its scope: %+v", x)
		}
	}
	if !has(f, "uncited", "In p3", "") {
		t.Fatal("the scope's own pages are checked")
	}
}

func TestPendingAndStaleChanges(t *testing.T) {
	tv := testvault.New(t)
	tv.Page("source", "Old paper", map[string]any{"sha256": "abcdef0123456789", "file": "[[x.pdf]]", "created": "2026-09-01"}, "")
	tv.Write("changes/2026-09/2026-09-25 Waiting.md", "---\nid: chg-bbbbbb\ntype: change\ncreated: 2026-09-25\nupdated: 2026-09-25\nstatus: proposed\nproposed: 2026-09-25T10:00:00\n---\n")
	f := run(t, tv, lint.Options{Now: testvault.Now.Add(time.Hour)})
	if !has(f, "change-stale", "2026-09-25 Waiting", "proposed") {
		t.Error("a change proposed two days ago is stale")
	}
}
