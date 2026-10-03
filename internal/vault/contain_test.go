package vault_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/core"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/thread"
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
		teach := "give a clean vault-relative path"
		if strings.HasPrefix(rel, "inbox/") {
			// A link: the fix is to the link, which the refusal names.
			teach = "a link that leads out of the vault; make inbox/"
		}
		if !strings.Contains(err.Error(), teach) {
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

func TestEveryWriteOfATransactionStaysInTheVault(t *testing.T) {
	tv := testvault.New(t)
	v := tv.V
	out := outside(t)
	link(t, out, v.Abs("wiki/documents/out"))
	tv.Write("wiki/documents/A.md", "a\n")
	tx, err := vault.BeginWrite(v)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Close()
	if err := tx.Write("wiki/documents/out/x.md", []byte("x\n")); !errors.Is(err, vault.ErrOutside) {
		t.Errorf("Write: %v", err)
	}
	if _, err := tx.WriteIfChanged("wiki/documents/out/x.md", []byte("x\n")); !errors.Is(err, vault.ErrOutside) {
		t.Errorf("WriteIfChanged: %v", err)
	}
	if err := tx.Remove("wiki/documents/out/secret.md"); !errors.Is(err, vault.ErrOutside) {
		t.Errorf("Remove: %v", err)
	}
	if err := tx.Move("wiki/documents/A.md", "wiki/documents/out/A.md"); !errors.Is(err, vault.ErrOutside) {
		t.Errorf("Move: %v", err)
	}
	if err := tx.Write("../escape.md", []byte("x\n")); !errors.Is(err, vault.ErrOutside) {
		t.Errorf("Write ..: %v", err)
	}
	if len(tx.Paths()) != 0 {
		t.Errorf("a refused write was marked: %v", tx.Paths())
	}
	entries, _ := os.ReadDir(out)
	if len(entries) != 1 || entries[0].Name() != "secret.md" {
		t.Fatalf("the outside folder changed: %v", entries)
	}
	if !v.Exists("wiki/documents/A.md") {
		t.Fatal("the refused move took the file")
	}
	if err := tx.Write("wiki/documents/B.md", []byte("b\n")); err != nil {
		t.Fatalf("a write inside the vault: %v", err)
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

func TestVaultWriteRefusesAPathThroughALinkOut(t *testing.T) {
	tv := testvault.New(t)
	out := outside(t)
	link(t, out, tv.V.Abs("sessions/2026-10"))
	if err := tv.V.Write("sessions/2026-10/x.md", []byte("x\n")); !errors.Is(err, vault.ErrOutside) {
		t.Errorf("Write: %v", err)
	}
	if _, err := tv.V.WriteIfChanged("sessions/2026-10/x.md", []byte("x\n")); !errors.Is(err, vault.ErrOutside) {
		t.Errorf("WriteIfChanged: %v", err)
	}
	if entries, _ := os.ReadDir(out); len(entries) != 1 {
		t.Fatalf("the outside folder changed: %v", entries)
	}
}

func TestThePluginInstallsThroughALinkedObsidianFolder(t *testing.T) {
	tv := testvault.New(t)
	shared := filepath.Join(t.TempDir(), "shared-obsidian")
	if err := os.Rename(tv.V.Abs(".obsidian"), shared); err != nil {
		t.Fatal(err)
	}
	link(t, shared, tv.V.Abs(".obsidian"))
	os.RemoveAll(filepath.Join(shared, "plugins"))
	if _, err := vault.InstallPlugin(tv.V); err != nil {
		t.Fatalf("install through a linked .obsidian: %v", err)
	}
	if _, err := os.Stat(filepath.Join(shared, "plugins", "atlas", "manifest.json")); err != nil {
		t.Fatal("the plugin is not in the shared folder")
	}
	if _, err := tv.V.WriteMachineIfChanged("wiki/documents/x.md", []byte("x\n")); err == nil {
		t.Fatal("the machine writer took a document path")
	}
}

// A file the user saves while a write that will fail is open keeps the save: rollback
// puts back only what still holds the write's own bytes.
func TestRollbackLeavesASaveMadeDuringTheWrite(t *testing.T) {
	tv := testvault.New(t)
	tv.Write("wiki/documents/A.md", "before\n")
	tv.Write("wiki/documents/B.md", "before\n")
	tv.Commit()
	tx, err := vault.BeginWrite(tv.V)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Write("wiki/documents/A.md", []byte("written by the call\n")); err != nil {
		t.Fatal(err)
	}
	if err := tx.Write("wiki/documents/B.md", []byte("written by the call\n")); err != nil {
		t.Fatal(err)
	}
	tv.Write("wiki/documents/A.md", "saved in Obsidian\n")
	lock := filepath.Join(tv.V.Root, ".git", "index.lock")
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Commit("a write that fails")
	os.Remove(lock)
	tx.Close()
	if err == nil || !strings.Contains(err.Error(), "left as saved") || !strings.Contains(err.Error(), "wiki/documents/A.md") {
		t.Fatalf("the failure message: %v", err)
	}
	if got := tv.Read("wiki/documents/A.md"); got != "saved in Obsidian\n" {
		t.Fatalf("the save was rolled back: %q", got)
	}
	if got := tv.Read("wiki/documents/B.md"); got != "before\n" {
		t.Fatalf("B was not put back: %q", got)
	}
}

// The harness writes .claude/settings.local.json too. A change that lands while Atlas
// merges the file makes the merge run again, so both writes keep their keys.
func TestSyncSettingsMergesAgainWhenTheFileChanges(t *testing.T) {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", nil)
	tv.Doc("repository", "p3-edge", map[string]any{"path": repo}, "")
	tv.Commit()
	file := tv.V.Abs(".claude/settings.local.json")
	tv.WriteFile(file, "{\n  \"mine\": 1\n}\n")
	once := false
	vault.SetBeforeRename(func(f string) {
		if f == file && !once {
			once = true
			os.WriteFile(file, []byte("{\n  \"mine\": 1,\n  \"theirs\": 2\n}\n"), 0o644)
		}
	})
	defer vault.SetBeforeRename(nil)
	wrote, err := tv.V.SyncSettings(nil)
	if err != nil || !wrote {
		t.Fatalf("sync settings: %v %v", wrote, err)
	}
	got, _ := os.ReadFile(file)
	for _, want := range []string{`"mine": 1`, `"theirs": 2`, repo} {
		if !strings.Contains(string(got), want) {
			t.Errorf("the settings lack %s:\n%s", want, got)
		}
	}
}

// A canvas the user saves while a sync writes it keeps the save.
func TestASyncKeepsACanvasSavedDuringItsWrite(t *testing.T) {
	tv := testvault.New(t)
	if _, err := thread.ChordCreate(tv.V, thread.ChordIn{Title: "Plan C", Text: "Ship it.", Threads: []thread.ChordThreadIn{{Title: "First", Text: "Do the first part."}, {Title: "Second", Text: "Then this.", After: []string{"First"}}}}, thread.Opts{Now: testvault.Now}); err != nil {
		t.Fatal(err)
	}
	canvas := tv.V.Abs("chords/Plan C.canvas")
	// Cards painted the wrong color: the sync paints them again, so it writes the canvas.
	data, _ := os.ReadFile(canvas)
	var c map[string]any
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatal(err)
	}
	for _, n := range c["nodes"].([]any) {
		n.(map[string]any)["color"] = "#123456"
	}
	moved, _ := json.Marshal(c)
	os.WriteFile(canvas, moved, 0o644)
	tv.Commit()
	saved := []byte(strings.Replace(string(moved), "#123456", "#654321", 1))
	vault.SetBeforeRename(func(f string) {
		if f == canvas {
			os.WriteFile(canvas, saved, 0o644)
		}
	})
	defer vault.SetBeforeRename(nil)
	synced, err := core.Sync(tv.V, testvault.Now, core.SyncOptions{Views: true})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(canvas); string(got) != string(saved) {
		t.Fatalf("the canvas save was overwritten (%v, skipped %v):\n%s", err, synced.Skipped, got)
	}
	if !strings.Contains(strings.Join(synced.Skipped, "|"), "chords/Plan C.canvas") {
		t.Fatalf("skipped %v", synced.Skipped)
	}
}
