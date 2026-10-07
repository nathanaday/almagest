package vault

import (
	"os"
	"path/filepath"
	"testing"
)

// A save that lands while a guarded write syncs its temporary file is kept: the write
// checks the file again right before the rename.
func TestAGuardedWriteChecksAgainBeforeTheRename(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(file, []byte("as read\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	beforeRename = func(string) { os.WriteFile(file, []byte("saved meanwhile\n"), 0o644) }
	defer func() { beforeRename = nil }()
	unchanged := func() bool {
		now, _ := os.ReadFile(file)
		return string(now) == "as read\n"
	}
	wrote, err := writeAtomicIf(file, []byte("derived\n"), unchanged)
	if err != nil || wrote {
		t.Fatalf("wrote %v, err %v", wrote, err)
	}
	if got, _ := os.ReadFile(file); string(got) != "saved meanwhile\n" {
		t.Fatalf("the save was overwritten: %q", got)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".almagest-*")); len(left) != 0 {
		t.Fatalf("temporary files left: %v", left)
	}
}
