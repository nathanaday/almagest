package hooks_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/change"
	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/hooks"
	"github.com/nathanaday/atlas-obsidian/internal/sessions"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
	"github.com/nathanaday/atlas-obsidian/internal/work"
)

const sid = "a1b2c3d4-5e6f-7a8b-9c0d-000000000001"

type fixture struct {
	t  *testing.T
	tv *testvault.T
}

func setup(t *testing.T) *fixture {
	return &fixture{t: t, tv: testvault.New(t)}
}

func (f *fixture) env() hooks.Env {
	return hooks.Env{Getenv: func(k string) string {
		if k == vault.EnvHome {
			return f.tv.Home.Root
		}
		return ""
	}, Now: func() time.Time { return f.tv.Clock }}
}

// run runs a hook with an event and returns its output.
func (f *fixture) run(command string, event map[string]any) string {
	f.t.Helper()
	if _, ok := event["session_id"]; !ok {
		event["session_id"] = sid
	}
	if _, ok := event["cwd"]; !ok {
		event["cwd"] = f.tv.V.Root
	}
	data, _ := json.Marshal(event)
	var out bytes.Buffer
	if err := hooks.Run(command, bytes.NewReader(data), &out, f.env()); err != nil {
		f.t.Fatalf("%s: %v", command, err)
	}
	return out.String()
}

func bash(command string) map[string]any {
	return map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": command}}
}

func denied(out string) bool { return strings.Contains(out, `"permissionDecision":"deny"`) }

func edit(path, old string) map[string]any {
	return map[string]any{"tool_name": "Edit", "tool_input": map[string]any{"file_path": path, "old_string": old, "new_string": "x"}}
}

func (f *fixture) ok(r *work.Result, err error) *work.Result {
	f.t.Helper()
	if err != nil {
		f.t.Fatal(err)
	}
	return r
}

// bind runs the touched hook for a work call, with the tool's result as the host passes
// it: content blocks that hold the JSON.
func (f *fixture) bind(action string, r *work.Result, input map[string]any) {
	f.t.Helper()
	data, _ := json.Marshal(r)
	if input == nil {
		input = map[string]any{}
	}
	input["action"] = action
	f.run("touched", map[string]any{"tool_name": "mcp__plugin_atlas-obsidian_atlas__work", "tool_input": input, "tool_response": []any{map[string]any{"type": "text", "text": string(data)}}})
}

func TestSessionStartCreatesTheDocumentAndPrintsContext(t *testing.T) {
	f := setup(t)
	f.tv.Write("Atlas.md", f.tv.Read("Atlas.md")+"\nEvery agent reads this.\n")
	out := f.run("session-start", map[string]any{"source": "startup"})
	for _, want := range []string{"atlas: vault Work at", "this session: [[2026-09-27 1432 a1b2c3]]", "Work: none open.", "Inbox: 0 files", "an edit in a repository needs a started plan", "<vault-context>", "Every agent reads this."} {
		if !strings.Contains(out, want) {
			t.Errorf("context lacks %q:\n%s", want, out)
		}
	}
	d := f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md")
	if !strings.Contains(d, "status: running") || !strings.Contains(d, "harness_id: "+sid) || !strings.Contains(d, "> [!session] running") {
		t.Fatalf("session:\n%s", d)
	}
	if out := f.run("session-start", map[string]any{"cwd": f.tv.Dir}); out != "" {
		t.Fatalf("outside a vault a hook says nothing: %q", out)
	}
	f.tv.Write("Atlas.md", strings.Replace(f.tv.Read("Atlas.md"), "layout: 3", "layout: 2", 1))
	if out := f.run("session-start", map[string]any{}); !strings.Contains(out, "vault migrate") {
		t.Fatalf("a 6.x vault names the migration: %s", out)
	}
}

func TestGuardProtectsTheVault(t *testing.T) {
	f := setup(t)
	root := f.tv.V.Root
	f.run("session-start", map[string]any{})
	f.tv.Doc("topic", "Knowledge", map[string]any{"kind": "concept"}, "## Definition\n\nx\n")
	f.tv.Commit()
	r := f.ok(work.Stub(f.tv.V, work.StubIn{Text: "an idea", Title: "Idea"}, work.Opts{Now: f.tv.Clock}))
	f.ok(work.Specs(f.tv.V, work.SpecsIn{Specs: []work.SpecIn{{Title: "Plan", Kind: "plan", Text: "## Goal\n\nx\n"}, {Title: "Part", Kind: "plan", Parent: "Plan", Text: "## Goal\n\ny\n"}}}, work.Opts{Now: f.tv.Clock}))
	stub := root + "/" + r.View.Doc.Path
	plan := root + "/wiki/documents/Plan.md"
	own := root + "/sessions/2026-09/2026-09-27 1432 a1b2c3.md"
	bin := "atlas-" + "obsidian"
	f.tv.Write("sessions/2026-09/2026-09-27 1400 ffffff.md", "---\nid: ses-ffffff\ntype: session\nharness_id: other\n---\n## Description\n")
	cases := []struct {
		name  string
		event map[string]any
		deny  bool
	}{
		{"a new document", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": root + "/wiki/documents/X.md"}}, true},
		{"a relative new document", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": "wiki/documents/X.md"}}, true},
		{"a topic", edit(root+"/wiki/documents/Knowledge.md", "x"), true},
		{"an asset", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": root + "/wiki/assets/x.png"}}, true},
		{"another folder of the wiki", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": root + "/wiki/notes/x.md"}}, true},
		{"a view", edit(root+"/views/View · Home.md", "x"), true},
		{"Atlas.md", edit(root+"/Atlas.md", "Work"), true},
		{"a Base", edit(root+"/sessions/Sessions.base", "filters"), true},
		{"a change document", edit(root+"/changes/2026-09/x.md", "x"), true},
		{"a stub's frontmatter", edit(stub, "priority: normal"), true},
		{"a stub's lead", edit(stub, "> [!stub] Open"), true},
		{"a stub's prose", edit(stub, "## Idea\n\nan idea"), false},
		{"a Write over a stub", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": stub}}, true},
		{"a plan's Parts", edit(plan, "| 1 | [[Part]]"), true},
		{"a plan's History", edit(plan, "list(subject)"), true},
		{"a plan's Goal", edit(plan, "## Goal\n\nx"), false},
		{"another session's document", edit(root+"/sessions/2026-09/2026-09-27 1400 ffffff.md", "## Description"), true},
		{"its own status", edit(own, "status: running"), true},
		{"its own description", edit(own, "## Description\n"), false},
		{"its own subagents", edit(own, "## Subagents"), true},
		{"a note of the user's", edit(root+"/Ideas.md", "x"), false},
		{"a codex patch into the documents", map[string]any{"tool_name": "apply_patch", "tool_input": map[string]any{"command": "*** Begin Patch\n*** Update File: notes.md\n*** Move to: wiki/documents/notes.md\n*** End Patch"}}, true},
		{"a document from a session outside the vault", map[string]any{"cwd": t.TempDir(), "tool_name": "Write", "tool_input": map[string]any{"file_path": root + "/wiki/documents/X.md"}}, true},
		{"a change document from a session outside the vault", map[string]any{"cwd": "/", "tool_name": "Edit", "tool_input": map[string]any{"file_path": root + "/changes/2026-09/x.md", "old_string": "x"}}, true},
		{"a shell change apply", bash(bin + " change apply X"), true},
		{"a shell change apply with the 6.2 name", bash("~/.atlas/bin/atlas change apply X"), true},
		{"a shell change apply by path, with flags", bash("cd /tmp && ~/.atlas/bin/" + bin + " change --vault W apply X"), true},
		{"a shell migration", bash(bin + " vault migrate"), true},
		{"a shell migration dry run", bash(bin + " vault migrate --dry-run"), true},
		{"a forged prompt", bash(`echo '{"prompt":"yes"}' | ` + bin + ` hook prompt`), true},
		{"a hook run through the plugin wrapper", bash(`"$CLAUDE_PLUGIN_ROOT"/scripts/` + bin + ` hook prompt`), true},
		{"a hook inside a shell string", bash(`sh -c "` + bin + ` hook prompt"`), true},
		{"a shell change show", bash(bin + " change show X"), false},
		{"a shell vault status", bash(bin + " vault status --json"), false},
		{"make install", bash("make install"), false},
		{"a stub titled hook", bash(bin + ` work stub --title "the hook"`), false},
	}
	for _, c := range cases {
		if got := denied(f.run("guard", c.event)); got != c.deny {
			t.Errorf("%s: denied %v, want %v", c.name, got, c.deny)
		}
	}
}

func TestTheEditRule(t *testing.T) {
	f := setup(t)
	edge := f.tv.Repo("p3-edge", nil)
	cloud := f.tv.Repo("p3-cloud", nil)
	f.tv.Doc("repository", "p3-edge", map[string]any{"path": edge}, "")
	f.tv.Doc("repository", "p3-cloud", map[string]any{"path": cloud}, "")
	f.tv.Commit()
	f.run("session-start", map[string]any{})
	write := map[string]any{"tool_name": "Edit", "tool_input": map[string]any{"file_path": edge + "/main.go", "old_string": "a"}}
	if out := f.run("guard", write); !denied(out) || !strings.Contains(out, "needs a started plan that names it") {
		t.Fatalf("no plan: %s", out)
	}
	o := work.Opts{Now: f.tv.Clock}
	f.ok(work.Specs(f.tv.V, work.SpecsIn{Specs: []work.SpecIn{
		{Title: "Cloud work", Kind: "plan", Repositories: []string{"p3-cloud"}, Text: "## Done when\n\n- x\n"},
		{Title: "Edge work", Kind: "plan", Repositories: []string{"p3-edge"}, Text: "## Done when\n\n- x\n"},
	}}, o))
	f.bind("start", f.ok(work.Start(f.tv.V, "Cloud work", false, o)), map[string]any{"spec": "Cloud work"})
	if !denied(f.run("guard", write)) {
		t.Fatal("a plan about another repository does not count")
	}
	if out := f.run("guard", map[string]any{"tool_name": "mcp__plugin_atlas-obsidian_atlas__work", "tool_input": map[string]any{"action": "start", "spec": "Cloud work"}}); !denied(out) || !strings.Contains(out, "started in this session already") {
		t.Fatalf("a second start in one session: %s", out)
	}
	f.bind("start", f.ok(work.Start(f.tv.V, "Edge work", false, o)), map[string]any{"spec": "Edge work"})
	if denied(f.run("guard", write)) {
		t.Fatal("a started plan that names the repository covers it")
	}
	session := f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md")
	if !strings.Contains(session, `specs: ["[[Cloud work]]", "[[Edge work]]"]`) || !strings.Contains(session, "events: 2") {
		t.Fatalf("session:\n%s", session)
	}
	edgeDoc := f.tv.Read("wiki/documents/Edge work.md")
	if !strings.Contains(edgeDoc, "active: true") || !strings.Contains(edgeDoc, "active in [[2026-09-27 1432 a1b2c3]]") {
		t.Fatalf("the plan is active:\n%s", edgeDoc)
	}
	f.run("touched", map[string]any{"tool_name": "Bash", "cwd": cloud, "tool_input": map[string]any{"command": "git log --oneline"}})
	if strings.Contains(f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md"), "[[p3-cloud]]") {
		t.Fatal("reading a repository is no edit")
	}
	f.run("touched", map[string]any{"tool_name": "Bash", "cwd": cloud, "tool_input": map[string]any{"command": "cat >> main.go <<'EOF'\nx\nEOF"}})
	if !strings.Contains(f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md"), "[[p3-cloud]]") {
		t.Fatal("a shell write in a repository lands in the session's record")
	}
	f.ok(work.Complete(f.tv.V, "Edge work", work.ResultIn{Delivered: "d", Verified: "v"}, o))
	if !denied(f.run("guard", write)) {
		t.Fatal("a done plan does not count")
	}
}

func TestTouchedBindsWorkEventsAndChanges(t *testing.T) {
	f := setup(t)
	f.run("session-start", map[string]any{})
	r := f.ok(work.Stub(f.tv.V, work.StubIn{Text: "x", Title: "Idea"}, work.Opts{Now: f.tv.Clock}))
	f.bind("stub", r, nil)
	p := f.ok(work.Promote(f.tv.V, work.PromoteIn{Stub: "Idea", Kind: "plan", Text: "## Done when\n\n- y\n"}, work.Opts{Now: f.tv.Clock}))
	f.bind("promote", p, nil)
	session := f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md")
	if !strings.Contains(session, `work: ["[[Idea]]"]`) || !strings.Contains(session, "events: 1") {
		t.Fatalf("session:\n%s", session)
	}
	event := f.tv.Read(p.Events[0].Path)
	if !strings.Contains(event, `session: "[[2026-09-27 1432 a1b2c3]]"`) || !strings.Contains(event, "By the agent in [[2026-09-27 1432 a1b2c3]]") {
		t.Fatalf("the event names its session:\n%s", event)
	}
	pv, err := change.Propose(f.tv.V, change.Plan{Title: "Add A", Writes: []change.Write{{Op: "create", Type: "topic", Kind: "concept", Title: "A", Fields: map[string]any{"description": "a"}}}}, f.tv.Tick(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(pv)
	tool := "mcp__plugin_atlas-obsidian_atlas__change"
	f.run("touched", map[string]any{"tool_name": tool, "tool_input": map[string]any{"action": "propose"}, "tool_response": json.RawMessage(data)})
	if !strings.Contains(f.tv.Read(pv.Ref.Path), `session: "[[2026-09-27 1432 a1b2c3]]"`) {
		t.Fatal("the change names its session")
	}
}

// TestTheGate drives the gate as a session does: the hooks record the proposal's session
// and the user's turns, and the change tool's apply reads them, whatever name the call
// gives the change.
func TestTheGate(t *testing.T) {
	f := setup(t)
	f.run("session-start", map[string]any{})
	propose := func(title string) *change.Preview {
		pv, err := change.Propose(f.tv.V, change.Plan{Title: title, Writes: []change.Write{{Op: "create", Type: "topic", Kind: "concept", Title: title, Fields: map[string]any{"description": "a"}}}}, f.tv.Tick(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		return pv
	}
	bind := func(pv *change.Preview) {
		data, _ := json.Marshal(pv)
		f.run("touched", map[string]any{"tool_name": "mcp__plugin_atlas-obsidian_atlas__change", "tool_input": map[string]any{"action": "propose"}, "tool_response": json.RawMessage(data)})
	}
	apply := func(key string) error {
		_, err := change.Apply(f.tv.V, key, f.tv.Clock, sessions.UserAnswered(f.tv.V))
		return err
	}
	pv := propose("Add A")
	bind(pv)
	title := pv.Ref.Title
	keys := []string{pv.Ref.ID, title, "[[" + title + "]]", title + ".md", "2026-09/" + title, pv.Ref.Path}
	for _, key := range keys {
		if err := apply(key); err == nil || !strings.Contains(err.Error(), "wait for the user's yes") {
			t.Fatalf("apply %q before the user's turn: %v", key, err)
		}
	}
	f.tv.Tick(time.Minute)
	f.run("prompt", map[string]any{"prompt": "<agent-message from=\"a22df3\">\n[Subagent hand-back] …"})
	if err := apply(title + ".md"); err == nil {
		t.Fatal("a subagent's hand-back is not the user's turn")
	}
	f.tv.Tick(time.Minute)
	f.run("prompt", map[string]any{"prompt": "yes"})
	if err := apply(title + ".md"); err != nil {
		t.Fatalf("after the user's turn apply passes: %v", err)
	}

	unbound := propose("Add B")
	f.tv.Tick(time.Minute)
	f.run("prompt", map[string]any{"prompt": "yes"})
	if err := apply(unbound.Ref.ID); err == nil || !strings.Contains(err.Error(), "Apply in Obsidian") {
		t.Fatalf("a change no session proposed: %v", err)
	}
	if _, err := change.Apply(f.tv.V, unbound.Ref.ID, f.tv.Clock, nil); err != nil {
		t.Fatalf("the user's own apply has no gate: %v", err)
	}

	garbled := propose("Add C")
	bind(garbled)
	f.tv.Write(garbled.Ref.Path, doc.SetField(f.tv.Read(garbled.Ref.Path), "proposed", "soon"))
	f.tv.Tick(time.Minute)
	f.run("prompt", map[string]any{"prompt": "yes"})
	if err := apply(garbled.Ref.ID); err == nil {
		t.Fatal("a change whose proposal time does not parse stays shut")
	}
}

func TestReadOnlyAgents(t *testing.T) {
	f := setup(t)
	agent := func(typ string, event map[string]any) map[string]any {
		event["agent_id"] = "9f07d1aa"
		event["agent_type"] = "atlas-obsidian:" + typ
		return event
	}
	cases := []struct {
		name  string
		event map[string]any
		deny  bool
	}{
		{"extract writes", agent("wiki-extract", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": "/tmp/x"}}), true},
		{"draft proposes", agent("wiki-draft", map[string]any{"tool_name": "mcp__plugin_atlas-obsidian_atlas__change", "tool_input": map[string]any{"action": "propose"}}), true},
		{"draft plants a stub", agent("wiki-draft", map[string]any{"tool_name": "mcp__plugin_atlas-obsidian_atlas__work", "tool_input": map[string]any{"action": "stub"}}), true},
		{"draft searches", agent("wiki-draft", map[string]any{"tool_name": "mcp__plugin_atlas-obsidian_atlas__search", "tool_input": map[string]any{"text": "x"}}), false},
		{"review shows work", agent("spec-review", map[string]any{"tool_name": "mcp__plugin_atlas-obsidian_atlas__work", "tool_input": map[string]any{"action": "show"}}), false},
		{"audit runs a shell", agent("wiki-audit", bash("ls")), true},
		{"review reads the log", agent("spec-review", bash("git log --oneline -5")), false},
		{"review reads another repository", agent("spec-review", bash(`git -C "/code/p3 edge" show abc123`)), false},
		{"review chains a command", agent("spec-review", bash("git -C /x log && rm -rf /")), true},
		{"review pipes", agent("spec-review", bash("git log | head")), true},
		{"review writes a file", agent("spec-review", bash("git diff --output=/tmp/x")), true},
		{"review runs other git", agent("spec-review", bash("git commit -m x")), true},
	}
	for _, c := range cases {
		if got := denied(f.run("guard", c.event)); got != c.deny {
			t.Errorf("%s: denied %v, want %v", c.name, got, c.deny)
		}
	}
}

func TestSubagentsAndTheEndOfASession(t *testing.T) {
	f := setup(t)
	repo := f.tv.Repo("p3-edge", nil)
	f.tv.Doc("repository", "p3-edge", map[string]any{"path": repo}, "")
	f.tv.Commit()
	f.run("session-start", map[string]any{})
	o := work.Opts{Now: f.tv.Clock}
	f.ok(work.Specs(f.tv.V, work.SpecsIn{Specs: []work.SpecIn{{Title: "Plan", Kind: "plan", Repositories: []string{"p3-edge"}, Text: "## Done when\n\n- x\n"}}}, o))
	f.bind("start", f.ok(work.Start(f.tv.V, "Plan", false, o)), map[string]any{"spec": "Plan"})
	f.run("subagent-start", map[string]any{"agent_id": "9f07d1aa", "agent_type": "atlas-obsidian:wiki-extract"})
	f.run("touched", map[string]any{"agent_id": "9f07d1aa", "agent_type": "atlas-obsidian:wiki-extract", "tool_name": "Read"})
	f.run("subagent-stop", map[string]any{"agent_id": "9f07d1aa", "agent_type": "atlas-obsidian:wiki-extract"})
	parent := f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md")
	if strings.Count(parent, "wiki-extract") != 1 || !strings.Contains(parent, "· ended 14:32") {
		t.Fatalf("a worker is one line:\n%s", parent)
	}
	f.run("subagent-start", map[string]any{"agent_id": "7e55aa01", "agent_type": "general-purpose"})
	child := f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3 · 7e55aa.md")
	if !strings.Contains(child, `parent: "[[2026-09-27 1432 a1b2c3]]"`) || !strings.Contains(child, `specs: ["[[Plan]]"]`) {
		t.Fatalf("a writing subagent inherits the started plans:\n%s", child)
	}
	write := map[string]any{"agent_id": "7e55aa01", "agent_type": "general-purpose", "tool_name": "Edit", "tool_input": map[string]any{"file_path": repo + "/main.go", "old_string": "a"}}
	if denied(f.run("guard", write)) {
		t.Fatal("the subagent may edit the repository its parent's plan names")
	}
	f.run("notify", map[string]any{"notification_type": "permission_prompt"})
	if !strings.Contains(f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md"), "status: waiting") {
		t.Fatal("waiting")
	}
	f.run("subagent-stop", map[string]any{"agent_id": "7e55aa01", "agent_type": "general-purpose"})
	f.run("session-end", map[string]any{"reason": "exit"})
	if s := f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md"); !strings.Contains(s, "status: ended") {
		t.Fatalf("ended:\n%s", s)
	}
	if !strings.Contains(f.tv.Read("wiki/documents/Plan.md"), "active: false") {
		t.Fatal("an ended session lets the plan go")
	}
}

func TestStopRemindsOnce(t *testing.T) {
	f := setup(t)
	repo := f.tv.Repo("p3-edge", nil)
	f.tv.Doc("repository", "p3-edge", map[string]any{"path": repo}, "")
	f.run("session-start", map[string]any{})
	f.run("touched", map[string]any{"tool_name": "Edit", "tool_input": map[string]any{"file_path": repo + "/x.go"}})
	out := f.run("stop", map[string]any{})
	if !strings.Contains(out, `"decision":"block"`) || !strings.Contains(out, "## Description") {
		t.Fatalf("stop: %s", out)
	}
	if out := f.run("stop", map[string]any{}); out != "" {
		t.Fatalf("once: %s", out)
	}
	if !strings.Contains(f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md"), "status: idle") {
		t.Fatal("idle")
	}
}

func TestPromptNamesTheOpenNote(t *testing.T) {
	f := setup(t)
	f.run("session-start", map[string]any{})
	f.tv.Write("Ideas.md", "x")
	f.tv.Write(".obsidian/workspace.json", `{"main":{"id":"m","type":"leaf","state":{"state":{"file":"Ideas.md"}}},"active":"m"}`)
	if out := f.run("prompt", map[string]any{"prompt": "what is this"}); out != "Open in Obsidian: [[Ideas]]\n" {
		t.Fatalf("prompt: %q", out)
	}
}
