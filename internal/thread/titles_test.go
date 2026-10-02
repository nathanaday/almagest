package thread_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/thread"
)

func TestALongStubTitleIsRefusedBeforeAnyWrite(t *testing.T) {
	tv := testvault.New(t)
	for _, n := range []int{151, 234, 300} {
		title := strings.Repeat("a", n)
		refuses(t, "a title holds at most 150")(thread.Stub(tv.V, thread.StubIn{Text: "An idea.", Title: title}, opts(tv)))
	}
	entries, _ := os.ReadDir(tv.V.Abs("wiki/documents"))
	if len(entries) != 0 {
		t.Fatalf("a refused stub wrote %v", entries)
	}
	r := ok(t)(thread.Stub(tv.V, thread.StubIn{Text: "An idea.", Title: strings.Repeat("b", 150)}, opts(tv)))
	if len(r.Wrote) != 1 {
		t.Fatalf("a title of 150 bytes: %+v", r)
	}
	tv.Clean()
}

func TestAStubTitleLosesControlCharactersAndArrows(t *testing.T) {
	tv := testvault.New(t)
	r := ok(t)(thread.Stub(tv.V, thread.StubIn{Text: "An idea.", Title: "Ring\a the \x00bell → now"}, opts(tv)))
	if got := r.Wrote[0].Title; got != "Ring the bell now" {
		t.Fatalf("title %q", got)
	}
	refuses(t, "the title is empty once cleaned")(thread.Stub(tv.V, thread.StubIn{Text: "An idea.", Title: "\a\x01 →"}, opts(tv)))
	tv.Clean()
}

// A title near the limit still leaves room for the titles code derives from it.
func TestDerivedTitlesOfALongStubFitAFileName(t *testing.T) {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", nil)
	tv.Doc("repository", "p3-edge", map[string]any{"path": repo}, "")
	tv.Commit()
	title := strings.Repeat("c", 150)
	ok(t)(thread.Stub(tv.V, thread.StubIn{Text: "An idea.", Title: title}, opts(tv)))
	ok(t)(thread.Spec(tv.V, thread.SpecIn{Thread: title, Text: specText}, opts(tv)))
	ok(t)(thread.TasksWrite(tv.V, thread.TasksIn{Thread: title, Repository: "p3-edge", Tasks: []thread.TaskIn{{Text: "Score each box", Requirements: []string{"R1", "R2"}}}}, opts(tv)))
	ok(t)(thread.Start(tv.V, title, false, opts(tv)))
	tv.Clean()
}

const (
	cafeNFC = "Café"
	cafeNFD = "Café"
)

// foldsUnicode reports whether the file system under dir stores NFC and NFD names as
// one file, as APFS does.
func foldsUnicode(t *testing.T, dir string) bool {
	t.Helper()
	probe := filepath.Join(dir, "probe-"+cafeNFC)
	if err := os.WriteFile(probe, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(probe)
	_, err := os.Lstat(filepath.Join(dir, "probe-"+cafeNFD))
	return err == nil
}

func TestAStubInAnotherUnicodeFormIsRefused(t *testing.T) {
	tv := testvault.New(t)
	if !foldsUnicode(t, tv.V.Abs("wiki/documents")) {
		t.Skip("this file system keeps NFC and NFD names apart")
	}
	ok(t)(thread.Stub(tv.V, thread.StubIn{Text: "The first.", Title: cafeNFC}, opts(tv)))
	first := tv.Read("wiki/documents/" + cafeNFC + ".md")
	refuses(t, "already exists on disk under another case or Unicode form")(thread.Stub(tv.V, thread.StubIn{Text: "The second.", Title: cafeNFD}, opts(tv)))
	if tv.Read("wiki/documents/"+cafeNFC+".md") != first {
		t.Fatal("the first stub changed")
	}
	tv.Clean()
}
