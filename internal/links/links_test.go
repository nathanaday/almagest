package links

import "testing"

func TestFindSkipsCodeAndComments(t *testing.T) {
	text := "see [[A]] and ![[b.pdf#page=3]]\n```\n[[not]]\n```\n`[[inline]]` %%[[hidden]]%% [[C#H|shown]]\n"
	got := Find(text)
	if len(got) != 3 || got[0].Target != "A" || !got[1].Embed || got[1].Fragment != "#page=3" || got[2].Alias != "shown" || got[2].Line != 5 {
		t.Fatalf("%+v", got)
	}
}

func TestRewriteKeepsFragmentAliasAndFolder(t *testing.T) {
	in := "[[Old]] [[old#H|x]] ![[Old]] [[wiki/concepts/Old]] [[Older]] `[[Old]]`"
	out, n := Rewrite(in, Rename{"Old": "New"})
	want := "[[New]] [[New#H|x]] ![[New]] [[wiki/concepts/New]] [[Older]] `[[Old]]`"
	if out != want || n != 4 {
		t.Fatalf("got %q (%d)", out, n)
	}
	if out, _ := Rewrite(`| [[Old\|x]] |`, Rename{"Old": "New"}); out != `| [[New\|x]] |` {
		t.Fatalf("a link in a table keeps its escaped pipe: %q", out)
	}
	if l := Parse(`Old#H\|x`); l.Target != "Old" || l.Fragment != "#H" || l.Alias != "x" {
		t.Fatalf("%+v", l)
	}
	if out, n := Rewrite(in, nil); out != in || n != 0 {
		t.Fatal("no rename, no change")
	}
}

func TestKeys(t *testing.T) {
	if Key(" Foo.md ") != "foo" || BaseKey("wiki/Concepts/Foo") != "foo" {
		t.Fatal("keys")
	}
}
