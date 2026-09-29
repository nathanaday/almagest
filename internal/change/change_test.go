package change_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/change"
	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/lint"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

func str(s string) *string { return &s }

func propose(t *testing.T, tv *testvault.T, p change.Plan) *change.Preview {
	t.Helper()
	pv, err := change.Propose(tv.V, p, tv.Tick(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	return pv
}

func apply(t *testing.T, tv *testvault.T, id string) *change.Preview {
	t.Helper()
	pv, err := change.Apply(tv.V, id, tv.Tick(time.Minute), nil)
	if err != nil {
		t.Fatal(err)
	}
	return pv
}

func TestProposeThenApply(t *testing.T) {
	tv := testvault.New(t)
	tv.Write("Scratch note.md", "a hand edit that is not committed\n")
	pv := propose(t, tv, change.Plan{
		Title: "Add the p3 area",
		Notes: "The p3 product gets an area.",
		Writes: []change.Write{
			{Op: "create", Type: "area", Title: "p3", Fields: map[string]any{"description": "The p3 product.", "id": "are-hacked"}},
			{Op: "create", Type: "concept", Title: "Motion scoring", Fields: map[string]any{"scope": "p3", "description": "Scoring boxes by motion.", "aliases": []any{"motion score"}, "status": "draft"}, Body: str("## Definition\n\nSee [[p3]] and [[ViT]].\n")},
		},
	})
	if pv.Status != "proposed" || pv.Counts.Create != 2 || len(pv.Writes) != 2 {
		t.Fatalf("preview %+v", pv)
	}
	if !strings.Contains(strings.Join(pv.Warnings, "\n"), "id is code's") || !strings.Contains(strings.Join(pv.Warnings, "\n"), "[[ViT]] resolves to nothing") {
		t.Fatalf("warnings %v", pv.Warnings)
	}
	if tv.V.Exists("wiki/p3/p3.md") {
		t.Fatal("propose writes no page")
	}
	content := tv.Read(pv.Ref.Path)
	if !strings.Contains(content, "### create · concept · Motion scoring · con-") || !strings.Contains(content, "`````markdown") {
		t.Fatalf("document:\n%s", content)
	}
	if !strings.Contains(content, `scope: "[[p3]]"`) {
		t.Fatalf("a scope given by title becomes a link:\n%s", content)
	}
	// The user edits the proposed page in Obsidian before saying yes.
	tv.Write(pv.Ref.Path, strings.Replace(content, "Scoring boxes by motion.", "Scoring detection boxes by their motion.", 1))

	applied := apply(t, tv, pv.Ref.ID)
	if applied.Status != "applied" || applied.Commit == "" {
		t.Fatalf("applied %+v", applied)
	}
	if applied.Writes[1].Lines == "+0" {
		t.Fatalf("the preview counts the lines a create writes: %+v", applied.Writes)
	}
	page := tv.Read("wiki/p3/concepts/Motion scoring.md")
	if !strings.Contains(page, "Scoring detection boxes by their motion.") {
		t.Fatalf("the user's edit goes in:\n%s", page)
	}
	log := tv.Log()
	if log[0] != "change: Add the p3 area" || !strings.HasPrefix(log[1], "snapshot: ") {
		t.Fatalf("log %v", log)
	}
	commits, _ := tv.V.Git().Log(1)
	if commits[0].Trailers["Atlas-Change"] != pv.Ref.ID {
		t.Fatalf("trailer %v", commits[0].Trailers)
	}
	tv.Clean()
	doc := tv.Read(pv.Ref.Path)
	if !strings.Contains(doc, "status: applied") || !strings.Contains(doc, "> [!change] Applied") {
		t.Fatalf("status:\n%s", doc)
	}
	if _, err := change.Apply(tv.V, pv.Ref.ID, testvault.Now, nil); err == nil {
		t.Fatal("an applied change does not apply twice")
	}
}

func TestRefusals(t *testing.T) {
	tv := testvault.New(t)
	tv.Page("area", "p3", nil, "")
	tv.Write("threads/X/X.md", "---\nid: thr-aaaaaa\ntype: stub\ncreated: 2026-09-27\nupdated: 2026-09-27\n---\n")
	tv.Commit()
	cases := []struct {
		name string
		w    change.Write
		want string
	}{
		{"a held title", change.Write{Op: "create", Type: "concept", Title: "P3", Fields: map[string]any{"description": "x"}}, "is held by wiki/p3/p3.md"},
		{"a source", change.Write{Op: "create", Type: "source", Title: "S"}, "capture"},
		{"no description", change.Write{Op: "create", Type: "concept", Title: "C"}, "description: is required"},
		{"a scope that is no scope", change.Write{Op: "create", Type: "concept", Title: "C", Fields: map[string]any{"description": "x", "scope": "X"}}, "must be a area or repository"},
		{"a thread document", change.Write{Op: "modify", ID: "thr-aaaaaa", Body: str("x")}, "writes only pages of the wiki"},
		{"a stale base", change.Write{Op: "modify", ID: "p3", Base: "0123456789ab", Body: str("x")}, "conflict"},
		{"a bad status", change.Write{Op: "create", Type: "concept", Title: "C", Fields: map[string]any{"description": "x", "status": "great"}}, `status: is "great"`},
	}
	for _, c := range cases {
		_, err := change.Propose(tv.V, change.Plan{Title: "T", Writes: []change.Write{c.w}}, testvault.Now)
		var r *change.Refusal
		if !errors.As(err, &r) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	many := make([]change.Write, change.MaxWrites+1)
	for i := range many {
		many[i] = change.Write{Op: "create", Type: "concept", Title: fmt.Sprintf("C%d", i), Fields: map[string]any{"description": "x"}}
	}
	if _, err := change.Propose(tv.V, change.Plan{Title: "Big", Writes: many}, testvault.Now); err == nil || !strings.Contains(err.Error(), "at most 100") {
		t.Errorf("too many writes: %v", err)
	}
}

func TestApplyRefusesAPageChangedSinceTheProposal(t *testing.T) {
	tv := testvault.New(t)
	id := tv.Page("concept", "Motion scoring", map[string]any{"scope": ""}, "## Definition\n")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Rewrite", Writes: []change.Write{{Op: "modify", ID: id, Body: str("## Definition\n\nNew.\n")}}})
	tv.Write("wiki/concepts/Motion scoring.md", tv.Read("wiki/concepts/Motion scoring.md")+"\nA hand edit.\n")
	_, err := change.Apply(tv.V, pv.Ref.ID, testvault.Now, nil)
	var c *change.Conflict
	if !errors.As(err, &c) || c.Paths[0] != "wiki/concepts/Motion scoring.md" {
		t.Fatalf("conflict: %v", err)
	}
	if !strings.Contains(tv.Read(pv.Ref.Path), "status: proposed") {
		t.Fatal("a refused apply leaves the change proposed")
	}
}

func TestRenameRewritesLinksEverywhere(t *testing.T) {
	tv := testvault.New(t)
	id := tv.Page("concept", "Old title", nil, "## Definition\n")
	tv.Page("concept", "Linker", map[string]any{"sources": []string{"[[Old title]]"}}, "See [[Old title#Definition|the old one]].\n")
	tv.Write("threads/F/F — Spec.md", "---\nid: spc-aaaaaa\ntype: spec\n---\nAs [[Old title]] says.\n")
	tv.Write("My note.md", "mine links [[Old title]] and `[[Old title]]` in code\n")
	tv.Write("changes/2026-09/2026-09-01 Earlier.md", "---\nid: chg-eeeeee\ntype: change\nstatus: applied\n---\n## Notes\n\nabout [[Old title]]\n\n## Writes\n\n### create · concept · X · con-xxxxxx\n\n`````markdown\n[[Old title]]\n`````\n")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Rename", Writes: []change.Write{{Op: "rename", ID: id, Title: "New title"}}})
	if pv.Counts.Rename != 1 || pv.Counts.Modify != 1 || pv.Counts.LinkRewrites != 3 {
		t.Fatalf("counts %+v writes %+v", pv.Counts, pv.Writes)
	}
	apply(t, tv, pv.Ref.ID)
	if tv.V.Exists("wiki/concepts/Old title.md") || !tv.V.Exists("wiki/concepts/New title.md") {
		t.Fatal("the file moved")
	}
	linker := tv.Read("wiki/concepts/Linker.md")
	if !strings.Contains(linker, `sources: ["[[New title]]"]`) || !strings.Contains(linker, "[[New title#Definition|the old one]]") {
		t.Fatalf("linker:\n%s", linker)
	}
	if !strings.Contains(tv.Read("threads/F/F — Spec.md"), "[[New title]]") {
		t.Fatal("a thread document's link follows")
	}
	if got := tv.Read("My note.md"); got != "mine links [[New title]] and `[[Old title]]` in code\n" {
		t.Fatalf("the user's note: %q", got)
	}
	earlier := tv.Read("changes/2026-09/2026-09-01 Earlier.md")
	if !strings.Contains(earlier, "about [[New title]]") || !strings.Contains(earlier, "`````markdown\n[[Old title]]") {
		t.Fatalf("a change keeps its Writes:\n%s", earlier)
	}
	tv.Clean()
}

func TestRemoveWithRedirectKeepsTypedFields(t *testing.T) {
	tv := testvault.New(t)
	area := tv.Page("area", "p3", nil, "")
	repo := tv.Repo("p3-edge", nil)
	rep := tv.Page("repository", "p3-edge", map[string]any{"path": repo, "parent": "[[p3]]"}, "")
	tv.Page("concept", "Edge fact", map[string]any{"scope": "[[p3-edge]]", "sources": []string{"[[p3-edge]]"}}, "About [[p3-edge]].\n")
	tv.Write("threads/F/F — T1 Do it.md", "---\nid: tsk-aaaaaa\ntype: task\nrepository: \"[[p3-edge]]\"\n---\nIn [[p3-edge]].\n")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Unlink p3-edge", Writes: []change.Write{{Op: "remove", ID: rep, Redirect: area}}})
	if !strings.Contains(strings.Join(pv.Warnings, "\n"), "repository still links [[p3-edge]]") {
		t.Fatalf("warnings %v", pv.Warnings)
	}
	apply(t, tv, pv.Ref.ID)
	if tv.V.Exists("wiki/p3/p3-edge") {
		t.Fatal("the removed repository's folder empties")
	}
	fact := tv.Read("wiki/p3/concepts/Edge fact.md")
	if !strings.Contains(fact, `scope: "[[p3]]"`) || !strings.Contains(fact, "About [[p3]].") {
		t.Fatalf("the page moves up to the area:\n%s", fact)
	}
	task := tv.Read("threads/F/F — T1 Do it.md")
	if !strings.Contains(task, `repository: "[[p3-edge]]"`) || !strings.Contains(task, "In [[p3]].") {
		t.Fatalf("a task's repository never becomes an area:\n%s", task)
	}
}

func TestUndo(t *testing.T) {
	tv := testvault.New(t)
	id := tv.Page("concept", "Motion scoring", nil, "## Definition\n\nOld.\n")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Edit", Writes: []change.Write{
		{Op: "modify", ID: id, Body: str("## Definition\n\nNew.\n")},
		{Op: "create", Type: "entity", Title: "Radar", Fields: map[string]any{"description": "A sensor.", "kind": "tool"}},
	}})
	applied := apply(t, tv, pv.Ref.ID)
	if applied.Writes[0].Lines != "+1 −1" {
		t.Fatalf("apply counts the lines against the page before it: %+v", applied.Writes)
	}
	if shown, _ := change.Show(tv.Index(), pv.Ref.ID); shown.Writes[0].Lines != "+1 −1" {
		t.Fatalf("show counts an applied change against the commit's parent: %+v", shown.Writes)
	}
	tv.Write("Unrelated.md", "a hand edit elsewhere\n")
	undone, err := change.Undo(tv.V, pv.Ref.ID, testvault.Now)
	if err != nil {
		t.Fatal(err)
	}
	if undone.Status != "undone" || tv.V.Exists("wiki/entities/Radar.md") || !strings.Contains(tv.Read("wiki/concepts/Motion scoring.md"), "Old.") {
		t.Fatalf("undo %+v", undone)
	}
	if tv.Log()[0] != "undo: Edit" {
		t.Fatalf("log %v", tv.Log())
	}
	if _, err := change.Undo(tv.V, pv.Ref.ID, testvault.Now); err == nil {
		t.Fatal("an undone change does not undo twice")
	}

	pv = propose(t, tv, change.Plan{Title: "Edit again", Writes: []change.Write{{Op: "modify", ID: id, Body: str("## Definition\n\nNewer.\n")}}})
	apply(t, tv, pv.Ref.ID)
	tv.Write("wiki/concepts/Motion scoring.md", tv.Read("wiki/concepts/Motion scoring.md")+"hand\n")
	if _, err := change.Undo(tv.V, pv.Ref.ID, testvault.Now); err == nil || !strings.Contains(err.Error(), "changed since") {
		t.Fatalf("undo refuses a page edited since: %v", err)
	}
}

func TestRecoveryAfterACrash(t *testing.T) {
	tv := testvault.New(t)
	id := tv.Page("concept", "Motion scoring", nil, "## Definition\n\nOld.\n")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Crash", Writes: []change.Write{{Op: "modify", ID: id, Body: str("## Definition\n\nNew.\n")}, {Op: "create", Type: "entity", Title: "Radar", Fields: map[string]any{"description": "A sensor."}}}})
	// A crash halfway: the status says applying, one page is written, one is new.
	content := tv.Read(pv.Ref.Path)
	content = doc.SetField(doc.SetField(content, "status", "applying"), "paths", []string{"wiki/concepts/Motion scoring.md", "wiki/entities/Radar.md"})
	tv.Write(pv.Ref.Path, content)
	tv.Write("wiki/concepts/Motion scoring.md", "half written")
	tv.Write("wiki/entities/Radar.md", "half written")
	if _, err := change.Reject(tv.V, pv.Ref.ID, "not now", testvault.Now); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tv.Read("wiki/concepts/Motion scoring.md"), "Old.") || tv.V.Exists("wiki/entities/Radar.md") {
		t.Fatal("recovery puts the pages back")
	}
	got := tv.Read(pv.Ref.Path)
	if !strings.Contains(got, "status: rejected") || !strings.Contains(got, "reason: not now") || strings.Contains(got, "paths:") {
		t.Fatalf("recovered, then rejected:\n%s", got)
	}
}

// TestRecoveryStaysInTheVault: paths is frontmatter, which a pull or a shell can write,
// so recovery restores only local documents and leaves every other path alone.
func TestRecoveryStaysInTheVault(t *testing.T) {
	tv := testvault.New(t)
	id := tv.Page("concept", "Motion scoring", nil, "## Definition\n\nOld.\n")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Crash", Writes: []change.Write{{Op: "modify", ID: id, Body: str("## Definition\n\nNew.\n")}}})

	outside := t.TempDir()
	victim := filepath.Join(filepath.Dir(tv.V.Root), "victim.md")
	tv.WriteFile(victim, "keep")
	absolute := filepath.Join(outside, "absolute.md")
	tv.WriteFile(absolute, "keep")
	tv.WriteFile(filepath.Join(outside, "secret.md"), "keep")
	if err := os.Symlink(outside, tv.V.Abs("linked")); err != nil {
		t.Fatal(err)
	}
	tv.Write("wiki/notes.txt", "keep")
	hostile := []string{"../victim.md", absolute, ".git/config", ".git/HEAD.md", "wiki/notes.txt", "linked/secret.md", "wiki/../../victim.md"}

	content := tv.Read(pv.Ref.Path)
	content = doc.SetField(doc.SetField(content, "status", "applying"), "paths", append([]string{"wiki/concepts/Motion scoring.md"}, hostile...))
	tv.Write(pv.Ref.Path, content)
	tv.Write("wiki/concepts/Motion scoring.md", "half written")
	if _, err := change.Reject(tv.V, pv.Ref.ID, "not now", testvault.Now); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tv.Read("wiki/concepts/Motion scoring.md"), "Old.") {
		t.Fatal("recovery puts the local page back")
	}
	for _, file := range []string{victim, absolute, filepath.Join(outside, "secret.md"), tv.V.Abs(".git/config"), tv.V.Abs("wiki/notes.txt")} {
		if _, err := os.Stat(file); err != nil {
			t.Errorf("recovery removed %s: %v", file, err)
		}
	}
	if err := tv.V.Git().RestoreFrom("HEAD", "../victim.md"); err == nil {
		t.Error("RestoreFrom refuses a path out of the tree")
	}
	if err := tv.V.Git().RestoreFrom("HEAD", "linked/secret.md"); err == nil {
		t.Error("RestoreFrom refuses a removal through a link out of the tree")
	}
	if _, err := os.Stat(filepath.Join(outside, "secret.md")); err != nil {
		t.Errorf("RestoreFrom removed a file through a link: %v", err)
	}
}

func TestAbsorbAndPending(t *testing.T) {
	tv := testvault.New(t)
	src := tv.Page("source", "DINOv2", map[string]any{"sha256": "3f9c1e2a7b8d44aa", "file": "[[x.pdf]]", "origin": "inbox"}, "")
	tv.Write("wiki/sources/files/x.pdf", "%PDF")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Ingest DINOv2", Absorbs: []string{src}, Writes: []change.Write{
		{Op: "create", Type: "concept", Title: "Self-supervised learning", Fields: map[string]any{"description": "Learning without labels.", "sources": []any{src}}},
		{Op: "modify", ID: src, Fields: map[string]any{"description": "The DINOv2 paper.", "authority": "primary"}},
	}})
	if len(pv.Absorbs) != 1 {
		t.Fatalf("absorbs %v", pv.Absorbs)
	}
	apply(t, tv, pv.Ref.ID)
	idx := tv.Index()
	if idx.Pending(idx.ByID(src)) {
		t.Fatal("the source is absorbed")
	}
	if _, err := change.Undo(tv.V, pv.Ref.ID, testvault.Now); err != nil {
		t.Fatal(err)
	}
	idx = tv.Index()
	if !idx.Pending(idx.ByID(src)) {
		t.Fatal("an undone change no longer counts")
	}
	if _, err := change.Propose(tv.V, change.Plan{Title: "X", Absorbs: []string{"Self-supervised learning"}}, testvault.Now); err == nil {
		t.Fatal("absorbs names only wikified types")
	}
}

func TestRepositoryPages(t *testing.T) {
	tv := testvault.New(t)
	repo := tv.Repo("p3-cloud", nil)
	os.MkdirAll(repo+"/sub", 0o755)
	bad := []struct {
		path, want string
	}{
		{repo + "/sub", "not the root"},
		{tv.V.Root, "inside the vault"},
		{"relative/path", "not absolute"},
	}
	for _, b := range bad {
		_, err := change.Propose(tv.V, change.Plan{Title: "Link", Writes: []change.Write{{Op: "create", Type: "repository", Title: "p3-cloud", Fields: map[string]any{"path": b.path, "description": "x"}}}}, testvault.Now)
		if err == nil || !strings.Contains(err.Error(), b.want) {
			t.Errorf("%s: %v", b.path, err)
		}
	}
	pv := propose(t, tv, change.Plan{Title: "Link p3-cloud", Writes: []change.Write{{Op: "create", Type: "repository", Title: "p3-cloud", Fields: map[string]any{"path": repo, "description": "The p3 cloud front end.", "remote": "x"}}}})
	apply(t, tv, pv.Ref.ID)
	page := tv.Read("wiki/p3-cloud/p3-cloud.md")
	if !strings.Contains(page, "branch: main") || strings.Contains(page, "remote: x") {
		t.Fatalf("code fills remote and branch:\n%s", page)
	}
	if !strings.Contains(tv.Read(".claude/settings.local.json"), repo) {
		t.Fatal("apply lists the repository for the harness")
	}
	_, err := change.Propose(tv.V, change.Plan{Title: "Again", Writes: []change.Write{{Op: "create", Type: "repository", Title: "p3 cloud 2", Fields: map[string]any{"path": repo, "description": "x"}}}}, testvault.Now)
	if err == nil || !strings.Contains(err.Error(), "already linked") {
		t.Fatalf("a path one page holds: %v", err)
	}
}

func TestSupersedeAndReject(t *testing.T) {
	tv := testvault.New(t)
	first := propose(t, tv, change.Plan{Title: "First", Writes: []change.Write{{Op: "create", Type: "concept", Title: "A", Fields: map[string]any{"description": "a"}}}})
	second := propose(t, tv, change.Plan{Title: "Second", Supersedes: first.Ref.ID, Writes: []change.Write{{Op: "create", Type: "concept", Title: "A", Fields: map[string]any{"description": "better a"}}}})
	if !strings.Contains(tv.Read(first.Ref.Path), "status: superseded") {
		t.Fatal("the first is superseded")
	}
	if _, err := change.Reject(tv.V, second.Ref.ID, "", testvault.Now); err == nil {
		t.Fatal("reject needs a reason")
	}
	if pv, err := change.Reject(tv.V, second.Ref.ID, "not needed", testvault.Now); err != nil || pv.Status != "rejected" {
		t.Fatalf("reject %v %v", pv, err)
	}
	idx := tv.Index()
	if pv, err := change.Show(idx, second.Ref.ID); err != nil || pv.Reason != "not needed" || len(pv.Writes) != 1 {
		t.Fatalf("show %+v %v", pv, err)
	}
	_ = vault.Marker
}

func TestDescribedFollowsAbsorbedSnapshot(t *testing.T) {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", nil)
	rep := tv.Page("repository", "p3-edge", map[string]any{"path": repo}, "")
	head, _ := tv.V.Git().Head()
	_ = head
	src := tv.Page("source", "p3-edge @ 4ac19e2", map[string]any{"sha256": "aaaabbbbccccdddd", "origin": "repository", "locator": rep + "@4ac19e2f00112233", "file": "[[f.md]]"}, "")
	tv.Write("wiki/sources/files/f.md", "snapshot")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Describe p3-edge", Absorbs: []string{src}, Writes: []change.Write{{Op: "modify", ID: src, Fields: map[string]any{"description": "A snapshot."}}}})
	apply(t, tv, pv.Ref.ID)
	if !strings.Contains(tv.Read("wiki/p3-edge/p3-edge.md"), "described: 4ac19e2") {
		t.Fatalf("described:\n%s", tv.Read("wiki/p3-edge/p3-edge.md"))
	}
	tv.Clean()
}

func TestAChangeThatRenamesWhatItAbsorbsKeepsItsOwnLinks(t *testing.T) {
	tv := testvault.New(t)
	src := tv.Page("source", "2309.01234v2", map[string]any{"sha256": "3f9c1e2a7b8d44aa", "file": "[[x.pdf]]", "origin": "inbox"}, "")
	tv.Write("wiki/sources/files/x.pdf", "%PDF")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Ingest a paper", Notes: "Renames [[2309.01234v2]] to its title.", Absorbs: []string{src}, Writes: []change.Write{
		{Op: "modify", ID: src, Fields: map[string]any{"description": "A paper on motion.", "authority": "primary"}},
		{Op: "rename", ID: src, Title: "Motion scoring at night"},
	}})
	apply(t, tv, pv.Ref.ID)
	record := tv.Read(pv.Ref.Path)
	if strings.Contains(record, "[[2309.01234v2]]") {
		t.Fatalf("the change's own record keeps the old title:\n%s", record)
	}
	for _, want := range []string{`absorbs: ["[[Motion scoring at night]]"]`, "| [[Motion scoring at night]] | " + src, "absorbs [[Motion scoring at night]]", "Renames [[Motion scoring at night]]"} {
		if !strings.Contains(record, want) {
			t.Errorf("the record lacks %q:\n%s", want, record)
		}
	}
	if f, _ := lint.Run(tv.Index(), lint.Options{Now: testvault.Now}); f.Counts[lint.Error] != 0 {
		t.Fatalf("lint after apply: %+v", f.Findings)
	}
	tv.Clean()
	if _, err := change.Undo(tv.V, pv.Ref.ID, testvault.Now); err != nil {
		t.Fatal(err)
	}
	if record := tv.Read(pv.Ref.Path); !strings.Contains(record, `absorbs: ["[[2309.01234v2]]"]`) {
		t.Fatalf("after undo the record links the old title again:\n%s", record)
	}
	if f, _ := lint.Run(tv.Index(), lint.Options{Now: testvault.Now}); f.Counts[lint.Error] != 0 {
		t.Fatalf("lint after undo: %+v", f.Findings)
	}
}
