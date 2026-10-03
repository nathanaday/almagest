package core_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/core"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/thread"
)

func TestCloseMentionWritesOnlyANoteOfTheVault(t *testing.T) {
	tv := testvault.New(t)
	if _, err := thread.Stub(tv.V, thread.StubIn{Text: "x", Title: "Idea"}, thread.Opts{Now: testvault.Now}); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(filepath.Dir(tv.V.Root), "outside", "victim.md")
	tv.WriteFile(victim, "- [ ] @atlas track this\n")
	if err := os.Symlink(victim, tv.V.Abs("Linked.md")); err != nil {
		t.Fatal(err)
	}
	tv.Write("sessions/2026-10/2026-10-02 0900 aaaaaa.md", "- [ ] @atlas track this\n")
	for note, want := range map[string]string{
		"../outside/victim.md": "is no note of the vault",
		"Linked.md":            "is a link",
		"sessions/2026-10/2026-10-02 0900 aaaaaa.md": "whose files hold no mentions",
		"Nowhere.md": "is no note of the vault",
	} {
		_, err := core.CloseMention(tv.V, note, 1, "Idea")
		if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "vault status lists each open mention") {
			t.Errorf("%s: %v", note, err)
		}
	}
	if data, _ := os.ReadFile(victim); string(data) != "- [ ] @atlas track this\n" {
		t.Fatalf("the outside note changed: %q", data)
	}
	if st, err := os.Lstat(tv.V.Abs("Linked.md")); err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Fatal("the link was replaced")
	}
	tv.Write("Ideas.md", "- [ ] @atlas track this\n")
	if _, err := core.CloseMention(tv.V, "Ideas.md", 1, "Idea"); err != nil {
		t.Fatalf("a note of the vault: %v", err)
	}
}

func TestSyncWritesNoCanvasThroughALinkedChordsFolder(t *testing.T) {
	tv := testvault.New(t)
	if _, err := thread.ChordCreate(tv.V, thread.ChordIn{Title: "Plan C", Text: "Ship it.", Threads: []thread.ChordThreadIn{{Title: "First", Text: "Do the first part."}}}, thread.Opts{Now: testvault.Now}); err != nil {
		t.Fatal(err)
	}
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
	_, err := core.Sync(tv.V, testvault.Now.Add(time.Hour), core.SyncOptions{})
	if err == nil || !strings.Contains(err.Error(), "chords is a link that leads out of the vault; make chords a plain folder") {
		t.Errorf("sync through a linked chords/: %v", err)
	}
	if entries, _ := os.ReadDir(away); len(entries) != 0 {
		t.Fatalf("sync wrote outside the vault: %v", entries)
	}
}
