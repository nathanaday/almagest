package doc

import (
	"strings"
	"testing"
)

func TestParseAndAccessors(t *testing.T) {
	d := Parse("wiki/concepts/Self-supervised learning.md", []byte(`---
id: con-k3m9qa
type: concept
created: 2026-09-27
scope: "[[p3]]"
aliases: [SSL]
tags:
  - vision
  - ml
sources: [[DINOv2]]
active: true
order: 3
---
## Definition
`))
	if d.FrontErr != nil {
		t.Fatal(d.FrontErr)
	}
	if d.Title() != "Self-supervised learning" || d.ID() != "con-k3m9qa" || d.Type() != "concept" {
		t.Fatalf("title %q id %q type %q", d.Title(), d.ID(), d.Type())
	}
	if got := d.Str("created"); got != "2026-09-27" {
		t.Errorf("created %q", got)
	}
	if got := d.Str("scope"); got != "[[p3]]" {
		t.Errorf("scope %q", got)
	}
	if got := strings.Join(d.List("tags"), ","); got != "vision,ml" {
		t.Errorf("tags %q", got)
	}
	if got := d.List("sources"); len(got) != 1 || got[0] != "[[DINOv2]]" {
		t.Errorf("an unquoted link list reads as the link: %q", got)
	}
	if !d.Front.Bool("active") || d.Front.Int("order") != 3 {
		t.Error("bool or int")
	}
	if d.Body != "## Definition\n" {
		t.Errorf("body %q", d.Body)
	}
}

func TestSetFieldKeepsOtherLines(t *testing.T) {
	in := "---\nid: thr-aaaaaa\n# a comment\ntags:\n  - a\n  - b\nmine: kept\n---\nbody\n"
	out := SetField(in, "tags", []string{"c"})
	want := "---\nid: thr-aaaaaa\n# a comment\ntags: [c]\nmine: kept\n---\nbody\n"
	if out != want {
		t.Fatalf("got\n%s\nwant\n%s", out, want)
	}
	if got := SetField(in, "tags", []string{"a", "b"}); got != in {
		t.Fatalf("the same list in another style is no change:\n%s", got)
	}
	added := SetField(want, "stage", "spec")
	if !strings.Contains(added, "mine: kept\nstage: spec\n---") {
		t.Fatalf("a new key goes last:\n%s", added)
	}
	if got := RemoveField(added, "mine"); strings.Contains(got, "mine") {
		t.Fatalf("remove: %s", got)
	}
	if got := SetField("no front\n", "id", "x"); got != "---\nid: x\n---\nno front\n" {
		t.Fatalf("new frontmatter: %q", got)
	}
}

func TestFormatValue(t *testing.T) {
	cases := map[string]any{
		`"[[p3]]"`:             "[[p3]]",
		`plain words`:          "plain words",
		`"true"`:               "true",
		`"42"`:                 "42",
		`2026-09-27`:           "2026-09-27",
		`2026-09-27T14:51:07`:  "2026-09-27T14:51:07",
		`"a: b"`:               "a: b",
		`""`:                   "",
		`["[[a]]", b, "c, d"]`: []string{"[[a]]", "b", "c, d"},
		`[]`:                   []string{},
		`true`:                 true,
		`7`:                    7,
	}
	for want, v := range cases {
		if got := FormatValue(v); got != want {
			t.Errorf("FormatValue(%#v) = %s, want %s", v, got, want)
		}
		// Every value reads back as itself.
		f, err := ParseFront("k: " + FormatValue(v) + "\n")
		if err != nil {
			t.Fatalf("%#v: %v", v, err)
		}
		if !f.Equal("k", v) {
			t.Errorf("%#v does not read back: %q", v, f.value("k"))
		}
	}
}

func TestLeadCallout(t *testing.T) {
	content := "---\nid: x\n---\n\n> [!spec] Old\n> old line\n\n## Goal\n\ntext\n"
	out := ReplaceLead(content, Callout("spec", "New", "a → b"))
	want := "---\nid: x\n---\n\n> [!spec] New\n> a → b\n\n## Goal\n\ntext\n"
	if out != want {
		t.Fatalf("got\n%q\nwant\n%q", out, want)
	}
	user := "---\nid: x\n---\n> [!note] mine\n\ntext\n"
	out = ReplaceLead(user, Callout("task", "T1"))
	if !strings.Contains(out, "> [!task] T1\n\n> [!note] mine") {
		t.Fatalf("a user's callout stays:\n%s", out)
	}
	if ContentHash("> [!spec] a\n\nsome   text\n") != ContentHash("> [!spec] b\n> more\nsome text") {
		t.Error("the lead and whitespace do not change the content hash")
	}
}

func TestSections(t *testing.T) {
	body := "## What\n\ndo it\n\n```\n## not a heading\n```\n\n## Progress\n\n## Result\n"
	if got, ok := Section(body, "What"); !ok || !strings.Contains(got, "not a heading") {
		t.Fatalf("section: %q", got)
	}
	body = AppendSection(body, "Progress", "- 2026-09-27: started")
	body = SetSection(body, "Result", "done")
	if got, _ := Section(body, "Progress"); got != "- 2026-09-27: started" {
		t.Fatalf("progress %q", got)
	}
	if !strings.HasSuffix(body, "## Result\n\ndone\n") {
		t.Fatalf("result: %q", body)
	}
	body = SetSection(body, "Summary", "all")
	if !strings.HasSuffix(body, "## Summary\n\nall\n") {
		t.Fatalf("appended: %q", body)
	}
}

func TestSplitWithoutClosingFence(t *testing.T) {
	if _, _, ok := Split("---\nid: x\nno close\n"); ok {
		t.Fatal("an unclosed fence is no frontmatter")
	}
	front, body, ok := Split("---\n---\nbody")
	if !ok || front != "" || body != "body" {
		t.Fatalf("empty front: %q %q %v", front, body, ok)
	}
}

func TestIDsAndTitles(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		id := NewID("con", func(s string) bool { return seen[s] })
		if !IDPattern.MatchString(id) || strings.ContainsAny(id[4:], "ilou") {
			t.Fatalf("bad id %s", id)
		}
		seen[id] = true
	}
	if got := CleanTitle(" .Fix: the [login] #timeout?  "); got != "Fix the login timeout" {
		t.Fatalf("clean %q", got)
	}
	if LinkTarget("[[A b#h|x]]") != "A b" || LinkTarget("rep-abc123") != "rep-abc123" {
		t.Fatal("link target")
	}
}
