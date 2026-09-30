package migrate_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/lint"
	"github.com/nathanaday/atlas-obsidian/internal/migrate"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
	"github.com/nathanaday/atlas-obsidian/internal/work"
)

const specBody = "> [!spec] Filter alarms\n> stub → **Spec**\n\n## Goal\n\nFewer false alarms on the edge.\n\n## Done when\n\n- half as many alarms\n"

// legacy writes a vault as 6.5 left it: scope folders in the wiki, threads under their
// home, and the 6.x fields.
func legacy(t *testing.T) *testvault.T {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", nil)
	tv.Write("Atlas.md", "---\nid: vlt-aaaaaa\ntype: vault\nname: Work\ndescription: Work notes.\ncreated: 2026-09-01\nupdated: 2026-09-01\nareas: few\nwikify: [source, spec, receipt]\nstale_hours: 12\nlayout: 2\n---\nThe vault's own context.\n")
	tv.Write("wiki/work/work.md", "---\nid: are-work01\ntype: area\ncreated: 2026-09-01\nupdated: 2026-09-01\nparent: \"\"\ndescription: All work.\naliases: []\nchain: []\n---\n\n> [!area] work\n\nThe work area.\n")
	tv.Write("wiki/work/p3/p3.md", "---\nid: are-p3aaaa\ntype: area\ncreated: 2026-09-01\nupdated: 2026-09-01\nparent: \"[[work]]\"\ndescription: The p3 product.\naliases: []\n---\n\nWhat an agent must know about p3.\n")
	tv.Write("wiki/work/p3/p3-edge/p3-edge.md", "---\nid: rep-edge01\ntype: repository\ncreated: 2026-09-01\nupdated: 2026-09-01\nparent: \"[[p3]]\"\ndescription: The edge service.\naliases: []\npath: \""+repo+"\"\nremote: \"\"\nbranch: main\ndescribed: \"\"\n---\n\n## What it is\n\nThe edge.\n")
	tv.Write("wiki/work/p3/concepts/Motion scoring.md", "---\nid: con-motion\ntype: concept\ncreated: 2026-09-02\nupdated: 2026-09-02\nscope: \"[[p3]]\"\ndescription: Scoring boxes by motion.\naliases: []\ntags: [vision]\nstatus: stable\nsources: [\"[[DINOv2]]\"]\n---\n\n## Definition\n\nMotion.\n")
	tv.Write("wiki/work/p3/p3-edge/entities/Radar.md", "---\nid: ent-radar1\ntype: entity\ncreated: 2026-09-02\nupdated: 2026-09-02\nscope: \"[[p3-edge]]\"\ndescription: A sensor.\nkind: tool\nsources: [\"[[DINOv2]]\"]\n---\n\n## What it is\n\nSee [[Motion scoring]].\n")
	tv.Write("wiki/policies/Pin deps.md", "---\nid: pol-pindep\ntype: policy\ncreated: 2026-09-02\nupdated: 2026-09-02\nscope: \"\"\ndescription: Pin every dependency.\nstrength: must\nsources: [\"[[DINOv2]]\"]\n---\n\n## Rule\n\nPin. See [[Motion scoring]].\n")
	tv.Write("wiki/work/p3/sources/DINOv2.md", "---\nid: src-aaaaaa\ntype: source\ncreated: 2026-09-02\nupdated: 2026-09-02\nscope: \"[[p3]]\"\ndescription: The DINOv2 paper.\nsources: []\nfile: \"[[src-aaaaaa.pdf]]\"\nsha256: 3f9c1e2a7b8d44aa\norigin: inbox\nlocator: DINOv2.pdf\nmeasure: 31 pages\ncaptured: 2026-09-02\nauthority: primary\n---\n\n![[src-aaaaaa.pdf]]\n\n## Summary\n\nFeatures. Cited by [[Radar]] and [[Pin deps]].\n")
	tv.Write("wiki/sources/files/src-aaaaaa.pdf", "%PDF")
	tv.Write("wiki/sources/files/src-bbbbbb.md", "---\ntype: concept\ntags: [vocabulary]\n---\n\nA captured note.\n")
	tv.Write("wiki/work/p3/diagram.png", "png")
	tv.Write("wiki/work/p3/My note.md", "a note of mine\n")
	old, _ := vault.OldTemplate("Wiki.base")
	tv.Write("wiki/Wiki.base", old)
	old, _ = vault.OldTemplate("Threads.base")
	tv.Write("threads/Threads.base", old)
	tv.Write("threads/Threads.canvas", "{\"nodes\":[],\"edges\":[]}\n")
	tv.Write("threads/Idea/Idea.md", "---\nid: thr-idea01\ntype: stub\ncreated: 2026-09-03\nupdated: 2026-09-03\nscope: []\npriority: low\nblocked: \"\"\nstage: stub\n---\n\n> [!stub] Idea\n\n## Stub\n\nTry a smaller backbone. Soon.\n\n## Notes\n\nmine\n")
	home := "threads/work/p3/p3-edge/Filter alarms/"
	tv.Write(home+"Filter alarms.md", "---\nid: thr-filter\ntype: stub\ncreated: 2026-09-04\nupdated: 2026-09-12\nscope: [\"[[p3-edge]]\"]\npriority: high\nblocked: waiting on data\nstage: tasks\nchain: [\"[[work]]\", \"[[p3]]\", \"[[p3-edge]]\"]\n---\n\n> [!tasks] Filter alarms\n\n## Stub\n\nCut the false alarms.\n")
	tv.Write(home+"Filter alarms — Spec.md", "---\nid: spc-filter\ntype: spec\nthread: \"[[Filter alarms]]\"\nthread_id: thr-filter\ncreated: 2026-09-05\nupdated: 2026-09-05\n---\n"+specBody)
	tv.Write(home+"Filter alarms — T1 Score boxes.md", "---\nid: tsk-score1\ntype: task\nthread: \"[[Filter alarms]]\"\nthread_id: thr-filter\ncreated: 2026-09-06\nupdated: 2026-09-08T10:00:00\norder: 1\nrepository: \"[[p3-edge]]\"\ndepends: []\nstatus: done\n---\n\n## What\n\nScore boxes by motion.\n\n## Progress\n\n- 2026-09-07: began\n\n## Result\n\nScored in abc123.\n")
	tv.Write(home+"Filter alarms — T2 Tune threshold.md", "---\nid: tsk-tune01\ntype: task\nthread: \"[[Filter alarms]]\"\nthread_id: thr-filter\ncreated: 2026-09-06\nupdated: 2026-09-06\norder: 2\nrepository: \"[[p3-edge]]\"\ndepends: [\"[[Filter alarms — T1 Score boxes]]\"]\nstatus: open\n---\n\n## What\n\nTune it.\n")
	tv.Write(home+"Filter alarms — Receipt (reopened 2026-09-12).md", "---\nid: rcp-first1\ntype: receipt\nthread: \"[[Filter alarms]]\"\nthread_id: thr-filter\ncreated: 2026-09-10\nupdated: 2026-09-12\noutcome: completed\nsuperseded: true\n---\n\n## Delivered\n\nScoring.\n\n## Verified\n\nTests.\n")
	tv.Write("threads/Killed/Killed.md", "---\nid: thr-killed\ntype: stub\ncreated: 2026-09-03\nupdated: 2026-09-03\nscope: []\nstage: closed\n---\n\n## Stub\n\nA dead idea.\n")
	tv.Write("threads/Killed/Killed — Receipt.md", "---\nid: rcp-killed\ntype: receipt\nthread: \"[[Killed]]\"\nthread_id: thr-killed\ncreated: 2026-09-04\nupdated: 2026-09-04\noutcome: killed\n---\n\n## Why killed\n\nNo longer needed.\n")
	tv.Write("sessions/2026-09/2026-09-08 0900 bbbbbb.md", "---\nid: ses-bbbbbb\ntype: session\ncreated: 2026-09-08\nupdated: 2026-09-08T10:00:00\nharness_id: bbbbbb\nstatus: ended\nthreads: [\"[[Filter alarms]]\"]\ntasks: [\"[[Filter alarms — T1 Score boxes]]\"]\nrepositories: [\"[[p3-edge]]\"]\nchanges: []\n---\n\n## Description\n\nScoring.\n")
	hash := doc.ContentHash(specBody)
	tv.Write("changes/2026-09/2026-09-10 Learn from the spec.md", "---\nid: chg-learn1\ntype: change\ncreated: 2026-09-10\nupdated: 2026-09-10\nstatus: applied\nabsorbs: [\"[[Filter alarms — Spec]]\", \"[[DINOv2]]\"]\nthread: \"[[Filter alarms]]\"\napplied: 2026-09-10T12:00:00\n---\n\n## Notes\n\nx\n\n## Absorbed\n\n| Document | Id | Hash |\n|---|---|---|\n| [[Filter alarms — Spec]] | spc-filter | "+hash[:12]+" |\n| [[DINOv2]] | src-aaaaaa | 3f9c1e2a7b8d |\n\n## Writes\n\n### create · concept · X · con-xxxxxx\n\n`````markdown\nsee [[Filter alarms — Spec]]\n`````\n")
	tv.Write("Ideas.md", "Next: [[Filter alarms — T1 Score boxes]], per [[Filter alarms — Spec|the spec]] and [[Killed — Receipt]].\n")
	for _, p := range []string{"wiki/documents", "wiki/assets", "views"} {
		tv.V.Remove(p)
	}
	tv.Commit()
	v, err := vault.Open(tv.V.Root)
	if err != nil {
		t.Fatal(err)
	}
	tv.V = v
	return tv
}

func TestMigration(t *testing.T) {
	tv := legacy(t)
	if _, err := work.Stub(tv.V, work.StubIn{Text: "x"}, work.Opts{Now: testvault.Now}); err == nil || !strings.Contains(err.Error(), "vault migrate") {
		t.Fatalf("a write refuses a 6.x vault: %v", err)
	}
	plan, err := migrate.Plan(tv.V, testvault.Now)
	if err != nil {
		t.Fatal(err)
	}
	tv.Clean()
	if plan.Documents != 12 || plan.Assets != 3 || len(plan.Inbox) != 1 || plan.Events < 6 {
		t.Fatalf("plan %+v", plan)
	}
	tagsOf := map[string]string{}
	for _, m := range plan.Tags {
		tagsOf[m.Scope] = m.Tag
	}
	if tagsOf["work"] != "work" || tagsOf["p3"] != "work/p3" || tagsOf["p3-edge"] != "work/p3/p3-edge" {
		t.Fatalf("tags %+v", plan.Tags)
	}
	tv.Write(vault.PluginDir+"/manifest.json", "{\n  \"id\": \"atlas\",\n  \"version\": \"6.5.0\"\n}\n")
	tv.Commit()
	report, err := migrate.Run(tv.V, testvault.Now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if got := tv.V.InstalledPluginVersion(); got != vault.PluginVersion() || report.Plugin != got {
		t.Fatalf("the Obsidian plugin: %q, reported %q", got, report.Plugin)
	}
	if log := tv.Log(); log[0] != "layout: migrate to 7.0" {
		t.Fatalf("log %v", log)
	}
	tv.Clean()
	atlas := tv.Read("Atlas.md")
	for _, want := range []string{"layout: 3", "tagging: open", "wikify: [source, spec, event]", "The vault's own context."} {
		if !strings.Contains(atlas, want) || strings.Contains(atlas, "areas:") {
			t.Fatalf("Atlas.md lacks %q:\n%s", want, atlas)
		}
	}
	for _, gone := range []string{"threads", "wiki/work", "wiki/policies", "wiki/sources", "wiki/Wiki.base"} {
		if tv.V.Exists(gone) {
			t.Errorf("%s is still there", gone)
		}
	}
	for _, there := range []string{"wiki/assets/src-aaaaaa.pdf", "wiki/assets/src-bbbbbb.md", "wiki/assets/diagram.png", "inbox/from 6.x/wiki/work/p3/My note.md", "scratchpad/from 6.x/Threads.canvas", "views/View · Home.md"} {
		if !tv.V.Exists(there) {
			t.Errorf("%s is missing", there)
		}
	}
	checks := map[string][]string{
		"work":           {"id: are-work01", "type: topic", "kind: overview", "defines: work", "## Summary\n\nAll work.", "## Context\n\nThe work area.", "> [!overview]"},
		"p3":             {"defines: work/p3", "tags: [work]", "What an agent must know about p3."},
		"p3-edge":        {"id: rep-edge01", "type: repository", "defines: work/p3/p3-edge", "tags: [work/p3]", "## What it is"},
		"Motion scoring": {"id: con-motion", "kind: concept", "tags: [work/p3, vision]"},
		"Radar":          {"kind: entity", "tags: [work/p3/p3-edge, tool]"},
		"Pin deps":       {"kind: policy", "strength: must", "tags: []"},
		"DINOv2":         {"type: source", "tags: [work/p3]", "media: pdf", "status: absorbed", "![[src-aaaaaa.pdf]]"},
		"Idea":           {"type: stub", "priority: low", "## Idea\n\nTry a smaller backbone. Soon.", "## Notes\n\nmine", "status: open"},
		"Filter alarms":  {"id: thr-filter", "type: spec", "kind: plan", "tags: [work/p3/p3-edge]", "repositories: [\"[[p3-edge]]\"]", "priority: high", "## Goal\n\nFewer false alarms", "## Origin\n\nCut the false alarms.", "## Parts", "status: open", "blocked: waiting on data"},
		"Score boxes":    {"id: tsk-score1", "parent: \"[[Filter alarms]]\"", "## Goal\n\nScore boxes by motion.", "## Progress", "status: done"},
		"Tune threshold": {"depends: [\"[[Score boxes]]\"]", "status: open"},
		"Killed":         {"type: spec", "status: dropped"},
	}
	for title, wants := range checks {
		got := tv.Read("wiki/documents/" + title + ".md")
		for _, w := range wants {
			if !strings.Contains(got, w) {
				t.Errorf("%s lacks %q:\n%s", title, w, got)
			}
		}
	}
	if strings.Contains(tv.Read("wiki/documents/DINOv2.md"), "sources:") {
		t.Error("an empty 6.x sources field goes")
	}
	if strings.Contains(tv.Read("wiki/documents/Score boxes.md"), "## Result") {
		t.Error("a task's result goes to its completed event")
	}
	if got := tv.Read("Ideas.md"); !strings.Contains(got, "[[Score boxes]]") || !strings.Contains(got, "[[Filter alarms|the spec]]") || strings.Contains(got, "Killed — Receipt") {
		t.Errorf("links follow: %s", got)
	}
	session := tv.Read("sessions/2026-09/2026-09-08 0900 bbbbbb.md")
	if !strings.Contains(session, `work: ["[[Filter alarms]]"]`) || !strings.Contains(session, `specs: ["[[Score boxes]]"]`) || strings.Contains(session, "threads:") {
		t.Errorf("session:\n%s", session)
	}
	change := tv.Read("changes/2026-09/2026-09-10 Learn from the spec.md")
	if !strings.Contains(change, `work: "[[Filter alarms]]"`) || !strings.Contains(change, "see [[Filter alarms — Spec]]") {
		t.Errorf("a change's Writes stay as they were:\n%s", change)
	}
	idx := tv.Index()
	b := work.Load(idx)
	root, _ := idx.Resolve("Filter alarms")
	if b.Status(root) != work.Open || b.Blocked(root) != "waiting on data" {
		t.Errorf("the reopened plan is open and blocked: %s %q", b.Status(root), b.Blocked(root))
	}
	var kinds []string
	for _, e := range b.Events(root) {
		kinds = append(kinds, e.Str("kind"))
	}
	if strings.Join(kinds, ",") != "started,completed,reopened,blocked" {
		t.Errorf("the root's events: %v", kinds)
	}
	if idx.Pending(root) {
		t.Error("a spec the wiki absorbed is not pending again")
	}
	if src, _ := idx.Resolve("DINOv2"); idx.Pending(src) {
		t.Error("an absorbed source stays absorbed")
	}
	f, err := lint.Run(idx, lint.Options{Now: testvault.Now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range f.Findings {
		if x.Severity == lint.Error {
			t.Errorf("lint after the migration: %s %s: %s", x.Check, x.Doc.Path, x.Message)
		}
	}
	if report.Problems != 0 {
		t.Errorf("problems %d", report.Problems)
	}
	if _, err := migrate.Plan(tv.V, testvault.Now); err == nil {
		t.Fatal("a 7.0 vault has nothing to migrate")
	}
}
