package doc

import (
	"testing"
)

func TestTextHelpers(t *testing.T) {
	if got := NonNil(nil); got == nil || len(got) != 0 {
		t.Fatalf("NonNil(nil) = %#v", got)
	}
	if got := OneLine("  a\n  b\tc ", 10); got != "a b c" {
		t.Fatalf("OneLine folds spaces: %q", got)
	}
	if got := OneLine("abcdef", 4); got != "abc…" {
		t.Fatalf("OneLine cuts to n runes: %q", got)
	}
	if Plural(1, "file", "files") != "file" || Plural(0, "file", "files") != "files" || Plural(2, "file", "files") != "files" {
		t.Fatal("Plural")
	}
	if Capital("started") != "Started" || Capital("") != "" || Capital("école") != "École" {
		t.Fatal("Capital")
	}
	if LineCount("") != 0 || LineCount("a") != 1 || LineCount("a\nb\n\n") != 2 {
		t.Fatal("LineCount")
	}
}
