package hooks_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/change"
	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/hooks"
	"github.com/nathanaday/atlas-obsidian/internal/sessions"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
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

func editNew(path, old, new string) map[string]any {
	return map[string]any{"tool_name": "Edit", "tool_input": map[string]any{"file_path": path, "old_string": old, "new_string": new}}
}

func TestSessionStartCreatesTheDocumentAndPrintsContext(t *testing.T) {
	f := setup(t)
	f.tv.Write("Atlas.md", f.tv.Read("Atlas.md")+"\nEvery agent reads this.\n")
	out := f.run("session-start", map[string]any{"source": "startup"})
	for _, want := range []string{"atlas: vault Work at", "this session: [[2026-09-27 1432 a1b2c3]]", "Ingest: 0 files", "Rules: knowledge changes only through a change. Edit a linked repository directly; on long work, add a dated line to ## Progress in this session's document.", "<vault-context>", "Every agent reads this."} {
		if !strings.Contains(out, want) {
			t.Errorf("context lacks %q:\n%s", want, out)
		}
	}
	for _, gone := range []string{"Threads:", "Resume Atlas"} {
		if strings.Contains(out, gone) {
			t.Errorf("context holds %q:\n%s", gone, out)
		}
	}
	d := f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md")
	if !strings.Contains(d, "status: running") || !strings.Contains(d, "harness_id: "+sid) || !strings.Contains(d, "> [!session] running") {
		t.Fatalf("session:\n%s", d)
	}
	if out := f.run("session-start", map[string]any{"cwd": f.tv.Dir}); out != "" {
		t.Fatalf("outside a vault a hook says nothing: %q", out)
	}
	f.tv.Write("Atlas.md", strings.Replace(f.tv.Read("Atlas.md"), "layout: 6", "layout: 5", 1))
	if out := f.run("session-start", map[string]any{}); !strings.Contains(out, "vault migrate") {
		t.Fatalf("a vault of an earlier layout names the migration: %s", out)
	}
}

func TestGuardProtectsTheVault(t *testing.T) {
	f := setup(t)
	root := f.tv.V.Root
	f.run("session-start", map[string]any{})
	f.tv.Doc("topic", "Knowledge", map[string]any{"kind": "concept"}, "## Definition\n\nx\n")
	f.tv.Doc("source", "Paper", map[string]any{"sha256": "abcdef0123456789", "file": "[[x.pdf]]"}, "")
	f.tv.Doc("spec", "Old · Spec", map[string]any{"thread": "[[Old]]"}, "## Goal\n\nx\n")
	f.tv.Write("threads/Plan · Spec.md", "---\nid: doc-pl0001\ntype: spec\n---\n## Goal\n\nx\n")
	f.tv.Commit()
	own := root + "/sessions/2026-09/2026-09-27 1432 a1b2c3.md"
	bin := "atlas-" + "obsidian"
	f.tv.Write("sessions/2026-09/2026-09-27 1400 ffffff.md", "---\nid: ses-ffffff\ntype: session\nharness_id: other\n---\n## Description\n")
	cases := []struct {
		name  string
		event map[string]any
		deny  bool
	}{
		{"a new document", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": root + "/source-core/documents/X.md"}}, true},
		{"a relative new document", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": "source-core/documents/X.md"}}, true},
		{"a topic", edit(root+"/source-core/documents/Knowledge.md", "x"), true},
		{"an asset", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": root + "/source-core/originals/x.png"}}, true},
		{"another folder of the wiki", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": root + "/source-core/notes/x.md"}}, true},
		{"a view", edit(root+"/wiki-view/View · Home.md", "x"), true},
		{"the trash", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": root + "/trash/2026-10-06/x.md"}}, true},
		{"Atlas.md", edit(root+"/Atlas.md", "Work"), true},
		{"a Base", edit(root+"/sessions/Sessions.base", "filters"), true},
		{"a change document", edit(root+"/changes/2026-09/x.md", "x"), true},
		{"a Write over a topic", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": root + "/source-core/documents/Knowledge.md"}}, true},
		{"a source", edit(root+"/source-core/documents/Paper.md", "abcdef0123456789"), true},
		{"a codex patch into a topic", map[string]any{"tool_name": "apply_patch", "tool_input": map[string]any{"command": "*** Begin Patch\n*** Update File: source-core/documents/Knowledge.md\n@@\n+## Findings\n+- found a thing\n*** End Patch"}}, true},
		{"a document of an unknown type", edit(root+"/source-core/documents/Old · Spec.md", "## Goal\n\nx"), false},
		{"a note in the threads archive", edit(root+"/threads/Plan · Spec.md", "## Goal\n\nx"), false},
		{"a new note in the threads archive", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": root + "/threads/Plan · Notes.md"}}, false},
		{"another session's document", edit(root+"/sessions/2026-09/2026-09-27 1400 ffffff.md", "## Description"), true},
		{"its own status", edit(own, "status: running"), true},
		{"its own lead", edit(own, "> [!session] running"), true},
		{"its own description", edit(own, "## Description\n"), false},
		{"its own subagents", edit(own, "## Subagents"), true},
		{"a note of the user's", edit(root+"/Ideas.md", "x"), false},
		{"a codex patch into the documents", map[string]any{"tool_name": "apply_patch", "tool_input": map[string]any{"command": "*** Begin Patch\n*** Update File: notes.md\n*** Move to: source-core/documents/notes.md\n*** End Patch"}}, true},
		{"a document from a session outside the vault", map[string]any{"cwd": t.TempDir(), "tool_name": "Write", "tool_input": map[string]any{"file_path": root + "/source-core/documents/X.md"}}, true},
		{"a change document from a session outside the vault", map[string]any{"cwd": "/", "tool_name": "Edit", "tool_input": map[string]any{"file_path": root + "/changes/2026-09/x.md", "old_string": "x"}}, true},
		{"a shell change apply", bash(bin + " change apply X"), true},
		{"a shell safe delete", bash(bin + " vault trash Notes.md"), true},
		{"a shell snapshot", bash(bin + " vault snapshot"), false},
		{"a shell change apply with the 6.2 name", bash("~/.atlas/bin/atlas change apply X"), true},
		{"a shell change apply by path, with flags", bash("cd /tmp && ~/.atlas/bin/" + bin + " change --vault W apply X"), true},
		{"a shell migration", bash(bin + " vault migrate"), true},
		{"a shell migration dry run", bash(bin + " vault migrate --dry-run"), true},
		{"a forged prompt", bash(`echo '{"prompt":"yes"}' | ` + bin + ` hook prompt`), true},
		{"a hook run through the plugin wrapper", bash(`"$CLAUDE_PLUGIN_ROOT"/scripts/` + bin + ` hook prompt`), true},
		{"a hook inside a shell string", bash(`sh -c "` + bin + ` hook prompt"`), true},
		{"a hook with a backslash in its name", bash(bin + ` ho\ok prompt`), true},
		{"a change apply behind a backslash", bash(`\` + bin + ` change apply X`), true},
		{"a change apply by a path in upper case", bash("/USERS/X/.ATLAS/BIN/" + strings.ToUpper(bin) + " change apply X"), true},
		{"a migration after an option", bash(bin + " vault --json migrate"), true},
		{"a migration after the vault option", bash(bin + " vault --vault W migrate"), true},
		{"a migration after --", bash(bin + " vault -- migrate"), true},
		{"a shell search", bash(bin + " search x"), false},
		{"a hook in ANSI-C quotes", bash(bin + ` $'hook' prompt`), true},
		{"a hook with an escaped letter in ANSI-C quotes", bash(bin + ` $'h\x6fok' prompt`), true},
		{"the binary in ANSI-C quotes", bash(`$'` + bin + `' hook prompt`), true},
		{"the binary in locale quotes", bash(`$"` + bin + `" hook prompt`), true},
		{"a redirect glued to the binary", bash(bin + `</dev/null hook prompt`), true},
		{"a backslash-newline before hook", bash(bin + " \\\nhook prompt"), true},
		{"an apply in ANSI-C quotes", bash(bin + ` change $'apply' X`), true},
		{"a search into a file", bash(bin + " search x > out.txt"), false},
		{"both outputs redirected before hook", bash(bin + " &>/dev/null hook prompt"), true},
		{"both outputs appended before hook", bash(bin + " &>>/tmp/log hook prompt"), true},
		{"a double-quoted target with a space", bash(bin + ` >"/tmp/a b" hook prompt`), true},
		{"a single-quoted target with a space", bash(bin + ` > '/tmp/a b' hook prompt`), true},
		{"an ANSI-C target with a space", bash(bin + ` >$'/tmp/a b' hook prompt`), true},
		{"a here-string before hook", bash(bin + ` <<<'a b' hook prompt`), true},
		{"a zsh unicode escape", bash(bin + ` $'\u68ook' prompt`), true},
		{"a long unicode escape", bash(bin + ` $'\U68ook' prompt`), true},
		{"the binary in a unicode escape", bash(`$'\u61tlas-obsidian' hook prompt`), true},
		{"a named descriptor before hook", bash(bin + ` {fd}>/dev/null hook prompt`), true},
		{"zsh's =command", bash(`=` + bin + ` hook prompt`), true},
		{"a here-string into a search", bash(bin + ` search <<<'x'`), false},
		{"a config set of the terminal command", bash(bin + ` config set terminal_command "kitty sh -lic {command}"`), true},
		{"a global config set of an agent command", bash(bin + ` config set --global agent_commands.claude "x"`), true},
		{"a config set of the terminal", bash(bin + ` config set terminal wezterm`), false},
		{"a config get", bash(bin + ` config get terminal_command`), false},
		// T32: a process substitution runs a command of its own.
		{"a process substitution read", bash(`cat <(` + bin + ` hook prompt)`), true},
		{"a process substitution compared", bash(`diff <(` + bin + ` hook prompt) /dev/null`), true},
		{"a process substitution written", bash(`echo x > >(` + bin + ` hook prompt)`), true},
		{"a process substitution redirected", bash(`cat < <(` + bin + ` hook prompt)`), true},
		{"a process substitution teed", bash(`tee >(` + bin + ` hook prompt)`), true},
		// T33: an escaped space quotes.
		{"eval of escaped spaces", bash(`eval ` + bin + `\ hook\ prompt`), true},
		{"sh -c of escaped spaces", bash(`sh -c ` + bin + `\ hook\ prompt`), true},
		// T34: bash's braced hex escape.
		{"a braced hex escape", bash(bin + ` $'\x{68}ook' prompt`), true},
		// T35: brace expansion.
		{"braces around hook", bash(bin + ` {hook,} prompt`), true},
		{"braces around the binary", bash(`{` + bin + `,hook} prompt`), true},
		{"braces in an echo", bash(`echo {a,b} ` + bin + `-notes`), false},
		// T38: quoted text and heredocs are commands only when a shell or eval runs them.
		{"a grep for the hook command", bash(`grep -rn "` + bin + ` hook" internal/`), false},
		{"a commit message naming the migration", bash(`git commit -m "run ` + bin + ` vault migrate by hand"`), false},
		{"a commit message in a heredoc", bash("git commit -F - <<'EOF'\nrun " + bin + " vault migrate by hand\nEOF"), false},
		{"a heredoc fed to bash", bash("bash <<'EOF'\n" + bin + " hook prompt\nEOF"), true},
		{"eval of a quoted command", bash(`eval "` + bin + ` hook prompt"`), true},
		{"sudo sh -c", bash(`sudo sh -c "` + bin + ` hook prompt"`), true},
		{"xargs sh -c", bash(`echo x | xargs sh -c "` + bin + ` hook prompt"`), true},
		{"a command after a heredoc", bash("cat <<EOF\ntext\nEOF\n" + bin + " hook prompt"), true},
		// T40: unset counts as set.
		{"a config unset of the terminal command", bash(bin + ` config unset terminal_command`), true},
		{"a config unset of an agent command", bash(bin + ` config unset agent_commands.claude`), true},
		{"a config unset of the terminal", bash(bin + ` config unset terminal`), false},
		// T42-T46 (round 4).
		{"env -S with a quoted command", bash(`env -S "` + bin + ` hook prompt"`), true},
		{"env with a variable", bash(`env FOO=1 go test ./...`), false},
		{"a runner in upper case", bash(`SH -c "` + bin + ` hook prompt"`), true},
		{"a runner as zsh =name", bash(`=sh -c "` + bin + ` hook prompt"`), true},
		{"a heredoc piped into sh", bash("cat <<'E' | sh\n" + bin + " hook prompt\nE"), true},
		{"echo piped into sh", bash(`echo "` + bin + ` hook prompt" | sh`), true},
		{"printf piped into bash", bash(`printf '%s' "` + bin + ` hook prompt" | bash`), true},
		{"a letter range", bash(bin + ` {h..h}ook prompt`), true},
		{"a brace list with a quoted member", bash(bin + ` {"hook",} prompt`), true},
		{"a number range in an echo", bash(`echo {1..3}`), false},
		{"a huge range", bash(`echo {1..1000}{1..1000}`), false},
		// T47, T48 (round 5): a runner counts where a command name stands, for its own pipeline.
		{"env before a commit message", bash(`env GIT_AUTHOR_DATE=2026-10-02 git commit -m "Refuse ` + bin + ` hook prompt"`), false},
		{"bash before a commit message", bash(`bash scripts/check.sh && git commit -m "Refuse ` + bin + ` change apply"`), false},
		{"a commit heredoc before ssh", bash("git commit -F - <<'EOF'\nRefuse " + bin + " hook prompt\nEOF\nssh build-host ls"), false},
		{"a grep before npm run watch", bash(`grep -rn "` + bin + ` hook" internal/; npm run watch`), false},
		{"go env before a commit message", bash(`go env GOPATH && git commit -m "Refuse ` + bin + ` hook prompt"`), false},
		{"a commit message naming /bin/sh", bash(`git commit -m "Refuse ` + bin + ` hook prompt under /bin/sh"`), false},
		{"timeout before sh -c", bash(`timeout 5 sh -c "` + bin + ` hook prompt"`), true},
		{"xargs with an option value before sh -c", bash(`echo x | xargs -I {} sh -c "` + bin + ` hook prompt"`), true},
		{"find -exec sh -c", bash(`find . -exec sh -c "` + bin + ` hook prompt" \;`), true},
		{"a shell change show", bash(bin + " change show X"), false},
		{"a shell vault status", bash(bin + " vault status --json"), false},
		{"make install", bash("make install"), false},
		{"a search for hook", bash(bin + ` search "the hook"`), false},
	}
	for _, c := range cases {
		if got := denied(f.run("guard", c.event)); got != c.deny {
			t.Errorf("%s: denied %v, want %v", c.name, got, c.deny)
		}
	}
}

func TestAnEditInALinkedRepositoryNeedsNoThread(t *testing.T) {
	f := setup(t)
	edge := f.tv.Repo("p3-edge", nil)
	cloud := f.tv.Repo("p3-cloud", nil)
	f.tv.Doc("repository", "p3-edge", map[string]any{"path": edge}, "")
	f.tv.Doc("repository", "p3-cloud", map[string]any{"path": cloud}, "")
	f.tv.Commit()
	f.run("session-start", map[string]any{})
	own := "sessions/2026-09/2026-09-27 1432 a1b2c3.md"
	write := edit(edge+"/main.go", "a")
	if out := f.run("guard", write); denied(out) {
		t.Fatalf("an edit in a linked repository from a session that started no thread: %s", out)
	}
	if out := f.run("guard", map[string]any{"cwd": t.TempDir(), "tool_name": "Edit", "tool_input": map[string]any{"file_path": filepath.Join(cloud, "README.md"), "old_string": "a"}}); denied(out) {
		t.Fatalf("an edit in a linked repository from a session outside every vault: %s", out)
	}
	f.run("touched", write)
	if !strings.Contains(f.tv.Read(own), "[[p3-edge]]") {
		t.Fatal("an edit in a repository lands in the session's record")
	}
	f.run("touched", map[string]any{"tool_name": "Bash", "cwd": cloud, "tool_input": map[string]any{"command": "git log --oneline"}})
	if strings.Contains(f.tv.Read(own), "[[p3-cloud]]") {
		t.Fatal("reading a repository is no edit")
	}
	f.run("touched", map[string]any{"tool_name": "Bash", "cwd": cloud, "tool_input": map[string]any{"command": "cat >> main.go <<'EOF'\nx\nEOF"}})
	if !strings.Contains(f.tv.Read(own), "[[p3-cloud]]") {
		t.Fatal("a shell write in a repository lands in the session's record")
	}
}

func TestTouchedBindsChanges(t *testing.T) {
	f := setup(t)
	f.run("session-start", map[string]any{})
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
	if !strings.Contains(f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md"), "[["+pv.Ref.Title+"]]") {
		t.Fatal("the session lists its change")
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
	if err := apply(unbound.Ref.ID); err == nil || !strings.Contains(err.Error(), "presses Approve in the change document") {
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

	// A change with no writes needs no answer.
	f.tv.Doc("source", "Paper", map[string]any{"sha256": "abcdef0123456789", "file": "[[x.pdf]]"}, "")
	f.tv.Commit()
	empty := func(title string, absorbs ...string) *change.Preview {
		pv, err := change.Propose(f.tv.V, change.Plan{Title: title, Notes: "Nothing new.", Absorbs: absorbs}, f.tv.Tick(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		bind(pv)
		return pv
	}
	if err := apply(empty("Absorb the paper", "Paper").Ref.ID); err != nil {
		t.Fatalf("a change that absorbs a source and writes nothing needs no answer: %v", err)
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
		{"draft searches", agent("wiki-draft", map[string]any{"tool_name": "mcp__plugin_atlas-obsidian_atlas__search", "tool_input": map[string]any{"text": "x"}}), false},
		{"draft calls an action nobody listed", agent("wiki-draft", map[string]any{"tool_name": "mcp__plugin_atlas-obsidian_atlas__change", "tool_input": map[string]any{"action": "rewrite"}}), true},
		{"draft calls a tool nobody listed", agent("wiki-draft", map[string]any{"tool_name": "mcp__atlas__purge", "tool_input": map[string]any{}}), true},
		{"draft calls a tool that left", agent("wiki-draft", map[string]any{"tool_name": "mcp__plugin_atlas-obsidian_atlas__thread", "tool_input": map[string]any{"action": "load"}}), true},
		{"draft reads the vault's status", agent("wiki-draft", map[string]any{"tool_name": "mcp__atlas__vault", "tool_input": map[string]any{"action": "status"}}), false},
		{"draft shows a change", agent("wiki-draft", map[string]any{"tool_name": "mcp__plugin_atlas-obsidian_atlas__change", "tool_input": map[string]any{"action": "show"}}), false},
		{"draft reads a source", agent("wiki-draft", map[string]any{"tool_name": "mcp__plugin_atlas-obsidian_atlas__source", "tool_input": map[string]any{"action": "read"}}), false},
		{"draft captures a source", agent("wiki-draft", map[string]any{"tool_name": "mcp__plugin_atlas-obsidian_atlas__source", "tool_input": map[string]any{"action": "capture"}}), true},
		{"wiki audit runs a shell", agent("wiki-audit", bash("ls")), true},
		{"wiki audit edits a file", agent("wiki-audit", map[string]any{"tool_name": "Edit", "tool_input": map[string]any{"file_path": "/code/p3/main.go"}}), true},
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
	f.run("subagent-start", map[string]any{"agent_id": "9f07d1aa", "agent_type": "atlas-obsidian:wiki-extract"})
	f.run("touched", map[string]any{"agent_id": "9f07d1aa", "agent_type": "atlas-obsidian:wiki-extract", "tool_name": "Read"})
	f.run("subagent-stop", map[string]any{"agent_id": "9f07d1aa", "agent_type": "atlas-obsidian:wiki-extract"})
	parent := f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md")
	if strings.Count(parent, "wiki-extract") != 1 || !strings.Contains(parent, "· ended 14:32") {
		t.Fatalf("a worker is one line:\n%s", parent)
	}
	f.run("subagent-start", map[string]any{"agent_id": "7e55aa01", "agent_type": "general-purpose"})
	child := f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3 · 7e55aa.md")
	if !strings.Contains(child, `parent: "[[2026-09-27 1432 a1b2c3]]"`) || !strings.Contains(child, `agent: general-purpose`) {
		t.Fatalf("a writing subagent gets its own document:\n%s", child)
	}
	write := map[string]any{"agent_id": "7e55aa01", "agent_type": "general-purpose", "tool_name": "Edit", "tool_input": map[string]any{"file_path": repo + "/main.go", "old_string": "a"}}
	if denied(f.run("guard", write)) {
		t.Fatal("the subagent may edit a linked repository")
	}
	f.run("touched", write)
	if !strings.Contains(f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3 · 7e55aa.md"), "[[p3-edge]]") {
		t.Fatal("the subagent's edit lands in its own record")
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
}

func TestStopRemindsOnce(t *testing.T) {
	f := setup(t)
	repo := f.tv.Repo("p3-edge", nil)
	f.tv.Doc("repository", "p3-edge", map[string]any{"path": repo}, "")
	f.run("session-start", map[string]any{})
	f.run("touched", map[string]any{"tool_name": "Edit", "tool_input": map[string]any{"file_path": repo + "/x.go"}})
	out := f.run("stop", map[string]any{})
	if !strings.Contains(out, `"decision":"block"`) || !strings.Contains(out, "## Description") || strings.Contains(out, "## Progress") {
		t.Fatalf("stop: %s", out)
	}
	if out := f.run("stop", map[string]any{}); out != "" {
		t.Fatalf("once: %s", out)
	}
	if !strings.Contains(f.tv.Read("sessions/2026-09/2026-09-27 1432 a1b2c3.md"), "status: idle") {
		t.Fatal("idle")
	}
}

func TestStopAsksNoRecordOfRepositoryWork(t *testing.T) {
	f := setup(t)
	repo := f.tv.Repo("p3-edge", nil)
	f.tv.Doc("repository", "p3-edge", map[string]any{"path": repo}, "")
	f.tv.Commit()
	f.run("session-start", map[string]any{})
	own := "sessions/2026-09/2026-09-27 1432 a1b2c3.md"
	f.tv.Write(own, strings.Replace(f.tv.Read(own), "## Description\n", "## Description\n\nWork on the edge.\n", 1))
	f.run("touched", map[string]any{"tool_name": "Edit", "tool_input": map[string]any{"file_path": repo + "/x.go"}})
	if !strings.Contains(f.tv.Read(own), "[[p3-edge]]") {
		t.Fatal("the session records the repository it changed")
	}
	if out := f.run("stop", map[string]any{}); out != "" {
		t.Fatalf("a session that changed a repository with no progress line owes nothing: %s", out)
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

func TestAHookTakesTheVaultFromAtlasVaultAndFallsBackToTheFolder(t *testing.T) {
	f := setup(t)
	sessionsIn := func() int {
		matches, _ := filepath.Glob(filepath.Join(f.tv.V.Root, vault.Sessions, "*", "*.md"))
		return len(matches)
	}
	runWith := func(envVault, cwd, id string) {
		data, _ := json.Marshal(map[string]any{"session_id": id, "cwd": cwd, "source": "startup"})
		env := f.env()
		home := env.Getenv
		env.Getenv = func(k string) string {
			if k == vault.EnvVault {
				return envVault
			}
			return home(k)
		}
		if err := hooks.Run("session-start", bytes.NewReader(data), &bytes.Buffer{}, env); err != nil {
			t.Fatal(err)
		}
	}
	before := sessionsIn()
	runWith(f.tv.V.Root, t.TempDir(), "b1b2c3d4-5e6f-7a8b-9c0d-000000000101")
	if sessionsIn() != before+1 {
		t.Fatal("ATLAS_VAULT did not choose the vault for a session outside it")
	}
	runWith("/no/such/vault", f.tv.V.Root, "c1b2c3d4-5e6f-7a8b-9c0d-000000000102")
	if sessionsIn() != before+2 {
		t.Fatal("a bad ATLAS_VAULT did not fall back to the folder's vault")
	}
	// From inside another vault, ATLAS_VAULT still wins over the folder's vault.
	other := testvault.New(t)
	otherSessions := func() int {
		matches, _ := filepath.Glob(filepath.Join(other.V.Root, vault.Sessions, "*", "*.md"))
		return len(matches)
	}
	otherBefore := otherSessions()
	runWith(f.tv.V.Root, other.V.Root, "d1b2c3d4-5e6f-7a8b-9c0d-000000000103")
	if sessionsIn() != before+3 || otherSessions() != otherBefore {
		t.Fatalf("ATLAS_VAULT did not win over the folder's vault: %d in ATLAS_VAULT's, %d in the folder's", sessionsIn()-before, otherSessions()-otherBefore)
	}
}

// The refusal of an edit under changes/ names what the agent and the user can do, and
// no edit the guard never allows.
func TestTheChangesRefusalNamesSupersedes(t *testing.T) {
	f := setup(t)
	f.run("session-start", map[string]any{})
	out := f.run("guard", edit(f.tv.V.Root+"/changes/2026-09/x.md", "x"))
	if !denied(out) || !strings.Contains(out, "supersedes") || !strings.Contains(out, "Obsidian") || strings.Contains(out, "only when the user asks") {
		t.Fatalf("refusal: %s", out)
	}
}
