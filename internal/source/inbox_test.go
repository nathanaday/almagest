package source_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/almagest/internal/source"
	"github.com/nathanaday/almagest/internal/testvault"
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
	if err := os.Symlink(dir, tv.V.Abs("ingest/away")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(file, tv.V.Abs("ingest/secret.txt")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"away/secret.txt", "ingest/away/secret.txt", "secret.txt"} {
		if _, err := source.Capture(tv.V, source.Request{Ingest: []string{name}}, at); err == nil || !strings.Contains(err.Error(), "copy the file there") {
			t.Errorf("capture %s: %v", name, err)
		}
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != "secret\n" {
		t.Fatalf("the outside file changed: %q %v", data, err)
	}
	if entries, _ := os.ReadDir(tv.V.Abs("source-core/documents")); len(entries) != 0 {
		t.Fatalf("a refused capture wrote %v", entries)
	}
	tv.Write("ingest/kept.txt", "kept\n")
	if _, err := source.Capture(tv.V, source.Request{Ingest: []string{"kept.txt"}}, at); err != nil {
		t.Fatalf("a file kept in ingest/: %v", err)
	}
	if tv.V.Exists("ingest/kept.txt") {
		t.Fatal("the capture left its inbox file")
	}
}

func TestACaptureWhoseCommitFailsPutsTheInboxBack(t *testing.T) {
	tv := testvault.New(t)
	tv.Write("ingest/notes.md", "# Notes\n\nKept.\n")
	tv.Commit()
	lock := filepath.Join(tv.V.Root, ".git", "index.lock")
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := source.Capture(tv.V, source.Request{Ingest: []string{"notes.md"}}, at)
	os.Remove(lock)
	if err == nil || !strings.Contains(err.Error(), "the vault is back as it was") {
		t.Fatalf("capture with the index locked: %v", err)
	}
	if tv.Read("ingest/notes.md") != "# Notes\n\nKept.\n" {
		t.Fatal("the inbox file is gone or changed")
	}
	if entries, _ := os.ReadDir(tv.V.Abs("source-core/documents")); len(entries) != 0 {
		t.Fatalf("a source stayed: %v", entries)
	}
	if entries, _ := os.ReadDir(tv.V.Abs("source-core/originals")); len(entries) != 0 {
		t.Fatalf("an asset stayed: %v", entries)
	}
	if out, err := exec.Command("git", "-C", tv.V.Root, "status", "--porcelain").CombinedOutput(); err != nil || strings.TrimSpace(string(out)) != "" {
		t.Fatalf("the tree is not clean:\n%s %v", out, err)
	}
}
