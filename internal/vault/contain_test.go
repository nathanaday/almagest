package vault_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// outside makes a folder beside the vault, in the test's temporary folder, with one file.
func outside(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secret.md"), []byte("secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func link(t *testing.T, target, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, name); err != nil {
		t.Fatal(err)
	}
}

func TestContainRefusesEveryPathThatLeavesTheVault(t *testing.T) {
	tv := testvault.New(t)
	v := tv.V
	out := outside(t)
	link(t, out, v.Abs("inbox/away"))
	link(t, filepath.Join(out, "secret.md"), v.Abs("inbox/secret.md"))
	link(t, v.Abs("wiki/documents"), v.Abs("scratchpad/docs"))
	tv.Write("wiki/documents/Alpha.md", "alpha\n")
	link(t, v.Abs("wiki/documents/Alpha.md"), v.Abs("scratchpad/alpha.md"))

	for _, rel := range []string{
		"",
		"/etc/passwd",
		"../outside/secret.md",
		"wiki/documents/../../../outside/secret.md",
		"wiki//documents/Alpha.md",
		"./wiki/documents/Alpha.md",
		`wiki\documents\Alpha.md`,
		".git/config",
		".GIT/x.md",
		".Git/hooks/pre-commit",
		"inbox/away/secret.md",
		"inbox/away/new/deeper.md",
		"inbox/secret.md",
		"wiki/documents/" + strings.Repeat("a", 253) + ".md",
	} {
		err := v.Contain(rel)
		if !errors.Is(err, vault.ErrOutside) {
			t.Errorf("Contain(%q) = %v, want a refusal", rel, err)
			continue
		}
		if !strings.Contains(err.Error(), "give a clean vault-relative path") {
			t.Errorf("Contain(%q) does not teach: %v", rel, err)
		}
		if strings.HasSuffix(rel, ".md") && v.Local(rel) {
			t.Errorf("Local(%q) is true", rel)
		}
	}
	for _, rel := range []string{
		"wiki/documents/Alpha.md",
		"wiki/documents/New.md",
		"wiki/documents/new/folder/Deep.md",
		"scratchpad/docs/Alpha.md",
		"scratchpad/alpha.md",
		".obsidian/plugins/atlas/data.json",
	} {
		if err := v.Contain(rel); err != nil {
			t.Errorf("Contain(%q) = %v, want nil", rel, err)
		}
	}
	if !v.Local("scratchpad/docs/Alpha.md") || !v.Local("wiki/documents/Alpha.md") {
		t.Error("Local refuses a document inside the vault")
	}
	for _, rel := range []string{".obsidian/x.md", ".Obsidian/x.md", ".CLAUDE/x.md", ".claude/x.md", "wiki/documents/Alpha.txt"} {
		if v.Local(rel) {
			t.Errorf("Local(%q) is true", rel)
		}
	}
	if data, err := os.ReadFile(filepath.Join(out, "secret.md")); err != nil || string(data) != "secret\n" {
		t.Fatalf("the outside file changed: %q %v", data, err)
	}
}

func TestAFailedRenameLeavesNoTemporaryFile(t *testing.T) {
	tv := testvault.New(t)
	v := tv.V
	tv.Write("wiki/documents/Busy/inner.md", "inner\n")
	if err := v.Write("wiki/documents/Busy", []byte("a file over a folder\n")); err == nil {
		t.Fatal("the write over a folder succeeded")
	}
	left, _ := filepath.Glob(v.Abs("wiki/documents/.atlas-*"))
	if len(left) != 0 {
		t.Fatalf("temporary files left: %v", left)
	}
	if !strings.Contains(strings.Join(vault.Excluded, " "), ".atlas-*") {
		t.Fatalf("Excluded holds no .atlas-*: %v", vault.Excluded)
	}
}
