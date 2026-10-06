package migrate_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/core"
	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/lint"
	"github.com/nathanaday/atlas-obsidian/internal/migrate"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

const atlas8 = "---\nid: vlt-aaaaaa\ntype: vault\nname: Work\ndescription: Work notes.\ncreated: 2026-09-01T00:00:00\nupdated: 2026-09-01T00:00:00\ntagging: open\nwikify: [source, spec, verification, chord, event]\nstale_hours: 12\nlayout: 4\n---\nThe vault's own context.\n"

const canvas8 = "{\"nodes\":[{\"id\":\"a\",\"type\":\"file\",\"file\":\"wiki/documents/Filter alarms.md\",\"x\":0,\"y\":0,\"width\":400,\"height\":120}],\"edges\":[]}\n"

const threadsBase = "## Threads\n\n```base\nfilters:\n  and:\n    - 'type == \"spec\"'\n```\n"

// eight writes a vault as 8.x left it: a thread with its spec, tasks, verification, and
// events, a chord with its canvas, and the thread fields on the documents that stay.
func eight(t *testing.T) *testvault.T {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", nil)
	tv.Write("Atlas.md", atlas8)
	tv.Doc("repository", "p3-edge", map[string]any{"path": repo, "defines": "work/p3-edge"}, "## What it is\n\nThe edge.\n\n"+threadsBase)
	tv.Doc("source", "DINOv2", map[string]any{"file": "[[doc-aaaaaa.pdf]]", "sha256": "3f9c1e2a7b8d44aa", "from": "[[Filter alarms]]"}, "## Summary\n\nFeatures.\n")
	tv.Write("wiki/assets/doc-aaaaaa.pdf", "%PDF")
	tv.Doc("topic", "Motion scoring", map[string]any{"kind": "concept", "sources": []string{"[[DINOv2]]"}, "from": "[[Filter alarms]]"}, "## Definition\n\nMotion, per [[Filter alarms · Spec]].\n\n"+threadsBase+"\n## Origin\n\nScore boxes by motion.\n")
	tv.Doc("stub", "Filter alarms", map[string]any{"status": "verified", "chord": "[[Quiet edge]]", "spec": "[[Filter alarms · Spec]]"}, "## Idea\n\nCut the false alarms.\n")
	tv.Doc("spec", "Filter alarms · Spec", map[string]any{"thread": "[[Filter alarms]]"}, "## Goal\n\nFewer alarms.\n\n## Requirements\n\n- R1: half as many alarms\n")
	tv.Doc("tasks", "Filter alarms · Tasks (p3-edge)", map[string]any{"thread": "[[Filter alarms]]", "repository": "[[p3-edge]]"}, "## Tasks\n\n- [x] T1: score boxes (R1)\n- [ ] T2: tune (R1)\n")
	tv.Doc("verification", "Filter alarms · Verification 1", map[string]any{"thread": "[[Filter alarms]]", "round": 1, "verdict": "pass"}, "## Scope\n\nAll.\n")
	tv.Doc("event", "Filter alarms · started 2026-09-04 1000", map[string]any{"kind": "started", "at": "2026-09-04T10:00:00", "subject": "[[Filter alarms]]"}, "")
	tv.Doc("chord", "Quiet edge", map[string]any{"threads": "0/1"}, "## Goal\n\nA quiet edge.\n\n## Threads\n\n![[chords/Quiet edge.canvas|the order as a graph]]\n")
	tv.Write("chords/Quiet edge.canvas", canvas8)
	tv.Write("chords/old/Loud edge.canvas", "{\"nodes\":[],\"edges\":[]}\n")
	tv.Write("chords/Chord notes.md", "My notes on the chords.\n")
	sessions8, _ := vault.OldTemplate("8.1/Sessions.base")
	tv.Write("sessions/Sessions.base", sessions8)
	tv.Write("sessions/2026-09/2026-09-08 0900 bbbbbb.md", "---\nid: ses-bbbbbb\ntype: session\ncreated: 2026-09-08T09:00:00\nupdated: 2026-09-08T10:00:00\nharness: claude\nharness_id: bbbbbb\nstatus: ended\nthreads: [\"[[Filter alarms]]\"]\nspecs: []\nwork: [\"[[Filter alarms · Spec]]\"]\nrepositories: [\"[[p3-edge]]\"]\nchanges: []\nevents: 1\nchecked: 1\n---\n\n> [!session] ended · [[Filter alarms]] · [[p3-edge]]\n\n## Description\n\nScoring.\n")
	tv.Write("changes/2026-09/2026-09-10 Learn from the spec.md", "---\nid: chg-learn1\ntype: change\ncreated: 2026-09-10T12:00:00\nupdated: 2026-09-10T12:00:00\nstatus: applied\nabsorbs: [\"[[DINOv2]]\"]\nwork: \"[[Filter alarms]]\"\napplied: 2026-09-10T12:00:00\ncounts: 0 create, 1 modify\n---\n\n## Notes\n\nx\n")
	tv.Write("Ideas.md", "Next: [[Filter alarms · Tasks (p3-edge)]], per [[Filter alarms · Spec|the spec]].\n")
	return older(t, tv)
}

// older puts the files that testvault wrote in the 10.0 folders into the folders of 9.0
// and 8.x, commits the tree, and opens the vault again.
func older(t *testing.T, tv *testvault.T) *testvault.T {
	t.Helper()
	for _, m := range [][2]string{{vault.Documents, "wiki/documents"}, {vault.Originals, "wiki/assets"}, {vault.Ingest, "inbox"}, {vault.WikiView, "views"}} {
		from, to := tv.V.Abs(m[0]), tv.V.Abs(m[1])
		if err := os.MkdirAll(to, 0o755); err != nil {
			t.Fatal(err)
		}
		entries, _ := os.ReadDir(from)
		for _, e := range entries {
			if err := os.Rename(filepath.Join(from, e.Name()), filepath.Join(to, e.Name())); err != nil {
				t.Fatal(err)
			}
		}
		os.RemoveAll(from)
	}
	os.RemoveAll(tv.V.Abs(vault.Core))
	os.RemoveAll(tv.V.Abs(vault.Journals))
	if err := tv.V.Git().Unexclude("/wiki-view/"); err != nil {
		t.Fatal(err)
	}
	if err := tv.V.Git().Exclude("/views/"); err != nil {
		t.Fatal(err)
	}
	tv.Commit()
	v, err := vault.Open(tv.V.Root)
	if err != nil {
		t.Fatal(err)
	}
	tv.V = v
	return tv
}

// moved maps each file the migration moves to its place in threads/.
var moved = map[string]string{
	"wiki/documents/Filter alarms.md":                           "threads/Filter alarms.md",
	"wiki/documents/Filter alarms · Spec.md":                    "threads/Filter alarms · Spec.md",
	"wiki/documents/Filter alarms · Tasks (p3-edge).md":         "threads/Filter alarms · Tasks (p3-edge).md",
	"wiki/documents/Filter alarms · Verification 1.md":          "threads/Filter alarms · Verification 1.md",
	"wiki/documents/Filter alarms · started 2026-09-04 1000.md": "threads/Filter alarms · started 2026-09-04 1000.md",
	"wiki/documents/Quiet edge.md":                              "threads/Quiet edge.md",
	"chords/Quiet edge.canvas":                                  "threads/Quiet edge.canvas",
	"chords/old/Loud edge.canvas":                               "threads/old/Loud edge.canvas",
}

func TestMigration(t *testing.T) {
	tv := eight(t)
	if _, err := core.Sync(tv.V, testvault.Now, core.SyncOptions{}); err == nil || !strings.Contains(err.Error(), "vault migrate") {
		t.Fatalf("a write refuses an 8.x vault: %v", err)
	}
	before := map[string]string{}
	for from := range moved {
		before[from] = tv.Read(from)
	}
	plan, err := migrate.Plan(tv.V)
	if err != nil {
		t.Fatal(err)
	}
	tv.Clean()
	if plan.From != "8.x" || len(plan.Moved) != len(moved) || !strings.Contains(strings.Join(plan.Warnings, " "), "chords/Chord notes.md") {
		t.Fatalf("the plan: %+v", plan)
	}
	for _, p := range []string{"Atlas.md", "wiki/documents/Motion scoring.md", "wiki/documents/p3-edge.md", "wiki/documents/DINOv2.md", "sessions/2026-09/2026-09-08 0900 bbbbbb.md", "changes/2026-09/2026-09-10 Learn from the spec.md"} {
		if !slices.Contains(plan.Edited, p) {
			t.Errorf("the plan does not edit %s: %v", p, plan.Edited)
		}
	}

	report, err := migrate.Run(tv.V, testvault.Now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if report.Commit == "" || report.Problems != 0 {
		t.Fatalf("the report: %+v", report)
	}
	tv.Clean()
	if log := tv.Log(); log[0] != "layout: migrate to 10.0" || slices.ContainsFunc(log, func(s string) bool { return strings.HasPrefix(s, "layout: upgrade") }) {
		t.Fatalf("the commits: %v", log)
	}
	// The canvas card and the chord's embed follow the moves; every other file moves as it was.
	follow := map[string]string{
		"chords/Quiet edge.canvas":     strings.Replace(canvas8, "wiki/documents/Filter alarms.md", "threads/Filter alarms.md", 1),
		"wiki/documents/Quiet edge.md": strings.Replace(before["wiki/documents/Quiet edge.md"], "[[chords/Quiet edge.canvas|", "[[threads/Quiet edge.canvas|", 1),
	}
	for from, to := range moved {
		want := before[from]
		if f, ok := follow[from]; ok {
			want = f
		}
		if tv.V.Exists(from) {
			t.Errorf("%s is still there", from)
		}
		if got := tv.Read(to); got != want {
			t.Errorf("%s on its way to %s:\n%s", from, to, got)
		}
	}
	if tv.Read("chords/Chord notes.md") != "My notes on the chords.\n" || tv.V.Exists("chords/old") {
		t.Error("chords/ keeps the wrong files")
	}
	if want, _ := vault.Template("Sessions.base"); tv.Read("sessions/Sessions.base") != string(want) {
		t.Error("the 8.x Sessions.base is not upgraded")
	}
	v, err := vault.Open(tv.V.Root)
	if err != nil {
		t.Fatal(err)
	}
	if v.LayoutVersion() != vault.Layout || v.Doc.Front.Has("wikify") {
		t.Fatalf("Atlas.md:\n%s", v.Doc.Content)
	}
	topic := doc.Parse("", []byte(tv.Read(vault.DocPath("Motion scoring"))))
	if topic.Front.Has("from") || strings.Contains(topic.Body, "## Threads") || !strings.Contains(topic.Body, "## Origin\n\nScore boxes by motion.") {
		t.Fatalf("the topic:\n%s", topic.Content)
	}
	if repo := tv.Read(vault.DocPath("p3-edge")); strings.Contains(repo, "## Threads") {
		t.Fatalf("the repository:\n%s", repo)
	}
	if src := doc.Parse("", []byte(tv.Read(vault.DocPath("DINOv2")))); src.Front.Has("from") {
		t.Fatalf("the source:\n%s", src.Content)
	}
	session := doc.Parse("", []byte(tv.Read("sessions/2026-09/2026-09-08 0900 bbbbbb.md")))
	for _, f := range []string{"threads", "specs", "work", "checked", "events"} {
		if session.Front.Has(f) {
			t.Errorf("the session keeps %s", f)
		}
	}
	if !session.Front.Has("repositories") || strings.Contains(session.Body, "Filter alarms") {
		t.Errorf("the session:\n%s", session.Content)
	}
	if c := doc.Parse("", []byte(tv.Read("changes/2026-09/2026-09-10 Learn from the spec.md"))); c.Front.Has("work") || len(c.List("absorbs")) != 1 {
		t.Fatalf("the change:\n%s", c.Content)
	}
	if tv.V.Exists("views/View · Threads.md") {
		t.Error("the views keep View · Threads")
	}

	idx, err := vault.Load(v)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range append(append(idx.Docs, idx.Notes...), idx.Misplaced...) {
		if strings.HasPrefix(d.Path, vault.Threads+"/") {
			t.Errorf("the index reads %s", d.Path)
		}
	}
	f, err := lint.Run(idx, lint.Options{Now: testvault.Now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range f.Findings {
		if x.Severity == lint.Error || x.Check == "dead-link" {
			t.Errorf("lint: %s %s: %s", x.Check, x.Doc.Path, x.Message)
		}
	}

	if _, err := migrate.Run(v, testvault.Now.Add(2*time.Hour)); err == nil || !strings.Contains(err.Error(), "10.0 layout already") {
		t.Fatalf("a second migration: %v", err)
	}
}

// nine writes a vault as 9.0 left it, with paths in links, a Base, and a canvas.
func nine(t *testing.T) *testvault.T {
	tv := testvault.New(t)
	tv.Write("Atlas.md", strings.Replace(tv.Read("Atlas.md"), "layout: 6", "layout: 5", 1))
	tv.Doc("source", "DINOv2", map[string]any{"file": "[[doc-aaaaaa.pdf]]", "sha256": "3f9c1e2a7b8d44aa", "origin": "inbox", "locator": "DINOv2.pdf"}, "## Summary\n\nFeatures.\n")
	tv.Write(vault.Originals+"/doc-aaaaaa.pdf", "%PDF")
	tv.Write(vault.Originals+"/diagram.png", "png")
	tv.Doc("topic", "Stack", map[string]any{"kind": "concept", "sources": []string{"[[DINOv2]]"}}, nineTopic)
	tv.Write("Reading.base", "filters:\n  and:\n    - file.inFolder(\"wiki/documents\")\n")
	tv.Write("threads/Plan.canvas", "{\"nodes\":[{\"id\":\"a\",\"type\":\"file\",\"file\":\"wiki/assets/diagram.png\"}],\"edges\":[]}\n")
	tv.Write(vault.Ingest+"/paper.pdf", "%PDF")
	tv.Write(vault.WikiView+"/View · Home.md", "> [!view] Written by Atlas from the documents. Edits here are lost at the next sync.\n\nHome.\n")
	tv.Write(vault.WikiView+"/My note.md", "mine\n")
	tv.Write(vault.AppJSON, `{"attachmentFolderPath": "wiki/assets", "userIgnoreFilters": ["views/"], "promptDelete": false}`+"\n")
	tv.Write(vault.Originals+"/doc-snap01.md", snapshot9)
	tv.Write("scratchpad/Record.md", record9)
	return older(t, tv)
}

// snapshot9 is a captured original that quotes paths: it stays as it was captured.
const snapshot9 = "var TAG_FOLDER = \"views/tags/\";\nsee [[wiki/documents/X]]\n"

// record9 is a note whose prose and code quote paths, beside a link and a base block.
const record9 = "The sync moved it to 'inbox/Meeting notes.md', as \"views/x.md\" said. See [[wiki/documents/X|X]] and `[[wiki/documents/Y]]`.\n\n```go\nroot := \"wiki/documents\"\n```\n\n```base\nfilters:\n  and:\n    - file.inFolder(\"wiki/documents\")\n```\n"

const nineTopic = "## Definition\n\nThe stack, as drawn: ![[wiki/assets/diagram.png]], and as a [file](wiki/assets/diagram.png). Agents read `wiki/documents` and inbox/ notes.\n"

func TestMigrationFrom9(t *testing.T) {
	tv := nine(t)
	plan, err := migrate.Plan(tv.V)
	if err != nil || plan.From != "9.0" || len(plan.Removed) != 1 || len(plan.Strays) != 1 {
		t.Fatalf("the plan: %+v %v", plan, err)
	}
	tv.Clean()
	report, err := migrate.Run(tv.V, testvault.Now.Add(time.Hour))
	if err != nil || report.Commit == "" || report.Problems != 0 {
		t.Fatalf("the report: %+v %v", report, err)
	}
	tv.Clean()
	if log := tv.Log(); log[0] != "layout: migrate to 10.0" {
		t.Fatalf("the commits: %v", log)
	}
	for _, rel := range []string{"wiki", "inbox", "views"} {
		if tv.V.Exists(rel) {
			t.Errorf("%s/ is still there", rel)
		}
	}
	for _, rel := range []string{vault.Originals + "/doc-aaaaaa.pdf", vault.Originals + "/diagram.png", vault.Ingest + "/paper.pdf", vault.Ingest + "/My note.md", vault.Journals, "wiki-view/View · Home.md"} {
		if !tv.V.Exists(rel) {
			t.Errorf("%s is missing", rel)
		}
	}
	topic := tv.Read(vault.DocPath("Stack"))
	if !strings.Contains(topic, "![[source-core/originals/diagram.png]], and as a [file](source-core/originals/diagram.png). Agents read `wiki/documents` and inbox/ notes.") {
		t.Errorf("the topic's paths:\n%s", topic)
	}
	if got := tv.Read(vault.Originals + "/doc-snap01.md"); got != snapshot9 {
		t.Errorf("a captured original changed:\n%s", got)
	}
	want := strings.Replace(strings.Replace(record9, "[[wiki/documents/X|X]]", "[[source-core/documents/X|X]]", 1), `file.inFolder("wiki/documents")`, `file.inFolder("source-core/documents")`, 1)
	if got := tv.Read("scratchpad/Record.md"); got != want {
		t.Errorf("the record:\n%s\nwant:\n%s", got, want)
	}
	if got := tv.Read("Reading.base"); !strings.Contains(got, `file.inFolder("source-core/documents")`) {
		t.Errorf("the Base:\n%s", got)
	}
	if got := tv.Read("threads/Plan.canvas"); !strings.Contains(got, `"file":"source-core/originals/diagram.png"`) {
		t.Errorf("the canvas:\n%s", got)
	}
	if src := doc.Parse("", []byte(tv.Read(vault.DocPath("DINOv2")))); src.Str("origin") != "ingest" {
		t.Errorf("the source:\n%s", src.Content)
	}
	app := tv.Read(vault.AppJSON)
	if !strings.Contains(app, `"attachmentFolderPath": "source-core/originals"`) || strings.Contains(app, `"views/"`) || !strings.Contains(app, `"wiki-view/"`) || !strings.Contains(app, `"promptDelete": false`) {
		t.Errorf("app.json:\n%s", app)
	}
	exclude, _ := os.ReadFile(filepath.Join(tv.V.Root, ".git", "info", "exclude"))
	if strings.Contains(string(exclude), "/views/") || !strings.Contains(string(exclude), "/wiki-view/") {
		t.Errorf("the exclude file:\n%s", exclude)
	}
	idx, err := vault.Load(tv.V)
	if err != nil {
		t.Fatal(err)
	}
	f, err := lint.Run(idx, lint.Options{Now: testvault.Now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range f.Findings {
		if x.Severity == lint.Error {
			t.Errorf("lint: %s %s: %s", x.Check, x.Doc.Path, x.Message)
		}
	}
}

func TestMigrationRefuses(t *testing.T) {
	t.Run("a vault older than 8.0", func(t *testing.T) {
		tv := eight(t)
		tv.Write("Atlas.md", strings.Replace(atlas8, "layout: 4", "layout: 3", 1))
		tv.Commit()
		v, _ := vault.Open(tv.V.Root)
		if _, err := migrate.Plan(v); err == nil || !strings.Contains(err.Error(), "8.1.1") {
			t.Fatalf("a 7.x vault: %v", err)
		}
	})
	t.Run("wikify without sources", func(t *testing.T) {
		tv := eight(t)
		tv.Write("Atlas.md", strings.Replace(atlas8, "wikify: [source, spec, verification, chord, event]", "wikify: [spec]", 1))
		tv.Commit()
		v, _ := vault.Open(tv.V.Root)
		if plan, err := migrate.Plan(v); err != nil || !strings.Contains(strings.Join(plan.Warnings, " "), "every source the wiki has not absorbed is pending") {
			t.Fatalf("a vault whose wikify left sources out: %+v %v", plan, err)
		}
	})
	t.Run("a proposed change", func(t *testing.T) {
		tv := eight(t)
		tv.Write("changes/2026-09/2026-09-11 Wait.md", "---\nid: chg-wait01\ntype: change\ncreated: 2026-09-11T12:00:00\nupdated: 2026-09-11T12:00:00\nstatus: proposed\nabsorbs: []\nproposed: 2026-09-11T12:00:00\ncounts: 0 create\n---\n\n## Notes\n\nx\n")
		tv.Commit()
		if _, err := migrate.Run(tv.V, testvault.Now.Add(time.Hour)); err == nil || !strings.Contains(err.Error(), "2026-09-11 Wait") {
			t.Fatalf("a vault with a proposed change: %v", err)
		}
		if !tv.V.Exists("wiki/documents/Filter alarms.md") {
			t.Fatal("the refused migration moved a file")
		}
		tv.Clean()
	})
	t.Run("a file in the way", func(t *testing.T) {
		tv := eight(t)
		tv.Write("threads/Quiet edge.canvas", "mine\n")
		tv.Commit()
		if _, err := migrate.Run(tv.V, testvault.Now.Add(time.Hour)); err == nil || !strings.Contains(err.Error(), "threads/Quiet edge.canvas") {
			t.Fatalf("a target that exists: %v", err)
		}
		if log := tv.Log(); strings.HasPrefix(log[0], "layout:") {
			t.Fatalf("the refused migration left a commit: %v", log)
		}
		if tv.Read("threads/Quiet edge.canvas") != "mine\n" || !tv.V.Exists("chords/Quiet edge.canvas") {
			t.Fatal("the refused migration moved a file")
		}
		tv.Clean()
	})
}

func TestAMigrationWhoseCommitFailsPutsTheVaultBack(t *testing.T) {
	tv := eight(t)
	atlas := tv.Read("Atlas.md")
	lock := filepath.Join(tv.V.Root, ".git", "index.lock")
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := migrate.Run(tv.V, testvault.Now.Add(time.Hour))
	os.Remove(lock)
	if err == nil || !strings.Contains(err.Error(), "the vault is back as it was") {
		t.Fatalf("a migration with the index locked: %v", err)
	}
	if tv.Read("Atlas.md") != atlas || !tv.V.Exists("wiki/documents/Filter alarms.md") || tv.Read("chords/Quiet edge.canvas") != canvas8 || !tv.V.Exists("chords/old/Loud edge.canvas") || tv.V.Exists("threads/Filter alarms.md") {
		t.Fatal("the failed migration left the vault changed")
	}
	if log := tv.Log(); strings.HasPrefix(log[0], "layout:") {
		t.Fatalf("the failed migration left a commit: %v", log)
	}
	out, err := exec.Command("git", "-C", tv.V.Root, "status", "--porcelain").CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "" {
		t.Fatalf("the tree is not clean:\n%s %v", out, err)
	}
	v, err := vault.Open(tv.V.Root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migrate.Run(v, testvault.Now.Add(2*time.Hour)); err != nil {
		t.Fatalf("the migration after the lock is gone: %v", err)
	}
}

// A save that lands during a migration, after the step that wrote its path, survives the
// rollback of a migration whose commit fails, and the error names it.
func TestAFailedMigrationKeepsASaveMadeDuringIt(t *testing.T) {
	for _, path := range []string{"Atlas.md", ".obsidian/app.json"} {
		t.Run(path, func(t *testing.T) {
			tv := eight(t)
			git := func(args ...string) string {
				out, err := exec.Command("git", append([]string{"-C", tv.V.Root}, args...)...).CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %v: %s", args, err, out)
				}
				return strings.TrimSpace(string(out))
			}
			head := git("rev-parse", "HEAD")
			lock := filepath.Join(tv.V.Root, ".git", "index.lock")
			saved := "saved during the migration\n"
			migrate.SetBeforeCommit(func() {
				if err := os.WriteFile(tv.V.Abs(path), []byte(saved), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(lock, nil, 0o644); err != nil {
					t.Fatal(err)
				}
			})
			defer migrate.SetBeforeCommit(nil)
			_, err := migrate.Run(tv.V, testvault.Now.Add(time.Hour))
			os.Remove(lock)
			if err == nil || !strings.Contains(err.Error(), path) {
				t.Fatalf("the error does not name the save: %v", err)
			}
			if got := tv.Read(path); got != saved {
				t.Fatalf("the save is gone:\n%s", got)
			}
			if git("rev-parse", "HEAD") != head || git("diff", "--cached", "--name-only") != "" {
				t.Fatal("the failed migration moved HEAD or left something staged")
			}
			for _, l := range strings.Split(git("status", "--porcelain"), "\n") {
				if l != "" && !strings.Contains(l, path) {
					t.Errorf("the failed migration left %s", l)
				}
			}
		})
	}
}

// TestMigrateACopy migrates the vault that ATLAS_MIGRATE_COPY names, for a dry run on a
// copy of a real vault: it prints the report and every lint finding.
func TestMigrateACopy(t *testing.T) {
	root := os.Getenv("ATLAS_MIGRATE_COPY")
	if root == "" {
		t.Skip("set ATLAS_MIGRATE_COPY to the copy of a vault")
	}
	t.Setenv(vault.EnvHome, t.TempDir())
	v, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	report, err := migrate.Run(v, now)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("moved %d files, edited %d documents, commit %s, warnings %v", len(report.Moved), len(report.Edited), report.Commit, report.Warnings)
	v, _ = vault.Open(root)
	idx, err := vault.Load(v)
	if err != nil {
		t.Fatal(err)
	}
	f, err := lint.Run(idx, lint.Options{Now: now})
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range f.Findings {
		if x.Severity == lint.Error {
			t.Errorf("lint: %s %s: %s", x.Check, x.Doc.Path, x.Message)
		} else {
			t.Logf("lint %s: %s %s: %s", x.Severity, x.Check, x.Doc.Path, x.Message)
		}
	}
}
