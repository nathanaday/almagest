package change_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/change"
	"github.com/nathanaday/atlas-obsidian/internal/doc"
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

func refused(t *testing.T, tv *testvault.T, p change.Plan, want string) {
	t.Helper()
	_, err := change.Propose(tv.V, p, tv.Tick(time.Minute))
	var r *change.Refusal
	if !errors.As(err, &r) || !strings.Contains(err.Error(), want) {
		t.Fatalf("want a refusal with %q, got %v", want, err)
	}
}

func TestProposeThenApply(t *testing.T) {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", nil)
	tv.Write("Scratch note.md", "a hand edit that is not committed\n")
	pv := propose(t, tv, change.Plan{
		Title: "Link p3-edge",
		Notes: "The edge service, with its first concept.",
		Writes: []change.Write{
			{Op: "create", Type: "repository", Title: "p3-edge", Fields: map[string]any{"description": "The p3 edge service.", "path": repo, "defines": "Work/P3/p3-edge", "tags": []any{"work/p3"}, "id": "doc-hacked"}, Body: str("## What it is\n\nThe edge.\n")},
			{Op: "create", Type: "topic", Kind: "concept", Title: "Motion scoring", Fields: map[string]any{"tags": []any{"work/p3/p3-edge", "ml"}, "description": "Scoring boxes by motion.", "aliases": []any{"motion score"}, "status": "draft"}, Body: str("## Definition\n\nSee [[p3-edge]] and [[ViT]].\n")},
		},
	})
	if pv.Status != "proposed" || pv.Counts.Create != 2 || len(pv.Writes) != 2 || pv.Writes[1].Kind != "concept" || len(pv.NewTags) != 3 {
		t.Fatalf("preview %+v", pv)
	}
	warnings := strings.Join(pv.Warnings, "\n")
	if !strings.Contains(warnings, "id is code's") || !strings.Contains(warnings, "[[ViT]] resolves to nothing") {
		t.Fatalf("warnings %v", pv.Warnings)
	}
	if tv.V.Exists("source-core/documents/p3-edge.md") {
		t.Fatal("propose writes no document")
	}
	content := tv.Read(pv.Ref.Path)
	for _, want := range []string{"### create · topic concept · Motion scoring · doc-", "### create · repository · p3-edge · doc-", "`````markdown", "new_tags: [work/p3/p3-edge, work/p3, ml]"} {
		if !strings.Contains(content, want) {
			t.Errorf("change document lacks %q", want)
		}
	}
	applied := apply(t, tv, pv.Ref.ID)
	if applied.Status != "applied" || applied.Commit == "" {
		t.Fatalf("applied %+v", applied)
	}
	topic := tv.Read("source-core/documents/Motion scoring.md")
	for _, want := range []string{"type: topic", "kind: concept", "tags: [work/p3/p3-edge, ml]", "refreshed: 2026-09-27T14:34:00", "> [!concept] Draft · Concept", "## Definition"} {
		if !strings.Contains(topic, want) {
			t.Errorf("topic lacks %q:\n%s", want, topic)
		}
	}
	rp := tv.Read("source-core/documents/p3-edge.md")
	for _, want := range []string{"defines: work/p3/p3-edge", "branch: ", "head: ", "> [!repository] `", "```atlas-repo", "## Knowledge"} {
		if !strings.Contains(rp, want) {
			t.Errorf("repository lacks %q:\n%s", want, rp)
		}
	}
	if strings.Contains(rp, "## Threads") {
		t.Errorf("repository has a Threads section:\n%s", rp)
	}
	if log := tv.Log(); log[0] != "change: Link p3-edge" || log[1] != "snapshot: 2 files edited by hand" {
		t.Fatalf("log %v", log)
	}
	tv.Clean()
}

func TestRefusals(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("topic", "CS513", map[string]any{"kind": "overview", "defines": "school/cs513"}, "")
	tv.Commit()
	refused(t, tv, change.Plan{Title: "x", Writes: []change.Write{{Op: "create", Type: "stub", Title: "S"}}}, "a change creates a topic or a repository")
	refused(t, tv, change.Plan{Title: "x", Writes: []change.Write{{Op: "create", Type: "source", Title: "S"}}}, "capture")
	refused(t, tv, change.Plan{Title: "x", Writes: []change.Write{{Op: "promote", ID: "CS513", Kind: "concept", Title: "S"}}}, `op "promote"; it must be create, modify, rename, remove, confirm, or retag`)
	refused(t, tv, change.Plan{Title: "x", Absorbs: []string{"CS513"}}, "CS513 is a topic; a change absorbs sources")
	refused(t, tv, change.Plan{Title: "x", Writes: []change.Write{{Op: "create", Type: "topic", Kind: "idea", Title: "S"}}}, "a topic is a concept")
	refused(t, tv, change.Plan{Title: "x", Writes: []change.Write{{Op: "create", Type: "topic", Kind: "concept", Title: "Tag · x", Fields: map[string]any{"description": "x"}}}}, "view's title")
	refused(t, tv, change.Plan{Title: "x", Writes: []change.Write{{Op: "create", Type: "topic", Kind: "overview", Title: "Another", Fields: map[string]any{"description": "x", "defines": "school/cs513"}}}}, "defines school/cs513 already")
	refused(t, tv, change.Plan{Title: "x", Writes: []change.Write{{Op: "create", Type: "topic", Kind: "concept", Title: "C", Fields: map[string]any{"description": "x", "defines": "x"}}}}, "only an overview or a repository defines")
	refused(t, tv, change.Plan{Title: "x", Writes: []change.Write{{Op: "create", Type: "repository", Title: "R", Fields: map[string]any{"description": "x", "path": tv.V.Root}}}}, "inside the vault")
	refused(t, tv, change.Plan{Title: "x", Writes: []change.Write{{Op: "create", Type: "topic", Kind: "concept", Title: "C", Fields: map[string]any{"description": "x", "tags": []any{"bad tag!"}}}}}, "no valid tag")
	refused(t, tv, change.Plan{Title: "x", Writes: []change.Write{{Op: "retag", From: "nothing", To: "x"}}}, "no document holds nothing")
	tv.Write("Atlas.md", strings.Replace(tv.Read("Atlas.md"), "tagging: open", "tagging: known", 1))
	tv.Commit()
	v, _ := vault.Open(tv.V.Root)
	tv.V = v
	refused(t, tv, change.Plan{Title: "x", Writes: []change.Write{{Op: "create", Type: "topic", Kind: "concept", Title: "C", Fields: map[string]any{"description": "x", "tags": []any{"brand-new"}}}}}, "new_tags: true")
	propose(t, tv, change.Plan{Title: "x", NewTags: true, Writes: []change.Write{{Op: "create", Type: "topic", Kind: "concept", Title: "C", Fields: map[string]any{"description": "x", "tags": []any{"brand-new", "school"}}}}})
}

func TestApplyRefusesADocumentChangedSinceTheProposal(t *testing.T) {
	tv := testvault.New(t)
	id := tv.Doc("topic", "Motion scoring", map[string]any{"kind": "concept"}, "## Definition\n\nOld.\n")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Edit", Writes: []change.Write{{Op: "modify", ID: id, Body: str("## Definition\n\nNew.\n")}}})
	tv.Write("source-core/documents/Motion scoring.md", tv.Read("source-core/documents/Motion scoring.md")+"hand edit\n")
	_, err := change.Apply(tv.V, pv.Ref.ID, tv.Tick(time.Minute), nil)
	var c *change.Conflict
	if !errors.As(err, &c) || c.Paths[0] != "source-core/documents/Motion scoring.md" {
		t.Fatalf("conflict: %v", err)
	}
}

func TestASyncBetweenProposalAndApplyIsNoConflict(t *testing.T) {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", nil)
	id := tv.Doc("repository", "p3-edge", map[string]any{"path": repo, "head": "aaaaaaa", "behind": 0}, "> [!repository] old\n\n## What it is\n\nOld.\n")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Describe", Writes: []change.Write{{Op: "modify", ID: id, Fields: map[string]any{"description": "The edge."}, Body: str("## What it is\n\nNew.\n")}}})
	// A sync refreshes what code derives: a git fact, the lead callout, a code section.
	rel := "source-core/documents/p3-edge.md"
	synced := strings.Replace(strings.Replace(tv.Read(rel), "head: aaaaaaa", "head: bbbbbbb", 1), "> [!repository] old", "> [!repository] new", 1)
	tv.Write(rel, strings.Replace(synced, "behind: 0", "behind: 3", 1)+"\n## Knowledge\n\n```base\nfilters: x\n```\n")
	apply(t, tv, pv.Ref.ID)
	got := tv.Read(rel)
	for _, want := range []string{"head: bbbbbbb", "behind: 3", "description: The edge.", "## What it is\n\nNew."} {
		if !strings.Contains(got, want) {
			t.Errorf("after the apply the page lacks %q:\n%s", want, got)
		}
	}
	// An edit of a field the model gives is a conflict.
	pv = propose(t, tv, change.Plan{Title: "Describe again", Writes: []change.Write{{Op: "modify", ID: id, Body: str("## What it is\n\nNewer.\n")}}})
	tv.Write(rel, strings.Replace(tv.Read(rel), "description: The edge.", "description: Edited by hand.", 1))
	var c *change.Conflict
	if _, err := change.Apply(tv.V, pv.Ref.ID, tv.Tick(time.Minute), nil); !errors.As(err, &c) {
		t.Fatalf("a hand edit of a field is a conflict: %v", err)
	}
}

func TestRenameRewritesLinksEverywhere(t *testing.T) {
	tv := testvault.New(t)
	id := tv.Doc("topic", "Motion scoring", map[string]any{"kind": "concept"}, "")
	tv.Doc("topic", "Tracking", map[string]any{"kind": "concept"}, "Uses [[Motion scoring|scores]] and [[Motion scoring#Definition]].\n")
	tv.Write("sessions/2026-09/2026-09-26 0900 aaaaaa.md", "---\nid: ses-aaaaaa\ntype: session\nharness_id: aaaaaa\nstatus: ended\nupdated: 2026-09-26T09:00:00\n---\n\nApply [[Motion scoring]].\n")
	tv.Write("My note.md", "See [[motion scoring]].\n")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Rename", Writes: []change.Write{{Op: "rename", ID: id, Title: "Motion score"}}})
	if pv.Counts.Rename != 1 || pv.Counts.Modify != 1 || pv.Counts.LinkRewrites != 2 {
		t.Fatalf("counts %+v", pv.Counts)
	}
	apply(t, tv, pv.Ref.ID)
	if tv.V.Exists("source-core/documents/Motion scoring.md") || !tv.V.Exists("source-core/documents/Motion score.md") {
		t.Fatal("the file moved")
	}
	for rel, want := range map[string]string{
		"source-core/documents/Tracking.md":          "Uses [[Motion score|scores]] and [[Motion score#Definition]].",
		"sessions/2026-09/2026-09-26 0900 aaaaaa.md": "Apply [[Motion score]].",
		"My note.md": "See [[Motion score]].",
	} {
		if !strings.Contains(tv.Read(rel), want) {
			t.Errorf("%s:\n%s", rel, tv.Read(rel))
		}
	}
	tv.Clean()
}

func TestRemoveRepositoryAndUnlink(t *testing.T) {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", nil)
	rid := tv.Doc("repository", "p3-edge", map[string]any{"path": repo}, "")
	tv.Commit()
	removal := propose(t, tv, change.Plan{Title: "Remove p3-edge", Writes: []change.Write{{Op: "remove", ID: rid}}})
	if removal.Counts.Remove != 1 {
		t.Fatalf("a remove of a repository: %+v", removal.Counts)
	}
	pv := propose(t, tv, change.Plan{Title: "Unlink p3-edge", Supersedes: removal.Ref.ID, Writes: []change.Write{{Op: "modify", ID: rid, Fields: map[string]any{"unlinked": true}}}})
	apply(t, tv, pv.Ref.ID)
	got := tv.Read("source-core/documents/p3-edge.md")
	if !strings.Contains(got, "unlinked: true") || !strings.Contains(got, `path: ""`) || !strings.Contains(got, "> [!repository-missing] Unlinked") {
		t.Fatalf("unlinked:\n%s", got)
	}
	for _, r := range tv.V.Repositories() {
		if r.Path != "" {
			t.Fatal("an unlinked repository names no path")
		}
	}
}

func TestUndo(t *testing.T) {
	tv := testvault.New(t)
	id := tv.Doc("topic", "Motion scoring", map[string]any{"kind": "concept"}, "## Definition\n\nOld.\n")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Edit", Writes: []change.Write{
		{Op: "modify", ID: id, Body: str("## Definition\n\nNew.\n")},
		{Op: "create", Type: "topic", Kind: "entity", Title: "Radar", Fields: map[string]any{"description": "A sensor.", "tags": []any{"tool"}}},
	}})
	applied := apply(t, tv, pv.Ref.ID)
	if applied.Writes[0].Lines != "+3 −2" {
		t.Fatalf("apply counts the lines against the document before it: %+v", applied.Writes)
	}
	if shown, _ := change.Show(tv.Index(), pv.Ref.ID); shown.Writes[0].Lines != "+3 −2" {
		t.Fatalf("show counts an applied change against the commit's parent: %+v", shown.Writes)
	}
	tv.Write("Unrelated.md", "a hand edit elsewhere\n")
	undone, err := change.Undo(tv.V, pv.Ref.ID, testvault.Now)
	if err != nil {
		t.Fatal(err)
	}
	if undone.Status != "undone" || tv.V.Exists("source-core/documents/Radar.md") || !strings.Contains(tv.Read("source-core/documents/Motion scoring.md"), "Old.") {
		t.Fatalf("undo %+v", undone)
	}
	if tv.Log()[0] != "undo: Edit" {
		t.Fatalf("log %v", tv.Log())
	}
	pv = propose(t, tv, change.Plan{Title: "Edit again", Writes: []change.Write{{Op: "modify", ID: id, Body: str("## Definition\n\nNewer.\n")}}})
	apply(t, tv, pv.Ref.ID)
	tv.Write("source-core/documents/Motion scoring.md", tv.Read("source-core/documents/Motion scoring.md")+"hand\n")
	if _, err := change.Undo(tv.V, pv.Ref.ID, testvault.Now); err == nil || !strings.Contains(err.Error(), "changed since") {
		t.Fatalf("undo refuses a document edited since: %v", err)
	}
}

func TestRecoveryAfterACrash(t *testing.T) {
	tv := testvault.New(t)
	id := tv.Doc("topic", "Motion scoring", map[string]any{"kind": "concept"}, "## Definition\n\nOld.\n")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Crash", Writes: []change.Write{{Op: "modify", ID: id, Body: str("## Definition\n\nNew.\n")}, {Op: "create", Type: "topic", Kind: "entity", Title: "Radar", Fields: map[string]any{"description": "A sensor."}}}})
	content := tv.Read(pv.Ref.Path)
	content = doc.SetField(doc.SetField(content, "status", "applying"), "paths", []string{"source-core/documents/Motion scoring.md", "source-core/documents/Radar.md", "../victim.md", ".git/config"})
	tv.Write(pv.Ref.Path, content)
	tv.Write("source-core/documents/Motion scoring.md", "half written")
	tv.Write("source-core/documents/Radar.md", "half written")
	victim := filepath.Join(filepath.Dir(tv.V.Root), "victim.md")
	tv.WriteFile(victim, "keep")
	if _, err := change.Reject(tv.V, pv.Ref.ID, "not now", testvault.Now); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tv.Read("source-core/documents/Motion scoring.md"), "Old.") || tv.V.Exists("source-core/documents/Radar.md") {
		t.Fatal("recovery puts the documents back")
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatal("recovery stays in the vault")
	}
	got := tv.Read(pv.Ref.Path)
	if !strings.Contains(got, "status: rejected") || !strings.Contains(got, "reason: not now") || strings.Contains(got, "paths:") {
		t.Fatalf("recovered, then rejected:\n%s", got)
	}
}

func TestAbsorbConfirmRetag(t *testing.T) {
	tv := testvault.New(t)
	src := tv.Doc("source", "DINOv2", map[string]any{"sha256": "3f9c1e2a7b8d44aa", "file": "[[doc-aaaaaa.pdf]]", "media": "pdf", "origin": "ingest", "tags": []string{"p3", "paper"}}, "")
	tv.Write("source-core/originals/doc-aaaaaa.pdf", "%PDF")
	old := tv.Doc("topic", "Old idea", map[string]any{"kind": "concept", "tags": []string{"p3/edge"}}, "## Definition\n\nx\n")
	tv.Write("My note.md", "- [ ] check #p3/edge later\n")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Ingest DINOv2", Absorbs: []string{src}, Writes: []change.Write{
		{Op: "create", Type: "topic", Kind: "concept", Title: "Motion scoring", Fields: map[string]any{"description": "Scoring boxes by motion.", "sources": []any{src}, "tags": []any{"p3"}}, Body: str("## Definition\n\nMotion.\n")},
		{Op: "modify", ID: src, Fields: map[string]any{"description": "The DINOv2 paper.", "authority": "primary"}, Body: str("## Summary\n\nSelf-supervised features.\n")},
		{Op: "confirm", ID: old},
		{Op: "retag", From: "p3", To: "work/p3"},
	}})
	if pv.Counts.Create != 1 || pv.Counts.Confirm != 1 || pv.Counts.Retag != 1 || pv.Counts.TagRewrites != 1 {
		t.Fatalf("counts %+v", pv.Counts)
	}
	if idx := tv.Index(); !idx.Pending(idx.ByID(src)) {
		t.Fatal("pending before apply")
	}
	apply(t, tv, pv.Ref.ID)
	idx := tv.Index()
	if idx.Pending(idx.ByID(src)) {
		t.Fatal("the source is absorbed")
	}
	source := tv.Read("source-core/documents/DINOv2.md")
	for _, want := range []string{"status: absorbed", "tags: [work/p3, paper]", "absorbed by [[2026-09-27 Ingest DINOv2]]", "![[doc-aaaaaa.pdf]]", "## Summary\n\nSelf-supervised features."} {
		if !strings.Contains(source, want) {
			t.Errorf("source lacks %q:\n%s", want, source)
		}
	}
	topic := tv.Read("source-core/documents/Motion scoring.md")
	for _, want := range []string{"type: topic", "kind: concept", "tags: [work/p3]", "sources: [\"[[DINOv2]]\"]", "> [!concept]"} {
		if !strings.Contains(topic, want) {
			t.Errorf("created topic lacks %q:\n%s", want, topic)
		}
	}
	if got := tv.Read("source-core/documents/Old idea.md"); !strings.Contains(got, "tags: [work/p3/edge]") || !strings.Contains(got, "refreshed: 2026-09-27T14:34:00") || !strings.Contains(got, "\nx\n") {
		t.Fatalf("confirmed and retagged:\n%s", got)
	}
	if got := tv.Read("My note.md"); got != "- [ ] check #work/p3/edge later\n" {
		t.Fatalf("inline tag: %q", got)
	}
	tv.Clean()
}

func TestSupersedeAndReject(t *testing.T) {
	tv := testvault.New(t)
	first := propose(t, tv, change.Plan{Title: "First", Writes: []change.Write{{Op: "create", Type: "topic", Kind: "concept", Title: "A", Fields: map[string]any{"description": "a"}}}})
	second := propose(t, tv, change.Plan{Title: "Second", Supersedes: first.Ref.ID, Writes: []change.Write{{Op: "create", Type: "topic", Kind: "concept", Title: "A", Fields: map[string]any{"description": "a, better"}}}})
	if shown, _ := change.Show(tv.Index(), first.Ref.ID); shown.Status != "superseded" {
		t.Fatalf("first %s", shown.Status)
	}
	if _, err := change.Apply(tv.V, first.Ref.ID, tv.Tick(time.Minute), nil); err == nil {
		t.Fatal("a superseded change does not apply")
	}
	if _, err := change.Reject(tv.V, second.Ref.ID, "", tv.Tick(time.Minute)); err == nil {
		t.Fatal("reject needs a reason")
	}
	pv, err := change.Reject(tv.V, second.Ref.ID, "not needed", tv.Tick(time.Minute))
	if err != nil || pv.Status != "rejected" || pv.Reason != "not needed" {
		t.Fatalf("%+v %v", pv, err)
	}
}

func TestDescribedFollowsAbsorbedSnapshot(t *testing.T) {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", nil)
	rid := tv.Doc("repository", "p3-edge", map[string]any{"path": repo}, "")
	head, _ := (&vault.Vault{Root: repo}).Git().Head()
	src := tv.Doc("source", "p3-edge @ "+head[:7], map[string]any{"sha256": "aa", "file": "[[doc-bbbbbb.md]]", "origin": "repository", "locator": rid + "@" + head}, "")
	tv.Commit()
	pv := propose(t, tv, change.Plan{Title: "Describe p3-edge", Absorbs: []string{src}, Writes: []change.Write{{Op: "modify", ID: src, Fields: map[string]any{"description": "A snapshot."}}}})
	apply(t, tv, pv.Ref.ID)
	got := tv.Read("source-core/documents/p3-edge.md")
	// A short hash such as 9572e60 reads as a number, so YAML quotes it.
	described := strings.Contains(got, "described: "+head[:7]) || strings.Contains(got, `described: "`+head[:7]+`"`)
	if !described || !strings.Contains(got, "behind: 0") || !strings.Contains(got, "current") {
		t.Fatalf("described:\n%s", got)
	}
}
