package migrate_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/lint"
	"github.com/nathanaday/atlas-obsidian/internal/migrate"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/thread"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
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
	if _, err := thread.Stub(tv.V, thread.StubIn{Text: "x"}, thread.Opts{Now: testvault.Now}); err == nil || !strings.Contains(err.Error(), "vault migrate") {
		t.Fatalf("a write refuses a 6.x vault: %v", err)
	}
	if plan, err := migrate.Plan(tv.V, testvault.Now); err != nil || plan.From != "6.x" || !strings.Contains(strings.Join(plan.Warnings, " "), "the first step") {
		t.Fatalf("a dry run of a 6.x vault lists the first step: %+v %v", plan, err)
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
	if report.From != "6.x" || report.Chords != 1 || report.Threads != 3 || report.Specs != 1 || report.TaskLists != 1 || report.Verifications != 1 {
		t.Fatalf("report %+v", report)
	}
	if log := tv.Log(); log[0] != "layout: migrate to 8.0" {
		t.Fatalf("log %v", log)
	}
	tv.Clean()
	atlas := tv.Read("Atlas.md")
	for _, want := range []string{"layout: 4", "tagging: open", "wikify: [source, spec, event, verification, chord]", "The vault's own context."} {
		if !strings.Contains(atlas, want) || strings.Contains(atlas, "areas:") {
			t.Fatalf("Atlas.md lacks %q:\n%s", want, atlas)
		}
	}
	for _, gone := range []string{"threads", "wiki/work", "wiki/policies", "wiki/sources", "wiki/Wiki.base"} {
		if tv.V.Exists(gone) {
			t.Errorf("%s is still there", gone)
		}
	}
	for _, there := range []string{"wiki/assets/src-aaaaaa.pdf", "wiki/assets/src-bbbbbb.md", "wiki/assets/diagram.png", "inbox/from 6.x/wiki/work/p3/My note.md", "scratchpad/from 6.x/Threads.canvas", "views/View · Home.md", "views/View · Threads.md", "chords/Filter alarms.canvas"} {
		if !tv.V.Exists(there) {
			t.Errorf("%s is missing", there)
		}
	}
	checks := map[string][]string{
		"work":                          {"id: are-work01", "type: topic", "kind: overview", "defines: work", "## Summary\n\nAll work.", "## Context\n\nThe work area.", "> [!overview]"},
		"p3":                            {"defines: work/p3", "tags: [work]", "What an agent must know about p3."},
		"p3-edge":                       {"id: rep-edge01", "type: repository", "defines: work/p3/p3-edge", "tags: [work/p3]", "## What it is"},
		"Motion scoring":                {"id: con-motion", "kind: concept", "tags: [work/p3, vision]"},
		"Radar":                         {"kind: entity", "tags: [work/p3/p3-edge, tool]"},
		"Pin deps":                      {"kind: policy", "strength: must", "tags: []"},
		"DINOv2":                        {"type: source", "tags: [work/p3]", "media: pdf", "status: absorbed", "![[src-aaaaaa.pdf]]"},
		"Idea":                          {"type: stub", "priority: low", "## Idea\n\nTry a smaller backbone. Soon.", "## Notes\n\nmine", "status: stub", "> [!thread] Stub · low"},
		"Filter alarms":                 {"id: thr-filter", "type: chord", "tags: [work/p3/p3-edge]", "priority: high", "status: started", "threads: 0/2", "## Goal\n\nFewer false alarms on the edge.\n\nDone when:\n\n- half as many alarms", "| 1 | [[Score boxes]] | verified | 1/1 |  | [[p3-edge]] |", "| 2 | [[Tune threshold]] | stub, ready |  | [[Score boxes]] |  |"},
		"Score boxes":                   {"id: tsk-score1", "type: stub", "chord: \"[[Filter alarms]]\"", "status: verified", "tasks: 1/1", "## Idea\n\nScore boxes by motion.", "- Spec: [[Score boxes · Spec]] · 1 requirement · complete (verified)", "> Missing: the wiki change that absorbs it"},
		"Score boxes · Spec":            {"thread: \"[[Score boxes]]\"", "## Goal\n\nScore boxes by motion.", "## Requirements\n\n- R1: The goal above is met."},
		"Score boxes · Tasks (p3-edge)": {"repository: \"[[p3-edge]]\"", "- [x] T1: The work of the 7.x plan (R1) · done before 8.0"},
		"Score boxes · Verification 1":  {"round: 1", "verdict: pass", "by: user", "## Scope\n\nScored in abc123.", "| R1: The goal above is met. | pass | Verified in 7.x: Recorded before 7.0 in the task's result. |", "## Findings\n\nNone."},
		"Tune threshold":                {"type: stub", "after: [\"[[Score boxes]]\"]", "chord: \"[[Filter alarms]]\"", "status: stub", "rank: 1"},
		"Killed":                        {"type: stub", "status: dropped", "> [!thread-dropped] Dropped 2026-09-04", "No longer needed."},
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
	if strings.Contains(tv.Read("wiki/documents/Score boxes.md"), "## Progress") {
		t.Error("a stub holds no progress")
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
	b := thread.Load(idx)
	root, _ := idx.Resolve("Filter alarms")
	if b.Status(root) != thread.ChordStarted || len(b.Members(root)) != 2 || b.ChordNext(root).Step != "work" {
		t.Errorf("the chord: %s, %d threads, next %+v", b.Status(root), len(b.Members(root)), b.ChordNext(root))
	}
	var kinds []string
	for _, e := range b.Events(root) {
		kinds = append(kinds, e.Str("kind"))
	}
	if strings.Join(kinds, ",") != "reopened,note" {
		t.Errorf("the chord's events: %v", kinds)
	}
	if note := tv.Read(b.Events(root)[1].Path); !strings.Contains(note, "The plan was blocked in 7.x: waiting on data.") || !strings.Contains(note, "### Origin (7.x)\n\nCut the false alarms.") {
		t.Errorf("the chord's note keeps what a chord does not hold:\n%s", note)
	}
	score, _ := idx.Resolve("Score boxes")
	kinds = nil
	for _, e := range b.Events(score) {
		kinds = append(kinds, e.Str("kind"))
	}
	if strings.Join(kinds, ",") != "started,note" || !strings.Contains(tv.Read(b.Events(score)[1].Path), "### Progress (7.x)\n\n- 2026-09-07: began") {
		t.Errorf("the thread's events: %v", kinds)
	}
	if tune, _ := idx.Resolve("Tune threshold"); !b.Ready(tune) {
		t.Error("a thread after a verified thread is ready")
	}
	if idx.Pending(root) {
		t.Error("a chord with open threads is not pending")
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
	if _, err := migrate.Plan(tv.V, testvault.Now); err == nil || !strings.Contains(err.Error(), "8.0 layout already") {
		t.Fatalf("an 8.0 vault has nothing to migrate: %v", err)
	}
	tv.Clean()
}

// flat writes a vault as 7.2 left it: plans, a design, and their events in
// wiki/documents.
func flat(t *testing.T) *testvault.T {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", nil)
	tv.Write("Atlas.md", strings.Replace(tv.Read("Atlas.md"), "layout: 4", "layout: 3", 1))
	plan := func(title, id, status, extra, body string) {
		tv.Write("wiki/documents/"+title+".md", "---\nid: "+id+"\ntype: spec\nkind: plan\ndescription: The "+title+" plan.\ntags: [p3/p3-edge]\naliases: []\ncreated: 2026-09-20T10:00:00\nupdated: 2026-09-24T10:00:00\nrefreshed: 2026-09-24T10:00:00\npriority: high\nstatus: "+status+"\nblocked: \"\"\nactive: false\nparts: \"\"\nroot: \"[["+title+"]]\"\nmine: kept\n"+extra+"---\n\n> [!spec] "+status+" · plan\n> **"+title+"**\n\n"+body)
	}
	event := func(subject, plan, id, kind, at, body string) {
		tv.Write("wiki/documents/"+subject+" · "+kind+" "+strings.NewReplacer("-", "-", "T", " ", ":", "").Replace(at)[:15]+".md", "---\nid: "+id+"\ntype: event\nkind: "+kind+"\ndescription: \""+kind+": "+subject+"\"\ntags: [p3/p3-edge]\naliases: []\ncreated: "+at+"\nupdated: "+at+"\nrefreshed: "+at+"\nat: "+at+"\nsubject: \"[["+subject+"]]\"\nsubject_id: "+plan+"\nsession: \"\"\nby: agent\n---\n\n> [!event-"+kind+"] x\n\n"+body)
	}
	tv.Doc("repository", "p3-edge", map[string]any{"path": repo, "defines": "p3/p3-edge", "tags": []string{"p3"}}, "")
	// A done plan the wiki absorbed.
	doneBody := "## Goal\n\nOne model format.\n\n### Why\n\nTwo formats drift.\n\n## Done when\n\n- [x] p3-edge loads TFLite\n  with the NPU delegate\n1. the image ships it\n\n## Decisions\n\n- TFLite, since the NPU needs it.\n\n## Conventions\n\n- [[Pin deps]]: the runtime is pinned.\n\n## Where\n\nloader.go\n\n## Verify\n\ngo test ./...\n\n## Progress\n\n- 2026-09-21: loaded\n\n### A finding\n\nThe delegate needs a flag.\n\n## History\n\n```base\nx\n```\n\n## Origin\n\nUse one format everywhere.\n\n## Notes\n\nmine\n"
	plan("One format", "doc-plan01", "done", "repositories: [\"[[p3-edge]]\"]\n", doneBody)
	event("One format", "doc-plan01", "doc-evnt01", "started", "2026-09-21T09:00:00", "")
	result := "## Delivered\n\nLoader in abc1234.\n\n## Verified\n\ngo test ./... passes.\n\n## Learned\n\nThe delegate needs a flag.\n"
	event("One format", "doc-plan01", "doc-evnt02", "completed", "2026-09-22T09:00:00", result)
	tv.Doc("topic", "Pin deps", map[string]any{"kind": "policy", "strength": "must", "sources": []string{"[[One format]]"}}, "## Rule\n\nPin.\n")
	hash := doc.ProseHash(doc.Parse("x", []byte("---\na: b\n---\n\n> [!spec] done · plan\n> **One format**\n\n"+doneBody)).Body, []string{"Parts", "History", "Implemented by", "Origin"})
	tv.Write("changes/2026-09/2026-09-23 Learn one format.md", "---\nid: chg-learn1\ntype: change\ncreated: 2026-09-23\nupdated: 2026-09-23\nstatus: applied\nabsorbs: [\"[[One format]]\"]\napplied: 2026-09-23T12:00:00\n---\n\n## Notes\n\nx\n\n## Absorbed\n\n| Document | Id | Hash |\n|---|---|---|\n| [[One format]] | doc-plan01 | "+hash[:12]+" |\n\n## Writes\n")
	// A started plan, with a long progress and a section an agent added.
	plan("YOLO study", "doc-plan03", "started", "repositories: [\"[[p3-edge]]\"]\n", "## Goal\n\nMeasure.\n\n## Done when\n\n- the matrix is measured\n- a cell is recommended\n\n## Progress\n\n### First results\n\n| a | b |\n\n## Findings and next steps\n\nINT8 caps the score.\n")
	event("YOLO study", "doc-plan03", "doc-evnt03", "started", "2026-09-24T09:00:00", "")
	// A plan with no Done when list, done and reopened.
	plan("Loose plan", "doc-plan04", "open", "", "## Goal\n\nSomething.\n")
	event("Loose plan", "doc-plan04", "doc-evnt04", "completed", "2026-09-22T10:00:00", "## Delivered\n\nA thing.\n\n## Verified\n\nSeen.\n")
	event("Loose plan", "doc-plan04", "doc-evnt14", "reopened", "2026-09-23T10:00:00", "")
	// A plan that waits on another, and a design.
	plan("After the study", "doc-plan05", "open", "depends: [\"[[YOLO study]]\"]\n", "## Goal\n\nDeploy.\n\n## Done when\n\n- deployed\n")
	tv.Write("wiki/documents/Model contract.md", "---\nid: doc-dsgn01\ntype: spec\nkind: design\ndescription: The model contract.\ntags: [p3]\ncreated: 2026-09-20\nupdated: 2026-09-20\nstatus: current\n---\n\n> [!design] Current · design\n\n## Purpose\n\nOne contract.\n\n## Behavior\n\nThe head emits boxes.\n")
	tv.Doc("stub", "An idea", map[string]any{"status": "open", "priority": "low"}, "> [!stub] Open · low\n\n## Idea\n\nLater.\n")
	// A stub an agent filled, and a plan made by hand with no id.
	tv.Doc("stub", "Long stub", map[string]any{"status": "open"}, "> [!stub] Open\n\n## Idea\n\nFlash the image.\n\n## Prepared 2026-09-24\n\nThe card is written.\n\n## Notes\n\nmine\n")
	tv.Write("wiki/documents/Hand made.md", "---\ntype: spec\nkind: plan\nstatus: open\npriority:\n---\n\n> [!spec] Open · plan\n")
	tv.Write("sessions/2026-09/2026-09-24 0900 cccccc.md", "---\nid: ses-cccccc\ntype: session\ncreated: 2026-09-24\nupdated: 2026-09-24T10:00:00\nharness_id: cccccc\nstatus: ended\nspecs: [\"[[YOLO study]]\"]\nwork: [\"[[YOLO study]]\"]\n---\n\n## Description\n\nThe study.\n")
	tv.Write("Ideas.md", "See [[One format · completed 2026-09-22 0900|the result]].\n")
	tv.Commit()
	v, err := vault.Open(tv.V.Root)
	if err != nil {
		t.Fatal(err)
	}
	tv.V = v
	return tv
}

func TestMigrationFromTheFlatLayout(t *testing.T) {
	tv := flat(t)
	plan, err := migrate.Plan(tv.V, testvault.Now)
	if err != nil {
		t.Fatal(err)
	}
	tv.Clean()
	if plan.From != "7.x" || plan.Threads != 5 || plan.Specs != 3 || plan.TaskLists != 1 || plan.Verifications != 1 || plan.Topics != 1 || plan.Chords != 0 || plan.Notes != 5 {
		t.Fatalf("plan %+v", plan)
	}
	report, err := migrate.Run(tv.V, testvault.Now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	tv.Clean()
	checks := map[string][]string{
		"One format": {"id: doc-plan01", "type: stub", "status: closed", "mine: kept", "priority: high", "## Idea\n\nUse one format everywhere.", "## Notes\n\nmine", "> [!thread-closed] Closed", "absorbed by [[2026-09-27 Migrate to 8.0]]",
			"- Spec: [[One format · Spec]] · 2 requirements · complete (verified)", "- Tasks: [[One format · Tasks (p3-edge)]] 1/1", "- Verification: [[One format · Verification 1]] · pass", "- Knowledge: [[Pin deps]]"},
		"One format · Spec": {"type: spec", "tags: [p3/p3-edge]", "## Goal\n\nOne model format.\n\n### Why\n\nTwo formats drift.", "## Requirements\n\n- R1: p3-edge loads TFLite\n  with the NPU delegate\n- R2: the image ships it",
			"## Rules\n\n- [[Pin deps]]: the runtime is pinned.", "## Decisions\n\n- TFLite, since the NPU needs it."},
		"One format · Tasks (p3-edge)": {"- [x] T1: The work of the 7.x plan (R1, R2) · done before 8.0", "### T1\n\n**Where (7.x)**\n\nloader.go\n\n**Verify (7.x)**\n\ngo test ./..."},
		"One format · Verification 1":  {"at: 2026-09-22T09:00:00", "by: agent", "## Scope\n\nLoader in abc1234.", "| R2: the image ships it | pass | Verified in 7.x: go test ./... passes. |", "## Notes\n\n### Verified (7.x)\n\ngo test ./... passes.\n\n### Learned\n\nThe delegate needs a flag."},
		"YOLO study":                   {"type: stub", "status: specified", "## Idea\n\nThe YOLO study plan.", "> Next: the spec has no task list (thread-tasks)"},
		"YOLO study · Spec":            {"- R1: the matrix is measured\n- R2: a cell is recommended"},
		"Loose plan":                   {"type: stub", "status: stub"},
		"After the study":              {"after: [\"[[YOLO study]]\"]", "status: specified", "> After [[YOLO study]] (specified)"},
		"Model contract":               {"type: topic", "kind: concept", "status: draft", "## Definition\n\nOne contract.", "## Explanation\n\n### Behavior\n\nThe head emits boxes."},
		"An idea":                      {"status: stub", "> [!thread] Stub · low"},
		"p3-edge":                      {"## Threads\n\n```base"},
		"Pin deps":                     {"## Threads\n\n```base", "file.hasLink(this.file)"},
	}
	for title, wants := range checks {
		got := tv.Read("wiki/documents/" + title + ".md")
		for _, w := range wants {
			if !strings.Contains(got, w) {
				t.Errorf("%s lacks %q:\n%s", title, w, got)
			}
		}
	}
	for _, gone := range []string{"wiki/documents/One format · completed 2026-09-22 0900.md", "wiki/documents/YOLO study · Tasks (p3-edge).md", "wiki/documents/Loose plan · Spec.md"} {
		if tv.V.Exists(gone) {
			t.Errorf("%s is there", gone)
		}
	}
	if strings.Contains(tv.Read("wiki/documents/Long stub.md"), "Prepared") || strings.Contains(tv.Read("wiki/documents/p3-edge.md"), "## Work") || strings.Contains(tv.Read("wiki/documents/One format.md"), "## Progress") {
		t.Error("the sections of 7.x go")
	}
	if got := tv.Read("Ideas.md"); got != "See [[One format · Verification 1|the result]].\n" {
		t.Errorf("a link to a result follows it: %s", got)
	}
	idx := tv.Index()
	b := thread.Load(idx)
	notes := func(title string) string {
		d, _ := idx.Resolve(title)
		var out []string
		for _, e := range b.Events(d) {
			if e.Str("kind") == "note" {
				out = append(out, tv.Read(e.Path))
			}
		}
		return strings.Join(out, "\n")
	}
	for title, wants := range map[string][]string{
		"One format": {"### Progress (7.x)\n\n- 2026-09-21: loaded\n\n#### A finding\n\nThe delegate needs a flag."},
		"YOLO study": {"Repositories of the 7.x plan: [[p3-edge]].", "### Progress (7.x)\n\n#### First results", "### Findings and next steps (7.x)\n\nINT8 caps the score."},
		"Loose plan": {"kind: note", "The result recorded in 7.x.", "### Delivered\n\nA thing.", "### Goal (7.x)\n\nSomething."},
		"Long stub":  {"keeps a stub short", "### Prepared 2026-09-24 (7.x)\n\nThe card is written."},
	} {
		got := notes(title)
		for _, w := range wants {
			if !strings.Contains(got, w) {
				t.Errorf("the notes of %s lack %q:\n%s", title, w, got)
			}
		}
	}
	study, _ := idx.Resolve("YOLO study")
	if l, err := thread.LoadThread(idx, study.ID()); err != nil || len(l.Notes) != 1 || len(l.Sessions) != 1 || l.Next.Step != "tasks" {
		t.Errorf("a load of a migrated thread carries its note and its session: %+v %v", l, err)
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
	if report.Problems != 0 || report.From != "7.x" {
		t.Errorf("report %+v", report)
	}
	v, _ := vault.Open(tv.V.Root)
	if _, err := thread.Stub(v, thread.StubIn{Text: "a new idea"}, thread.Opts{Now: testvault.Now.Add(2 * time.Hour)}); err != nil {
		t.Fatalf("a migrated vault takes writes: %v", err)
	}
}

// TestMigrateACopy migrates the vault that ATLAS_MIGRATE_COPY names, for a dry run on a
// copy of a real vault: it prints the report and every lint error. It never runs
// without the variable, and it must name a copy.
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
	plan, err := migrate.Plan(v, now)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("plan: %+v", plan)
	report, err := migrate.Run(v, now)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("report: %+v", report)
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
	b := thread.Load(idx)
	for _, s := range b.Stubs {
		t.Logf("thread %-12s %s · missing: %s", b.Status(s), s.Title(), strings.Join(b.Missing(s), "; "))
	}
}
