package hooks_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/thread"
)

// foldsCase reports whether the file system under dir ignores case, as APFS does.
func foldsCase(t *testing.T, dir string) bool {
	t.Helper()
	probe := filepath.Join(dir, "probe-case")
	if err := os.WriteFile(probe, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(probe)
	_, err := os.Lstat(filepath.Join(dir, "PROBE-CASE"))
	return err == nil
}

func TestTheGuardJudgesThePathTheDiskNames(t *testing.T) {
	f := setup(t)
	root := f.tv.V.Root
	repo := f.tv.Repo("repo1", nil)
	f.tv.Doc("repository", "repo1", map[string]any{"path": repo}, "")
	f.tv.Doc("topic", "Alpha", map[string]any{"kind": "concept"}, "## Definition\n\nx\n")
	f.tv.Commit()
	f.run("session-start", map[string]any{})
	outside := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "wiki", "documents"), filepath.Join(outside, "docs")); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		event map[string]any
	}{
		{"a topic through a link from outside the vault", map[string]any{"cwd": outside, "tool_name": "Edit", "tool_input": map[string]any{"file_path": filepath.Join(outside, "docs", "Alpha.md"), "old_string": "x", "new_string": "y"}}},
	}
	if foldsCase(t, root) {
		upper := filepath.Join(filepath.Dir(repo), strings.ToUpper(filepath.Base(repo)))
		cases = append(cases, []struct {
			name  string
			event map[string]any
		}{
			{"a topic in another case", edit(root+"/Wiki/documents/Alpha.md", "x")},
			{"Atlas.md in upper case", edit(root+"/ATLAS.md", "Work")},
			{"a view in upper case", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": root + "/VIEWS/x.md"}}},
			{"a repository's file in another case", map[string]any{"tool_name": "Edit", "tool_input": map[string]any{"file_path": upper + "/README.md", "old_string": "a"}}},
		}...)
	} else {
		t.Log("this file system keeps case; only the link case runs")
	}
	for _, c := range cases {
		if !denied(f.run("guard", c.event)) {
			t.Errorf("%s: allowed", c.name)
		}
	}
}

// A heading inside a code fence moves no protected span: the real ## Thread of a stub
// stays code's, and the fenced lines stay the model's prose.
func TestAFencedHeadingMovesNoProtectedSpan(t *testing.T) {
	f := setup(t)
	f.run("session-start", map[string]any{})
	r := f.ok(thread.Stub(f.tv.V, thread.StubIn{Title: "Fenced", Text: "An idea with an example:\n\n```md\n## Thread\n\nexample line\n```"}, thread.Opts{Now: f.tv.Clock}))
	stub := filepath.Join(f.tv.V.Root, r.State.Thread.Path)
	if got := f.tv.Read(r.State.Thread.Path); !strings.Contains(got, "example line") || !strings.Contains(got, "- Spec: none") {
		t.Fatalf("the stub is not as the test needs:\n%s", got)
	}
	if !denied(f.run("guard", edit(stub, "- Spec: none"))) {
		t.Error("an edit inside the real ## Thread was allowed")
	}
	if denied(f.run("guard", edit(stub, "example line"))) {
		t.Error("an edit inside the fence was refused")
	}
}

// A Codex hunk that only inserts lines is placed by its context lines.
func TestAnInsertOnlyPatchHunkIsPlacedByItsContext(t *testing.T) {
	f := setup(t)
	f.run("session-start", map[string]any{})
	r := f.ok(thread.Stub(f.tv.V, thread.StubIn{Title: "Patched", Text: "The first idea line."}, thread.Opts{Now: f.tv.Clock}))
	patch := func(context, added string) map[string]any {
		return map[string]any{"tool_name": "apply_patch", "tool_input": map[string]any{"command": "*** Begin Patch\n*** Update File: " + r.State.Thread.Path + "\n@@\n " + context + "\n+" + added + "\n*** End Patch"}}
	}
	if !denied(f.run("guard", patch("- Spec: none", "- Spec: [[Forged]]"))) {
		t.Errorf("an insert into ## Thread was allowed:\n%s", f.tv.Read(r.State.Thread.Path))
	}
	if denied(f.run("guard", patch("The first idea line.", "A second idea line."))) {
		t.Error("an insert into ## Idea was refused")
	}
	raw := func(body string) map[string]any {
		return map[string]any{"tool_name": "apply_patch", "tool_input": map[string]any{"command": "*** Begin Patch\n*** Update File: " + r.State.Thread.Path + "\n" + body + "\n*** End Patch"}}
	}
	// As Codex 0.155.1 places them: a hunk with no old line at the end of the file, a
	// blank context line at any blank line, and lines that differ by trailing spaces.
	for name, body := range map[string]string{
		"an insert with no context":            "@@\n+- Spec: [[Forged]]",
		"an insert after a header":             "@@ ## Thread\n+- Spec: [[Forged]]",
		"an insert after a blank line":         "@@\n \n+status: forged",
		"a removed line with a trailing space": "@@\n-- Spec: none \n+- Spec: [[Forged]]",
		"a context line with a trailing space": "@@\n - Spec: none  \n+- Tasks: [[Forged]]",
		"a hunk whose context matches nowhere": "@@\n no such line\n+- Spec: [[Forged]]",
	} {
		if !denied(f.run("guard", raw(body))) {
			t.Errorf("%s: allowed", name)
		}
	}
}

// The files whose values decide what Atlas runs are the user's.
func TestTheGuardRefusesTheFilesThatDecideWhatRuns(t *testing.T) {
	f := setup(t)
	f.run("session-start", map[string]any{})
	for name, file := range map[string]string{
		"the machine config":    filepath.Join(f.tv.Home.Root, "config.json"),
		"the vault's config":    filepath.Join(f.tv.V.Root, ".atlas", "config.json"),
		"the plugin's settings": filepath.Join(f.tv.V.Root, ".obsidian", "plugins", "atlas", "data.json"),
		"the plugin's code":     filepath.Join(f.tv.V.Root, ".obsidian", "plugins", "atlas", "main.js"),
	} {
		if !denied(f.run("guard", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": file}})) {
			t.Errorf("%s: allowed", name)
		}
	}
	if denied(f.run("guard", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": filepath.Join(f.tv.V.Root, ".obsidian", "plugins", "other", "data.json")}})) {
		t.Error("another plugin's settings were refused")
	}
}

// The guard finds a thread's task lists by their thread field, so a list renamed by hand
// still lets the thread's session edit its repository.
func TestARenamedTaskListStillCoversItsRepository(t *testing.T) {
	f := setup(t)
	edge := f.tv.Repo("p3-edge", nil)
	f.tv.Doc("repository", "p3-edge", map[string]any{"path": edge}, "")
	f.tv.Commit()
	f.run("session-start", map[string]any{})
	f.thread("Edge work", "p3-edge")
	f.bind("start", f.ok(thread.Start(f.tv.V, "Edge work", false, thread.Opts{Now: f.tv.Clock})), map[string]any{"thread": "Edge work"})
	lists, _ := filepath.Glob(filepath.Join(f.tv.V.Root, "wiki", "documents", "Edge work · Tasks*.md"))
	if len(lists) != 1 {
		t.Fatalf("task lists: %v", lists)
	}
	if err := os.Rename(lists[0], filepath.Join(f.tv.V.Root, "wiki", "documents", "Edge checklist.md")); err != nil {
		t.Fatal(err)
	}
	write := map[string]any{"tool_name": "Edit", "tool_input": map[string]any{"file_path": edge + "/main.go", "old_string": "a"}}
	if out := f.run("guard", write); denied(out) {
		t.Fatalf("a renamed task list with an open task does not cover its repository: %s", out)
	}
}

// An edit adds no second heading of a section the document holds, or of a section code
// owns, since a second heading would move the protected span.
func TestAnEditAddsNoSecondHeading(t *testing.T) {
	f := setup(t)
	f.run("session-start", map[string]any{})
	r := f.ok(thread.Stub(f.tv.V, thread.StubIn{Title: "Twice", Text: "An idea."}, thread.Opts{Now: f.tv.Clock}))
	stub := filepath.Join(f.tv.V.Root, r.State.Thread.Path)
	if !denied(f.run("guard", editNew(stub, "## Idea\n\nAn idea.", "## Idea\n\nAn idea.\n\n## Thread\n\n- Spec: none"))) {
		t.Error("a second ## Thread was allowed")
	}
	if !denied(f.run("guard", editNew(stub, "## Idea\n\nAn idea.", "## Idea\n\nAn idea.\n\n## Idea\n\nMore."))) {
		t.Error("a second ## Idea was allowed")
	}
	if denied(f.run("guard", editNew(stub, "## Idea\n\nAn idea.", "## Idea\n\nAn idea, said better."))) {
		t.Error("an edit that keeps its own heading was refused")
	}
	if denied(f.run("guard", editNew(stub, "## Idea\n\nAn idea.", "## Idea\n\nAn idea, with an example:\n\n```md\n## Thread\n```"))) {
		t.Error("a fenced example heading was refused")
	}
	own := filepath.Join(f.tv.V.Root, "sessions/2026-09/2026-09-27 1432 a1b2c3.md")
	if !denied(f.run("guard", editNew(own, "## Description\n", "## Description\n\n## Subagents\n"))) {
		t.Error("a second ## Subagents was allowed")
	}
}

// A session outside every vault meets the edit rule in a linked repository too.
func TestTheEditRuleHoldsForASessionOutsideEveryVault(t *testing.T) {
	f := setup(t)
	repo := f.tv.Repo("repo1", nil)
	f.tv.Doc("repository", "repo1", map[string]any{"path": repo}, "")
	f.tv.Commit()
	outside := t.TempDir()
	write := map[string]any{"cwd": outside, "tool_name": "Edit", "tool_input": map[string]any{"file_path": filepath.Join(repo, "README.md"), "old_string": "a"}}
	if out := f.run("guard", write); !denied(out) || !strings.Contains(out, "needs a thread this session started") {
		t.Fatalf("an edit in a linked repository from outside every vault: %s", out)
	}
}

// A link to a file that does not exist yet is judged by where a write through it lands.
func TestADanglingLinkIsJudgedByItsTarget(t *testing.T) {
	f := setup(t)
	f.run("session-start", map[string]any{})
	link := filepath.Join(f.tv.V.Root, "inbox", "dang.md")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../views/dang.md", link); err != nil {
		t.Fatal(err)
	}
	if !denied(f.run("guard", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": link}})) {
		t.Error("a Write through a dangling link into views/ was allowed")
	}
	loop := filepath.Join(f.tv.V.Root, "inbox", "loop.md")
	if err := os.Symlink("loop.md", loop); err != nil {
		t.Fatal(err)
	}
	f.run("guard", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": loop}})
}

// A folder that does not exist yet keeps the typed case, so fixed names compare without
// case.
func TestFixedNamesCompareWithoutCase(t *testing.T) {
	f := setup(t)
	f.run("session-start", map[string]any{})
	if err := os.RemoveAll(filepath.Join(f.tv.V.Root, ".obsidian", "plugins", "atlas")); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".obsidian/plugins/ATLAS/data.json", "Other.BASE", ".ATLAS/config.json"} {
		if !denied(f.run("guard", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": filepath.Join(f.tv.V.Root, rel)}})) {
			t.Errorf("%s: allowed", rel)
		}
	}
}

// Codex takes a file marker with whitespace before it, and so does the guard.
func TestAPatchMarkerWithWhitespaceIsJudged(t *testing.T) {
	f := setup(t)
	f.run("session-start", map[string]any{})
	r := f.ok(thread.Stub(f.tv.V, thread.StubIn{Title: "Marked", Text: "An idea."}, thread.Opts{Now: f.tv.Clock}))
	f.tv.Doc("topic", "Alpha", map[string]any{"kind": "concept"}, "## Definition\n\nx\n")
	f.tv.Commit()
	for name, body := range map[string]string{
		"an insert into ## Thread": " *** Update File: " + r.State.Thread.Path + "\n@@\n - Verification: none\n+- Forged: yes",
		"the vault's config":       "\t*** Add File: .atlas/config.json\n+{}",
		"a document deleted":       " *** Delete File: wiki/documents/Alpha.md",
	} {
		patch := map[string]any{"tool_name": "apply_patch", "tool_input": map[string]any{"command": "*** Begin Patch\n" + body + "\n*** End Patch"}}
		if !denied(f.run("guard", patch)) {
			t.Errorf("%s: allowed", name)
		}
	}
}

// An edit is judged by the document it leaves, read by doc.Headings: no heading trick,
// fence, or tab moves or hides a section code owns.
func TestAnEditIsJudgedByTheDocumentItLeaves(t *testing.T) {
	f := setup(t)
	f.run("session-start", map[string]any{})
	r := f.ok(thread.Stub(f.tv.V, thread.StubIn{Title: "Tricks", Text: "An idea."}, thread.Opts{Now: f.tv.Clock}))
	stub := filepath.Join(f.tv.V.Root, r.State.Thread.Path)
	for name, added := range map[string]string{
		"a heading after a tab":                "##\tThread\n\n- Spec: none",
		"a heading between indented fences":    "    ```\n## Thread\n    ```",
		"an unclosed fence":                    "```",
		"a level-two heading of a new section": "##\tProgress\n\n- did a thing",
	} {
		if !denied(f.run("guard", editNew(stub, "## Idea\n\nAn idea.", "## Idea\n\nAn idea.\n\n"+added))) {
			t.Errorf("%s: allowed", name)
		}
	}
	patch := "*** Begin Patch\n*** Update File: " + r.State.Thread.Path + "\n@@\n An idea.\n+```\n*** End Patch"
	if !denied(f.run("guard", map[string]any{"tool_name": "apply_patch", "tool_input": map[string]any{"command": patch}})) {
		t.Error("a patch that leaves an unclosed fence: allowed")
	}
	if denied(f.run("guard", editNew(stub, "## Idea\n\nAn idea.", "## Idea\n\nAn idea, said better.\n\n```go\nx := 1\n```"))) {
		t.Error("an ordinary Edit of ## Idea with a closed fence was refused")
	}
}
