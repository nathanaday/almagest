package migrate_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/almagest/internal/change"
	"github.com/nathanaday/almagest/internal/checkout"
	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/lint"
	"github.com/nathanaday/almagest/internal/migrate"
	"github.com/nathanaday/almagest/internal/testvault"
	"github.com/nathanaday/almagest/internal/vault"
)

// The notes of a layout-7 vault that name a moved folder, and what the migration makes of them.
const (
	note = "See [[source-core/documents/Alpha|alpha]], the paper ![](source-core/originals/doc-aaaaaa.pdf), and [[sessions/2026-09/2026-09-27 0900 a1b2c3]].\n" +
		"The old \"source-core/documents\" stays in prose, and `[[source-core/documents/Alpha]]` in code.\n\n" +
		"```base\nfilters:\n  and:\n    - file.inFolder(\"source-core/documents\")\n```\n"
	noteAfter = "See [[tool/source-core/documents/Alpha|alpha]], the paper ![](tool/source-core/originals/doc-aaaaaa.pdf), and [[tool/sessions/2026-09/2026-09-27 0900 a1b2c3]].\n" +
		"The old \"source-core/documents\" stays in prose, and `[[source-core/documents/Alpha]]` in code.\n\n" +
		"```base\nfilters:\n  and:\n    - file.inFolder(\"tool/source-core/documents\")\n```\n"
	// An applied change records what it wrote inside a fence of five backticks; a fence
	// of three inside it does not close it, and its history stays as written.
	applied = "---\nid: chg-old001\ntype: change\ncreated: 2026-09-20T09:00:00\nupdated: 2026-09-20T09:00:00\nstatus: applied\n---\n\n## Writes\n\n### modify · Beta · doc-bbbbbb\n\n`````markdown\n" +
		"```almagest-change\n```\n\n## Knowledge\n\n```base\nfilters:\n  and:\n    - file.inFolder(\"source-core/documents\")\n```\n`````\n"
	session  = "---\nid: ses-a1b2c3\ntype: session\ncreated: 2026-09-27T09:00:00\nharness: claude\nharness_id: a1b2c3\nstatus: ended\nupdated: 2026-09-27T10:00:00\n---\n\n## Description\n\nWork.\n"
	trashed  = "Gone, with [[source-core/documents/Alpha]].\n"
	original = "An original that names [[source-core/documents/Alpha]].\n"
	waiting  = "A file to ingest, with [[source-core/documents/Alpha]].\n"
	canvas   = `{"nodes":[{"id":"a","type":"file","file":"source-core/documents/Alpha.md"}],"edges":[]}` + "\n"
	marks    = `{"items":[{"type":"file","path":"source-core/documents/Alpha.md"}]}` + "\n"
)

// seven is a vault as layout 7 left it: sessions/, source-core/, and trash/ at its root,
// with a link, a Base, a canvas, a bookmark, and Obsidian's settings that name them, and a
// change proposed on Alpha that waits for an answer.
func seven(t *testing.T) (*testvault.T, string) {
	t.Helper()
	tv := testvault.New(t)
	tv.Doc("topic", "Alpha", map[string]any{"kind": "concept"}, "## Definition\n\nAlpha.\n")
	tv.Doc("source", "Paper", map[string]any{"sha256": "abcdef0123456789", "file": "[[doc-aaaaaa.pdf]]"}, "## Summary\n\nA paper.\n")
	tv.Write(vault.Originals+"/doc-aaaaaa.pdf", "%PDF")
	tv.Write(vault.Originals+"/doc-bbbbbb.md", original)
	tv.Write(vault.Sessions+"/2026-09/2026-09-27 0900 a1b2c3.md", session)
	tv.Write(vault.Trash+"/2026-09-20/source-core/documents/Gone.md", trashed)
	tv.Commit()
	pv, err := change.Propose(tv.V, change.Plan{Title: "Edit Alpha", Writes: []change.Write{{Op: "modify", ID: "Alpha", Body: str("## Definition\n\nAlpha, edited.\n")}}}, tv.Tick(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{vault.Sessions, vault.Core, vault.Trash} {
		if err := os.Rename(tv.V.Abs(dir), tv.V.Abs(strings.TrimPrefix(dir, vault.Tool+"/"))); err != nil {
			t.Fatal(err)
		}
	}
	os.Remove(tv.V.Abs(vault.Tool))
	tv.Write("Almagest.md", strings.Replace(tv.Read("Almagest.md"), "\nlayout: 8\n", "\nlayout: 7\n", 1))
	tv.Write("sessions/Sessions.base", strings.Replace(tv.Read("sessions/Sessions.base"), `file.inFolder("tool/sessions")`, `file.inFolder("sessions")`, 1))
	tv.Write(vault.AppJSON, "{\n  \"attachmentFolderPath\": \"source-core/originals\",\n  \"userIgnoreFilters\": [\n    \"wiki-view/\",\n    \"trash/\"\n  ],\n  \"alwaysUpdateLinks\": true\n}\n")
	tv.Write(".obsidian/bookmarks.json", marks)
	tv.Write("Notes.md", note)
	tv.Write("Map.canvas", canvas)
	tv.Write("changes/2026-09/2026-09-20 Edit Beta.md", applied)
	tv.Write("ingest/waiting.md", waiting)
	tv.Commit()
	var err2 error
	if tv.V, err2 = vault.Open(tv.V.Root); err2 != nil {
		t.Fatal(err2)
	}
	return tv, pv.Ref.ID
}

func str(s string) *string { return &s }

func TestMigrateMovesTheToolFoldersAndFollowsTheirPaths(t *testing.T) {
	tv, pending := seven(t)
	r, err := migrate.Run(tv.V, tv.Tick(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if r.From != 7 || r.To != 8 || r.Commit == "" || len(r.Moved) != 7 {
		t.Fatalf("report %+v", r)
	}
	tv.Clean()
	for _, dir := range []string{"sessions", "source-core", "trash"} {
		if _, err := os.Stat(tv.V.Abs(dir)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s/ stays: %v", dir, err)
		}
	}
	want := map[string]string{
		"Notes.md":                 noteAfter,
		"Map.canvas":               strings.Replace(canvas, `"source-core/`, `"tool/source-core/`, 1),
		".obsidian/bookmarks.json": strings.Replace(marks, `"source-core/`, `"tool/source-core/`, 1),
		"changes/2026-09/2026-09-20 Edit Beta.md":                 applied,
		vault.Trash + "/2026-09-20/source-core/documents/Gone.md": trashed,
		vault.Originals + "/doc-bbbbbb.md":                        original,
		vault.Sessions + "/2026-09/2026-09-27 0900 a1b2c3.md":     session,
		"ingest/waiting.md":                                       waiting,
	}
	for rel, text := range want {
		if got := tv.Read(rel); got != text {
			t.Errorf("%s:\n%s\nwant:\n%s", rel, got, text)
		}
	}
	if got := tv.Read(vault.Sessions + "/Sessions.base"); !strings.Contains(got, `file.inFolder("tool/sessions")`) {
		t.Errorf("the Sessions Base:\n%s", got)
	}
	if got := tv.Read(vault.AppJSON); !strings.Contains(got, `"attachmentFolderPath": "tool/source-core/originals"`) || !strings.Contains(got, `"tool/trash/"`) || strings.Contains(got, `"trash/"`) || !strings.Contains(got, `"alwaysUpdateLinks": true`) {
		t.Errorf("app.json:\n%s", got)
	}
	slices.Sort(r.Edited)
	if wantEdited := []string{".obsidian/app.json", ".obsidian/bookmarks.json", "Almagest.md", "Map.canvas", "Notes.md", vault.Sessions + "/Sessions.base"}; !slices.Equal(r.Edited, wantEdited) {
		t.Errorf("edited %v, want %v", r.Edited, wantEdited)
	}
	if log := tv.Log(); log[0] != "layout: move sessions/, source-core/, and trash/ into tool/" {
		t.Errorf("log %v", log)
	}
	if sha, err := tv.V.Git().FindTrailer(migrate.Trailer, "7 to 8"); err != nil || sha == "" || !strings.HasPrefix(sha, r.Commit[:7]) {
		t.Errorf("the commit's trailer: %q %v", sha, err)
	}
	v, err := vault.Open(tv.V.Root)
	if err != nil || v.CheckLayout() != nil {
		t.Fatalf("the layout after: %v %v", err, v.CheckLayout())
	}
	tv.V = v
	if !v.Exists("wiki-view/View · Home.md") {
		t.Error("no views after the migration")
	}
	if f, err := lint.Run(tv.Index(), lint.Options{Quick: true, Now: tv.Clock}); err != nil || f.Counts[lint.Error] != 0 || r.Problems != 0 {
		t.Errorf("lint after: %+v %v, report %d", f.Findings, err, r.Problems)
	}
	if !slices.ContainsFunc(r.Warnings, func(w string) bool { return strings.Contains(w, "undo") }) {
		t.Errorf("warnings %v", r.Warnings)
	}
	// The change that waited applies to the moved document.
	if _, err := change.Apply(tv.V, pending, tv.Tick(time.Minute), nil); err != nil {
		t.Fatalf("the pending change: %v", err)
	}
	if got := tv.Read(vault.DocPath("Alpha")); !strings.Contains(got, "Alpha, edited.") {
		t.Errorf("Alpha:\n%s", got)
	}
}

func TestADryRunWritesNothing(t *testing.T) {
	tv, _ := seven(t)
	r, err := migrate.Plan(tv.V)
	if err != nil || r.Commit != "" || len(r.Moved) != 7 || len(r.Edited) != 6 {
		t.Fatalf("plan %+v %v", r, err)
	}
	tv.Clean()
	if _, err := os.Stat(tv.V.Abs(vault.Tool)); !errors.Is(err, os.ErrNotExist) || tv.Read("Notes.md") != note {
		t.Fatalf("the dry run wrote: %v", err)
	}
}

func TestMigrateRefusesAVaultItDoesNotTake(t *testing.T) {
	tv := testvault.New(t)
	if _, err := migrate.Run(tv.V, tv.Clock); err == nil || !strings.Contains(err.Error(), "nothing to migrate") {
		t.Fatalf("a current vault: %v", err)
	}
	tv.Write("Almagest.md", strings.Replace(tv.Read("Almagest.md"), "\nlayout: 8\n", "\nlayout: 9\n", 1))
	v, _ := vault.Open(tv.V.Root)
	if _, err := migrate.Run(v, tv.Clock); !errors.Is(err, vault.ErrLayout) {
		t.Fatalf("a newer vault: %v", err)
	}
}

func TestATakenPathRefusesTheMigration(t *testing.T) {
	tv, _ := seven(t)
	tv.Write(vault.Sessions+"/2026-09/2026-09-27 0900 a1b2c3.md", "Mine.\n")
	tv.Commit()
	if _, err := migrate.Run(tv.V, tv.Clock); err == nil || !strings.Contains(err.Error(), "which exists") {
		t.Fatalf("a taken path: %v", err)
	}
	tv.Clean()
	if tv.Read("sessions/2026-09/2026-09-27 0900 a1b2c3.md") != session || !strings.Contains(tv.Read("Almagest.md"), "\nlayout: 7\n") {
		t.Fatal("the refused migration moved something")
	}
}

func TestAFailedCommitPutsTheVaultBack(t *testing.T) {
	tv, _ := seven(t)
	lock := filepath.Join(tv.V.Root, ".git", "index.lock")
	migrate.SetBeforeCommit(func() { os.WriteFile(lock, nil, 0o644) })
	t.Cleanup(func() { migrate.SetBeforeCommit(nil) })
	if _, err := migrate.Run(tv.V, tv.Clock); err == nil {
		t.Fatal("the migration committed through a held index lock")
	}
	os.Remove(lock)
	tv.Clean()
	if tv.Read("Notes.md") != note || tv.Read("sessions/2026-09/2026-09-27 0900 a1b2c3.md") != session || !strings.Contains(tv.Read("Almagest.md"), "\nlayout: 7\n") {
		t.Fatal("the failed migration left the vault changed")
	}
	if tv.V.Exists(vault.DocPath("Alpha")) {
		t.Fatal("a moved file stays in tool/")
	}
}

func TestTheUsersFilesInToolStay(t *testing.T) {
	tv, _ := seven(t)
	tv.Write("tool/Mine.md", "My own tool notes.\n")
	tv.Commit()
	r, err := migrate.Run(tv.V, tv.Clock)
	if err != nil {
		t.Fatal(err)
	}
	if tv.Read("tool/Mine.md") != "My own tool notes.\n" || !slices.ContainsFunc(r.Warnings, func(w string) bool { return strings.Contains(w, "1 file of yours") }) {
		t.Fatalf("warnings %v", r.Warnings)
	}
}

// The checkouts of 11.0: each reading list becomes the checkout's index, a returned
// checkout moves to tool/returned/ with the links that name it, and the ledger becomes
// its Base.
func TestMigrateBringsTheCheckouts(t *testing.T) {
	tv, _ := seven(t)
	reading := func(name, returned string) string {
		return "---\nrequest: \"Check out " + name + "\"\nchecked_out: 2026-09-20T10:00:00\ndocuments: 2\nreturned: " + returned + "\n---\n\n> [!almagest] Checked out 2026-09-20\n\n## Reading order\n\n1. [[checkout/2026-09-20 " + name + "/Alpha (checkout)|Alpha]]\n"
	}
	tv.Write("checkout/2026-09-20 Out/Checkout · 2026-09-20 Out.md", reading("Out", `""`))
	tv.Write("checkout/2026-09-20 Out/Alpha (checkout).md", "---\ncheckout_id: doc-a\n---\n\nAlpha.\n")
	tv.Write("checkout/2026-09-20 Back/Checkout · 2026-09-20 Back.md", reading("Back", "2026-09-21T10:00:00"))
	tv.Write("checkout/2026-09-20 Back/Alpha (checkout).md", "---\ncheckout_id: doc-a\n---\n\nAlpha, with [[checkout/2026-09-20 Back/Beta (checkout)|Beta]] and my edit.\n")
	tv.Write("checkout/2026-09-20 Back/Beta (checkout).md", "Beta.\n")
	tv.Write("checkout/Checkout · Ledger.md", "| Checked out | Request |\n|---|---|\n| 2026-09-20 | [[checkout/2026-09-20 Back/Checkout · 2026-09-20 Back\\|Back]] |\n")
	tv.Write("scratchpad/Reading.md", "Back is in [[checkout/2026-09-20 Back/Checkout · 2026-09-20 Back|the reading list]].\n")
	tv.Commit()
	if _, err := migrate.Run(tv.V, tv.Tick(time.Minute)); err != nil {
		t.Fatal(err)
	}
	tv.Clean()
	out := tv.Read("checkout/2026-09-20 Out/_index.md")
	if fm := doc.Parse("", []byte(out)); fm.Str("name") != "Out" || fm.Str("status") != "out" || !strings.Contains(out, "1. [[checkout/2026-09-20 Out/Alpha (checkout)|Alpha]]") {
		t.Fatalf("the index of a checkout that is out:\n%s", out)
	}
	if tv.V.Exists("checkout/2026-09-20 Back") || tv.V.Exists("checkout/2026-09-20 Out/Checkout · 2026-09-20 Out.md") {
		t.Fatal("a reading list or a returned checkout stays")
	}
	back := tv.Read("tool/returned/2026-09-20 Back/_index.md")
	if fm := doc.Parse("", []byte(back)); fm.Str("name") != "Back" || fm.Str("status") != "returned" || !strings.Contains(back, "1. [[tool/returned/2026-09-20 Back/Alpha (checkout)|Alpha]]") {
		t.Fatalf("the index of a returned checkout:\n%s", back)
	}
	if got := tv.Read("tool/returned/2026-09-20 Back/Alpha (checkout).md"); !strings.Contains(got, "[[tool/returned/2026-09-20 Back/Beta (checkout)|Beta]] and my edit.") {
		t.Fatalf("a returned copy:\n%s", got)
	}
	if got := tv.Read("scratchpad/Reading.md"); got != "Back is in [[tool/returned/2026-09-20 Back/_index|the reading list]].\n" {
		t.Fatalf("a note's link to the reading list:\n%s", got)
	}
	if tv.Read("checkout/Checkout · Ledger.md") != checkout.LedgerNote {
		t.Fatalf("the ledger:\n%s", tv.Read("checkout/Checkout · Ledger.md"))
	}
	if l := checkout.List(tv.V); len(l) != 2 || l[0].Status == l[1].Status {
		t.Fatalf("the checkouts after the migration: %+v", l)
	}
}
