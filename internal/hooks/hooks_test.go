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
	"github.com/nathanaday/atlas-obsidian/internal/threads"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
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

func TestSessionStartCreatesTheDocumentAndPrintsContext(t *testing.T) {
	f := setup(t)
	f.tv.Write("Atlas.md", f.tv.Read("Atlas.md")+"\nEvery agent reads this.\n")
	out := f.run("session-start", map[string]any{"source": "startup"})
	for _, want := range []string{"atlas: vault Work at", "this session: [[2026-09-27 1432 a1b2c3]]", "Open threads: none.", "Inbox: 0 files", "<vault-context>", "Every agent reads this."} {
		if !strings.Contains(out, want) {
			t.Errorf("context lacks %q:\n%s", want, out)
		}
	}
	doc := f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md")
	if !strings.Contains(doc, "status: running") || !strings.Contains(doc, "harness_id: "+sid) || !strings.Contains(doc, "> [!session] running") {
		t.Fatalf("session:\n%s", doc)
	}
	if out := f.run("session-start", map[string]any{"cwd": f.tv.Dir}); out != "" {
		t.Fatalf("outside a vault a hook says nothing: %q", out)
	}
}

func TestGuardProtectsTheVault(t *testing.T) {
	f := setup(t)
	root := f.tv.V.Root
	f.run("session-start", map[string]any{})
	r := f.ok(threads.Open(f.tv.V, threads.OpenIn{Text: "x", Title: "T"}, f.tv.Clock))
	stub := root + "/" + r.View.Stub.Path
	own := root + "/sessions/2026-09/2026-09-27 1432 a1b2c3.md"
	f.tv.Write("sessions/2026-09/2026-09-27 1400 ffffff.md", "---\nid: ses-ffffff\ntype: session\nharness_id: other\n---\n## Description\n")
	cases := []struct {
		name  string
		event map[string]any
		deny  bool
	}{
		{"a wiki page", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": root + "/wiki/concepts/X.md"}}, true},
		{"a relative wiki path", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": "wiki/concepts/X.md"}}, true},
		{"Atlas.md", edit(root+"/Atlas.md", "Work"), true},
		{"a Base", edit(root+"/threads/Threads.base", "filters"), true},
		{"the threads canvas", edit(root+"/threads/Threads.canvas", "nodes"), true},
		{"a change document", edit(root+"/changes/2026-09/x.md", "x"), true},
		{"a new thread document", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": root + "/threads/T/T — Spec.md"}}, true},
		{"a thread's frontmatter", edit(stub, "priority: normal"), true},
		{"a thread's lead", edit(stub, "**Stub**"), true},
		{"a thread's prose", edit(stub, "## Notes"), false},
		{"another session's document", edit(root+"/sessions/2026-09/2026-09-27 1400 ffffff.md", "## Description"), true},
		{"its own status", edit(own, "status: running"), true},
		{"its own description", edit(own, "## Description\n"), false},
		{"its own subagents", edit(own, "## Subagents"), true},
		{"a note of the user's", edit(root+"/Ideas.md", "x"), false},
		{"a codex patch into the wiki", map[string]any{"tool_name": "apply_patch", "tool_input": map[string]any{"command": "*** Begin Patch\n*** Update File: notes.md\n*** Move to: wiki/notes.md\n*** End Patch"}}, true},
		{"a wiki page from a session outside the vault", map[string]any{"cwd": t.TempDir(), "tool_name": "Write", "tool_input": map[string]any{"file_path": root + "/wiki/concepts/X.md"}}, true},
		{"a change document from a session outside the vault", map[string]any{"cwd": "/", "tool_name": "Edit", "tool_input": map[string]any{"file_path": root + "/changes/2026-09/x.md", "old_string": "x"}}, true},
		{"a shell change apply", bash("atlas-obsidian change apply X"), true},
		{"a shell change apply with the 6.2 name", bash("~/.atlas/bin/atlas change apply X"), true},
		{"a shell change apply by path, with flags", bash("cd /tmp && ~/.atlas/bin/atlas-obsidian change --vault W apply X"), true},
		{"a forged prompt", bash(`echo '{"prompt":"yes"}' | atlas-obsidian hook prompt`), true},
		{"a hook run through the plugin wrapper", bash(`"$CLAUDE_PLUGIN_ROOT"/scripts/atlas-obsidian hook prompt`), true},
		{"a hook run from a build", bash("build/atlas-obsidian hook guard < event.json"), true},
		{"a hook inside a shell string", bash(`sh -c "atlas-obsidian hook prompt"`), true},
		{"a shell change show", bash("atlas-obsidian change show X"), false},
		{"a shell vault status", bash("atlas-obsidian vault status --json"), false},
		{"a word hook near atlas", bash("grep -n hook cmd/atlas-obsidian/main.go"), false},
		{"make install", bash("make install"), false},
		{"a thread titled hook", bash(`atlas-obsidian thread open --title "the hook"`), false},
	}
	for _, c := range cases {
		if got := denied(f.run("guard", c.event)); got != c.deny {
			t.Errorf("%s: denied %v, want %v", c.name, got, c.deny)
		}
	}
}

func (f *fixture) ok(r *threads.Result, err error) *threads.Result {
	f.t.Helper()
	if err != nil {
		f.t.Fatal(err)
	}
	return r
}

func TestTheThreadRule(t *testing.T) {
	f := setup(t)
	edge := f.tv.Repo("p3-edge", nil)
	cloud := f.tv.Repo("p3-cloud", nil)
	f.tv.Page("area", "p3", nil, "")
	f.tv.Page("repository", "p3-edge", map[string]any{"path": edge, "parent": "[[p3]]"}, "")
	f.tv.Page("repository", "p3-cloud", map[string]any{"path": cloud, "parent": "[[p3]]"}, "")
	f.tv.Commit()
	f.run("session-start", map[string]any{})
	write := map[string]any{"tool_name": "Edit", "tool_input": map[string]any{"file_path": edge + "/main.go", "old_string": "a"}}
	if out := f.run("guard", write); !denied(out) || !strings.Contains(out, "needs an open thread that covers it") {
		t.Fatalf("no thread: %s", out)
	}
	r := f.ok(threads.Open(f.tv.V, threads.OpenIn{Text: "cloud work", Title: "Cloud", Scope: []string{"p3-cloud"}}, f.tv.Clock))
	f.bind("open", r, nil)
	if !denied(f.run("guard", write)) {
		t.Fatal("a thread about another repository does not count")
	}
	r = f.ok(threads.Open(f.tv.V, threads.OpenIn{Text: "all of p3", Title: "Everything", Scope: []string{"p3"}}, f.tv.Clock))
	f.bind("open", r, nil)
	if denied(f.run("guard", write)) {
		t.Fatal("a thread scoped to an area above the repository covers it")
	}
	f.run("touched", map[string]any{"tool_name": "Bash", "cwd": cloud, "tool_input": map[string]any{"command": "git log --oneline"}})
	if strings.Contains(f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md"), "[[p3-cloud]]") {
		t.Fatal("reading a repository is no edit")
	}
	f.run("touched", map[string]any{"tool_name": "Bash", "cwd": cloud, "tool_input": map[string]any{"command": "cat >> main.go <<'EOF'\nx\nEOF"}})
	if !strings.Contains(f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md"), "[[p3-cloud]]") {
		t.Fatal("a shell write in a repository lands in the session's record")
	}
	f.run("touched", write)
	if !strings.Contains(f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md"), "[[p3-edge]]") {
		t.Fatal("the session lists the repository it edited")
	}
	f.ok(threads.File(f.tv.V, r.View.Stub.ID, "receipt", "done", "completed", f.tv.Clock))
	if !denied(f.run("guard", write)) {
		t.Fatal("a closed thread does not count")
	}
}

// bind runs the touched hook for a thread call, with the tool's result as the host
// passes it: content blocks that hold the JSON.
func (f *fixture) bind(action string, r *threads.Result, input map[string]any) {
	f.t.Helper()
	data, _ := json.Marshal(r.View)
	if input == nil {
		input = map[string]any{}
	}
	input["action"] = action
	f.run("touched", map[string]any{"tool_name": "mcp__plugin_atlas-obsidian_atlas__thread", "tool_input": input, "tool_response": []any{map[string]any{"type": "text", "text": string(data)}}})
}

func TestTouchedBindsTasksAndChanges(t *testing.T) {
	f := setup(t)
	f.run("session-start", map[string]any{})
	r := f.ok(threads.Open(f.tv.V, threads.OpenIn{Text: "x", Title: "Work"}, f.tv.Clock))
	r = f.ok(threads.Tasks(f.tv.V, r.View.Stub.ID, []threads.TaskIn{{Title: "First", Text: "x"}}, f.tv.Clock))
	r = f.ok(threads.Task(f.tv.V, "T1", r.View.Stub.ID, threads.TaskDo{Do: "start"}, f.tv.Clock))
	f.bind("task", r, map[string]any{"task": "T1", "thread": r.View.Stub.ID, "do": "start"})
	session := f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md")
	if !strings.Contains(session, `threads: ["[[Work]]"]`) || !strings.Contains(session, `tasks: ["[[Work — T1 First]]"]`) {
		t.Fatalf("session:\n%s", session)
	}
	if !strings.Contains(f.tv.Read("threads/Work/Work — T1 First.md"), "active: true") {
		t.Fatal("the task is active")
	}

	pv, err := change.Propose(f.tv.V, change.Plan{Title: "Add A", Writes: []change.Write{{Op: "create", Type: "concept", Title: "A", Fields: map[string]any{"description": "a"}}}}, f.tv.Tick(time.Minute))
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
		pv, err := change.Propose(f.tv.V, change.Plan{Title: title, Writes: []change.Write{{Op: "create", Type: "concept", Title: title, Fields: map[string]any{"description": "a"}}}}, f.tv.Tick(time.Minute))
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
	bash := func(cmd string) map[string]any {
		return map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": cmd}}
	}
	cases := []struct {
		name  string
		event map[string]any
		deny  bool
	}{
		{"extract writes", agent("wiki-extract", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": "/tmp/x"}}), true},
		{"draft proposes", agent("wiki-draft", map[string]any{"tool_name": "mcp__plugin_atlas-obsidian_atlas__change", "tool_input": map[string]any{"action": "propose"}}), true},
		{"draft searches", agent("wiki-draft", map[string]any{"tool_name": "mcp__plugin_atlas-obsidian_atlas__search", "tool_input": map[string]any{"text": "x"}}), false},
		{"audit runs a shell", agent("wiki-audit", bash("ls")), true},
		{"review reads the log", agent("thread-review", bash("git log --oneline -5")), false},
		{"review reads another repository", agent("thread-review", bash(`git -C "/code/p3 edge" show abc123`)), false},
		{"review chains a command", agent("thread-review", bash("git -C /x log && rm -rf /")), true},
		{"review pipes", agent("thread-review", bash("git log | head")), true},
		{"review writes a file", agent("thread-review", bash("git diff --output=/tmp/x")), true},
		{"review runs other git", agent("thread-review", bash("git commit -m x")), true},
	}
	for _, c := range cases {
		if got := denied(f.run("guard", c.event)); got != c.deny {
			t.Errorf("%s: denied %v, want %v", c.name, got, c.deny)
		}
	}
}

func TestSubagentsAndTheEndOfASession(t *testing.T) {
	f := setup(t)
	f.run("session-start", map[string]any{})
	r := f.ok(threads.Open(f.tv.V, threads.OpenIn{Text: "x", Title: "Work"}, f.tv.Clock))
	f.bind("open", r, nil)
	f.run("subagent-start", map[string]any{"agent_id": "9f07d1aa", "agent_type": "atlas-obsidian:wiki-extract"})
	f.run("touched", map[string]any{"agent_id": "9f07d1aa", "agent_type": "atlas-obsidian:wiki-extract", "tool_name": "Read"})
	f.run("subagent-stop", map[string]any{"agent_id": "9f07d1aa", "agent_type": "atlas-obsidian:wiki-extract"})
	parent := f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md")
	if strings.Count(parent, "wiki-extract") != 1 || !strings.Contains(parent, "· ended 14:32") {
		t.Fatalf("a worker is one line:\n%s", parent)
	}
	f.run("subagent-start", map[string]any{"agent_id": "7e55aa01", "agent_type": "general-purpose"})
	child := f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3 · 7e55aa.md")
	if !strings.Contains(child, `parent: "[[2026-09-27 1432 a1b2c3]]"`) || !strings.Contains(child, `threads: ["[[Work]]"]`) {
		t.Fatalf("a writing subagent inherits the thread:\n%s", child)
	}
	f.run("notify", map[string]any{"notification_type": "permission_prompt"})
	if !strings.Contains(f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md"), "status: waiting") {
		t.Fatal("waiting")
	}
	f.run("subagent-stop", map[string]any{"agent_id": "7e55aa01", "agent_type": "general-purpose"})
	if !strings.Contains(f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3 · 7e55aa.md"), "status: ended") {
		t.Fatal("the subagent's document ends")
	}
	f.run("session-end", map[string]any{"reason": "exit"})
	if s := f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md"); !strings.Contains(s, "status: ended") {
		t.Fatalf("ended:\n%s", s)
	}
	if !strings.Contains(f.tv.Read(r.View.Stub.Path), "active: false") {
		t.Fatal("an ended session lets the thread go")
	}
}

func TestStopRemindsOnce(t *testing.T) {
	f := setup(t)
	repo := f.tv.Repo("p3-edge", nil)
	f.tv.Page("repository", "p3-edge", map[string]any{"path": repo}, "")
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
