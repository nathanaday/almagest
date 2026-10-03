package core_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/core"
	"github.com/nathanaday/atlas-obsidian/internal/derive"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/thread"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
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

func TestTheViewsSyncMovesAStrayNoteToTheInbox(t *testing.T) {
	tv := testvault.New(t)
	note := "# Meeting notes\n\nWritten while View · Home was open.\n"
	tv.Write("views/Meeting notes.md", note)
	tv.Write("inbox/Meeting notes.md", "an older note of that name\n")
	stale := "views/tags/gone/Tag · gone.md"
	tv.Write(stale, "> [!view] Written by Atlas from the documents. Edits here are lost at the next sync.\n")
	synced, err := core.Sync(tv.V, testvault.Now, core.SyncOptions{Views: true})
	if err != nil {
		t.Fatal(err)
	}
	if tv.V.Exists("views/Meeting notes.md") {
		t.Fatal("the stray note stayed in views/")
	}
	if got := tv.Read("inbox/Meeting notes (2).md"); got != note {
		t.Fatalf("the stray note in the inbox: %q", got)
	}
	if tv.Read("inbox/Meeting notes.md") != "an older note of that name\n" {
		t.Fatal("the move overwrote a note in the inbox")
	}
	if len(synced.Strays) != 1 || synced.Strays[0].To != "inbox/Meeting notes (2).md" || synced.Strays[0].From != "views/Meeting notes.md" {
		t.Fatalf("strays %v", synced.Strays)
	}
	if tv.V.Exists(stale) {
		t.Fatal("a stale view stayed")
	}
}

// A save that lands after a sync read a document and before its derived write keeps the
// save; the sync names it, and the next sync derives it with the save in place.
func TestADerivedWriteKeepsASaveMadeAfterTheRead(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("source", "Paper", map[string]any{"sha256": "abcdef0123456789", "file": "[[x.pdf]]", "media": "pdf"}, "## Summary\n\nFirst.\n")
	tv.Commit()
	idx := tv.Index()
	path := idx.ByID(tv.ID("Paper")).Path
	saved := tv.Read(path) + "\nA line saved in Obsidian.\n"
	tv.Write(path, saved)
	guard := vault.NewGuard(idx, tv.V)
	if _, err := derive.Sync(idx, guard.Write); err != nil {
		t.Fatal(err)
	}
	skipped := guard.Skipped
	if tv.Read(path) != saved {
		t.Fatal("the derived write overwrote the save")
	}
	if len(skipped) != 1 || skipped[0] != path {
		t.Fatalf("skipped %v", skipped)
	}
	if _, err := core.Sync(tv.V, testvault.Now, core.SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := tv.Read(path); !strings.Contains(got, "A line saved in Obsidian.") || !strings.Contains(got, "> [!source]") {
		t.Fatalf("the next sync did not derive the document with the save:\n%s", got)
	}
}
