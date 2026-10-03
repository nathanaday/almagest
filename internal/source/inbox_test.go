package source_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/source"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/thread"
)

// outsideFile makes a folder beside the vault, in the test's temporary folder, holding
// secret.txt.
func outsideFile(t *testing.T) (dir, file string) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file = filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(file, []byte("secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, file
}

func TestTheInboxTakesOnlyAFileKeptInIt(t *testing.T) {
	tv := testvault.New(t)
	dir, file := outsideFile(t)
	if err := os.Symlink(dir, tv.V.Abs("inbox/away")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(file, tv.V.Abs("inbox/secret.txt")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"away/secret.txt", "inbox/away/secret.txt", "secret.txt"} {
		if _, err := source.Capture(tv.V, source.Request{Inbox: []string{name}}, at); err == nil || !strings.Contains(err.Error(), "copy the file there") {
			t.Errorf("capture %s: %v", name, err)
		}
		if _, err := thread.Stub(tv.V, thread.StubIn{Text: "Read the secret.", Inbox: name}, at); err == nil || !strings.Contains(err.Error(), "copy the file there") {
			t.Errorf("stub %s: %v", name, err)
		}
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != "secret\n" {
		t.Fatalf("the outside file changed: %q %v", data, err)
	}
	if entries, _ := os.ReadDir(tv.V.Abs("wiki/documents")); len(entries) != 0 {
		t.Fatalf("a refused capture wrote %v", entries)
	}
	tv.Write("inbox/kept.txt", "kept\n")
	if _, err := thread.Stub(tv.V, thread.StubIn{Text: "Read the kept file.", Inbox: "kept.txt"}, at); err != nil {
		t.Fatalf("a file kept in inbox/: %v", err)
	}
	if tv.V.Exists("inbox/kept.txt") {
		t.Fatal("the stub left its inbox file")
	}
}

func TestACaptureWhoseCommitFailsPutsTheInboxBack(t *testing.T) {
	tv := testvault.New(t)
	tv.Write("inbox/notes.md", "# Notes\n\nKept.\n")
	tv.Commit()
	lock := filepath.Join(tv.V.Root, ".git", "index.lock")
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := source.Capture(tv.V, source.Request{Inbox: []string{"notes.md"}}, at)
	os.Remove(lock)
	if err == nil || !strings.Contains(err.Error(), "the vault is back as it was") {
		t.Fatalf("capture with the index locked: %v", err)
	}
	if tv.Read("inbox/notes.md") != "# Notes\n\nKept.\n" {
		t.Fatal("the inbox file is gone or changed")
	}
	if entries, _ := os.ReadDir(tv.V.Abs("wiki/documents")); len(entries) != 0 {
		t.Fatalf("a source stayed: %v", entries)
	}
	if entries, _ := os.ReadDir(tv.V.Abs("wiki/assets")); len(entries) != 0 {
		t.Fatalf("an asset stayed: %v", entries)
	}
}
