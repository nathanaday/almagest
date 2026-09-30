package tags

import (
	"slices"
	"testing"
)

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{"#Self Driving": "self-driving", "School/CS513": "school/cs513", " ml_ops ": "ml-ops", "a//b": "", "1984": "", "a/-/b": ""} {
		got, err := Normalize(in)
		if want == "" {
			if err == nil {
				t.Errorf("%q: want a refusal, got %q", in, got)
			}
			continue
		}
		if err != nil || got != want {
			t.Errorf("%q: got %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestTree(t *testing.T) {
	if !slices.Equal(Ancestors("a/b/c"), []string{"a", "a/b"}) || Parent("a/b/c") != "a/b" || Parent("a") != "" || Leaf("a/b") != "b" || Top("a/b") != "a" {
		t.Fatal("tree helpers")
	}
	if !Holds([]string{"school/cs513/hw1"}, "school") || Holds([]string{"schools"}, "school") || !HoldsAll([]string{"school/cs513", "self-driving"}, []string{"school", "self-driving"}) {
		t.Fatal("holds")
	}
	if got := Expand([]string{"a/b", "c"}); !slices.Equal(got, []string{"a", "a/b", "c"}) {
		t.Fatal(got)
	}
}

func TestRename(t *testing.T) {
	got, ok := RenameList([]string{"p3", "p3/edge", "work/p3"}, "p3", "work/p3")
	if !ok || !slices.Equal(got, []string{"work/p3", "work/p3/edge"}) {
		t.Fatal(got)
	}
	text := "Fix it #p3/edge soon. `#p3 in code` and #p3x stays.\n```\n#p3\n```\n- [ ] #todo #P3"
	out, n := RenameInline(text, "p3", "work/p3")
	want := "Fix it #work/p3/edge soon. `#p3 in code` and #p3x stays.\n```\n#p3\n```\n- [ ] #todo #work/p3"
	if out != want || n != 2 {
		t.Fatalf("%d\n%s", n, out)
	}
	if got := Inline("a #todo and #x/y, not a#b"); !slices.Equal(got, []string{"todo", "x/y"}) {
		t.Fatal(got)
	}
}
