package core_test

import (
	"strings"
	"testing"

	"github.com/nathanaday/almagest/internal/core"
	"github.com/nathanaday/almagest/internal/derive"
	"github.com/nathanaday/almagest/internal/testvault"
	"github.com/nathanaday/almagest/internal/vault"
)

func TestTheViewsSyncMovesAStrayNoteToIngest(t *testing.T) {
	tv := testvault.New(t)
	note := "# Meeting notes\n\nWritten while View · Home was open.\n"
	tv.Write("wiki-view/Meeting notes.md", note)
	tv.Write("ingest/Meeting notes.md", "an older note of that name\n")
	stale := "wiki-view/nav/gone/Tag · gone.md"
	tv.Write(stale, "> [!view] Written by Almagest from the documents. Edits here are lost at the next sync.\n")
	synced, err := core.Sync(tv.V, testvault.Now, core.SyncOptions{Views: true})
	if err != nil {
		t.Fatal(err)
	}
	if tv.V.Exists("wiki-view/Meeting notes.md") {
		t.Fatal("the stray note stayed in wiki-view/")
	}
	if got := tv.Read("ingest/Meeting notes (2).md"); got != note {
		t.Fatalf("the stray note in the inbox: %q", got)
	}
	if tv.Read("ingest/Meeting notes.md") != "an older note of that name\n" {
		t.Fatal("the move overwrote a note in the inbox")
	}
	if len(synced.Strays) != 1 || synced.Strays[0].To != "ingest/Meeting notes (2).md" || synced.Strays[0].From != "wiki-view/Meeting notes.md" {
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
