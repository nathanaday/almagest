package thread_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/thread"
)

// porcelain is git status of the vault, empty when the tree is clean.
func porcelain(t *testing.T, tv *testvault.T) string {
	t.Helper()
	out, err := exec.Command("git", "-C", tv.V.Root, "status", "--porcelain").CombinedOutput()
	if err != nil {
		t.Fatalf("git status: %v %s", err, out)
	}
	return strings.TrimSpace(string(out))
}

func docs(t *testing.T, tv *testvault.T) []string {
	t.Helper()
	entries, _ := os.ReadDir(tv.V.Abs("wiki/documents"))
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func TestAWriteWhoseCommitFailsPutsTheVaultBack(t *testing.T) {
	tv := testvault.New(t)
	ok(t)(thread.Stub(tv.V, thread.StubIn{Text: "An idea.", Title: "Idea"}, opts(tv)))
	stub := tv.Read("wiki/documents/Idea.md")
	before := docs(t, tv)
	lock := filepath.Join(tv.V.Root, ".git", "index.lock")
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	refuses(t, "the vault is back as it was")(thread.Note(tv.V, "Idea", "A note that cannot land.", opts(tv)))
	os.Remove(lock)
	if tv.Read("wiki/documents/Idea.md") != stub {
		t.Fatal("the stub changed")
	}
	if after := docs(t, tv); strings.Join(after, "|") != strings.Join(before, "|") {
		t.Fatalf("the documents changed:\n%v\n%v", before, after)
	}
	if st := porcelain(t, tv); st != "" {
		t.Fatalf("the tree is not clean:\n%s", st)
	}
	ok(t)(thread.Note(tv.V, "Idea", "The note lands now.", opts(tv)))
}

// A write refused part way, at the canvas of a chord whose folder links out of the
// vault, leaves no document behind, and the same call passes once the folder is plain.
func TestAWriteRefusedPartWayLeavesNothing(t *testing.T) {
	tv := testvault.New(t)
	ok(t)(thread.ChordCreate(tv.V, thread.ChordIn{Title: "Plan C", Text: "Ship it.", Threads: []thread.ChordThreadIn{{Title: "First", Text: "Do the first part."}}}, opts(tv)))
	away := filepath.Join(filepath.Dir(tv.V.Root), "away")
	if err := os.MkdirAll(away, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(tv.V.Abs("chords")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(away, tv.V.Abs("chords")); err != nil {
		t.Fatal(err)
	}
	refuses(t, "chords is a link")(thread.Stub(tv.V, thread.StubIn{Text: "Unrelated.", Title: "Zed"}, opts(tv)))
	refuses(t, "the vault is back as it was before this call")(thread.Stub(tv.V, thread.StubIn{Text: "Unrelated.", Title: "Zed"}, opts(tv)))
	if tv.V.Exists("wiki/documents/Zed.md") {
		t.Fatal("the refused stub stayed on disk")
	}
	os.Remove(tv.V.Abs("chords"))
	os.MkdirAll(tv.V.Abs("chords"), 0o755)
	ok(t)(thread.Stub(tv.V, thread.StubIn{Text: "Unrelated.", Title: "Zed"}, opts(tv)))
}

// A document that is a link inside the vault stays a link with its target when a write
// to it fails at its commit.
func TestAFailedWriteKeepsALinkedDocumentALink(t *testing.T) {
	tv := testvault.New(t)
	ok(t)(thread.Stub(tv.V, thread.StubIn{Text: "An idea.", Title: "Idea"}, opts(tv)))
	real := tv.V.Abs("scratchpad/Idea.md")
	if err := os.Rename(tv.V.Abs("wiki/documents/Idea.md"), real); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../scratchpad/Idea.md", tv.V.Abs("wiki/documents/Idea.md")); err != nil {
		t.Fatal(err)
	}
	tv.Commit()
	lock := filepath.Join(tv.V.Root, ".git", "index.lock")
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	refuses(t, "the vault is back as it was before this call")(thread.Note(tv.V, "Idea", "A note that cannot land.", opts(tv)))
	os.Remove(lock)
	target, err := os.Readlink(tv.V.Abs("wiki/documents/Idea.md"))
	if err != nil || target != "../../scratchpad/Idea.md" {
		t.Fatalf("the link is gone or changed: %q %v", target, err)
	}
	if st := porcelain(t, tv); st != "" {
		t.Fatalf("the tree is not clean:\n%s", st)
	}
}
