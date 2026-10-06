package hooks_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	f.tv.Doc("topic", "Alpha", map[string]any{"kind": "concept"}, "## Definition\n\nx\n")
	f.tv.Commit()
	f.run("session-start", map[string]any{})
	outside := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "source-core", "documents"), filepath.Join(outside, "docs")); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		event map[string]any
	}{
		{"a topic through a link from outside the vault", map[string]any{"cwd": outside, "tool_name": "Edit", "tool_input": map[string]any{"file_path": filepath.Join(outside, "docs", "Alpha.md"), "old_string": "x", "new_string": "y"}}},
	}
	if foldsCase(t, root) {
		cases = append(cases, []struct {
			name  string
			event map[string]any
		}{
			{"a topic in another case", edit(root+"/Source-Core/documents/Alpha.md", "x")},
			{"Atlas.md in upper case", edit(root+"/ATLAS.md", "Work")},
			{"a view in upper case", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": root + "/WIKI-VIEW/x.md"}}},
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

// own starts the session, gives its document a worker's line under ## Subagents and
// summary under ## Summary, and returns the document's path in the vault and the line.
func (f *fixture) own(summary string) (string, string) {
	f.t.Helper()
	f.run("session-start", map[string]any{})
	f.run("subagent-start", map[string]any{"agent_id": "9f07d1aa", "agent_type": "atlas-obsidian:wiki-extract"})
	rel := "sessions/2026-09/2026-09-27 1432 a1b2c3.md"
	line := "- wiki-extract · `9f07d1` · started 14:32"
	content := f.tv.Read(rel)
	if !strings.Contains(content, "## Summary\n\n## Subagents\n\n"+line) {
		f.t.Fatalf("the session document is not as the test needs:\n%s", content)
	}
	f.tv.Write(rel, strings.Replace(content, "## Summary\n", "## Summary\n\n"+summary+"\n", 1))
	return rel, line
}

// A heading inside a code fence moves no protected span: the real ## Subagents of a
// session document stays the hooks', and the fenced lines stay the model's prose.
func TestAFencedHeadingMovesNoProtectedSpan(t *testing.T) {
	f := setup(t)
	rel, line := f.own("A summary with an example:\n\n```md\n## Subagents\n\nexample line\n```")
	own := filepath.Join(f.tv.V.Root, rel)
	if !denied(f.run("guard", edit(own, line))) {
		t.Error("an edit inside the real ## Subagents was allowed")
	}
	if denied(f.run("guard", edit(own, "example line"))) {
		t.Error("an edit inside the fence was refused")
	}
}

// A Codex hunk that only inserts lines is placed by its context lines.
func TestAnInsertOnlyPatchHunkIsPlacedByItsContext(t *testing.T) {
	f := setup(t)
	rel, line := f.own("The first summary line.")
	patch := func(context, added string) map[string]any {
		return map[string]any{"tool_name": "apply_patch", "tool_input": map[string]any{"command": "*** Begin Patch\n*** Update File: " + rel + "\n@@\n " + context + "\n+" + added + "\n*** End Patch"}}
	}
	if !denied(f.run("guard", patch(line, "- forged · `000000`"))) {
		t.Errorf("an insert into ## Subagents was allowed:\n%s", f.tv.Read(rel))
	}
	if denied(f.run("guard", patch("The first summary line.", "A second summary line."))) {
		t.Error("an insert into ## Summary was refused")
	}
	raw := func(body string) map[string]any {
		return map[string]any{"tool_name": "apply_patch", "tool_input": map[string]any{"command": "*** Begin Patch\n*** Update File: " + rel + "\n" + body + "\n*** End Patch"}}
	}
	// As Codex 0.155.1 places them: a hunk with no old line at the end of the file, a
	// blank context line at any blank line, and lines that differ by trailing spaces.
	for name, body := range map[string]string{
		"an insert with no context":            "@@\n+- forged · `000000`",
		"an insert after a header":             "@@ ## Subagents\n+- forged · `000000`",
		"an insert after a blank line":         "@@\n \n+status: forged",
		"a removed line with a trailing space": "@@\n-" + line + " \n+- forged · `000000`",
		"a context line with a trailing space": "@@\n " + line + "  \n+- forged · `000000`",
		"a hunk whose context matches nowhere": "@@\n no such line\n+- forged · `000000`",
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

// An edit adds no second heading of a section the hooks own, since a second heading
// would move the protected span.
func TestAnEditAddsNoSecondHeading(t *testing.T) {
	f := setup(t)
	rel, _ := f.own("A summary.")
	own := filepath.Join(f.tv.V.Root, rel)
	if !denied(f.run("guard", editNew(own, "## Summary\n\nA summary.", "## Summary\n\nA summary.\n\n## Subagents\n"))) {
		t.Error("a second ## Subagents was allowed")
	}
	if denied(f.run("guard", editNew(own, "## Summary\n\nA summary.", "## Summary\n\nA summary, said better."))) {
		t.Error("an edit that keeps its own heading was refused")
	}
	if denied(f.run("guard", editNew(own, "## Summary\n\nA summary.", "## Summary\n\nA summary, with an example:\n\n```md\n## Subagents\n```"))) {
		t.Error("a fenced example heading was refused")
	}
}

// A link to a file that does not exist yet is judged by where a write through it lands.
func TestADanglingLinkIsJudgedByItsTarget(t *testing.T) {
	f := setup(t)
	f.run("session-start", map[string]any{})
	link := filepath.Join(f.tv.V.Root, "ingest", "dang.md")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../wiki-view/dang.md", link); err != nil {
		t.Fatal(err)
	}
	if !denied(f.run("guard", map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": link}})) {
		t.Error("a Write through a dangling link into wiki-view/ was allowed")
	}
	loop := filepath.Join(f.tv.V.Root, "ingest", "loop.md")
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
	rel, line := f.own("A summary.")
	f.tv.Doc("topic", "Alpha", map[string]any{"kind": "concept"}, "## Definition\n\nx\n")
	f.tv.Commit()
	for name, body := range map[string]string{
		"an insert into ## Subagents": " *** Update File: " + rel + "\n@@\n " + line + "\n+- forged · `000000`",
		"the vault's config":          "\t*** Add File: .atlas/config.json\n+{}",
		"a document deleted":          " *** Delete File: source-core/documents/Alpha.md",
	} {
		patch := map[string]any{"tool_name": "apply_patch", "tool_input": map[string]any{"command": "*** Begin Patch\n" + body + "\n*** End Patch"}}
		if !denied(f.run("guard", patch)) {
			t.Errorf("%s: allowed", name)
		}
	}
}

// An edit is judged by the document it leaves, read by doc.Headings: no heading trick,
// fence, or tab moves or hides a section the hooks own.
func TestAnEditIsJudgedByTheDocumentItLeaves(t *testing.T) {
	f := setup(t)
	rel, _ := f.own("A summary.")
	own := filepath.Join(f.tv.V.Root, rel)
	for name, added := range map[string]string{
		"a heading after a tab":             "##\tSubagents\n\n- forged · `000000`",
		"a heading between indented fences": "    ```\n## Subagents\n    ```",
		"an unclosed fence":                 "```",
	} {
		if !denied(f.run("guard", editNew(own, "## Summary\n\nA summary.", "## Summary\n\nA summary.\n\n"+added))) {
			t.Errorf("%s: allowed", name)
		}
	}
	patch := "*** Begin Patch\n*** Update File: " + rel + "\n@@\n A summary.\n+```\n*** End Patch"
	if !denied(f.run("guard", map[string]any{"tool_name": "apply_patch", "tool_input": map[string]any{"command": patch}})) {
		t.Error("a patch that leaves an unclosed fence: allowed")
	}
	if denied(f.run("guard", editNew(own, "## Summary\n\nA summary.", "## Summary\n\nA summary, said better.\n\n```go\nx := 1\n```"))) {
		t.Error("an ordinary Edit of ## Summary with a closed fence was refused")
	}
}

// A hunk that fits nowhere is refused for that, not for a rule it may not break.
func TestAnUnplacedHunkSaysSo(t *testing.T) {
	f := setup(t)
	rel, _ := f.own("A summary.")
	patch := "*** Begin Patch\n*** Update File: " + rel + "\n@@\n no such line\n+more\n*** End Patch"
	out := f.run("guard", map[string]any{"tool_name": "apply_patch", "tool_input": map[string]any{"command": patch}})
	if !denied(out) || !strings.Contains(out, "cannot tell where a hunk") {
		t.Fatalf("an unplaced hunk: %s", out)
	}
}

// A Codex move takes a file from its place: the source is judged as a delete, the
// target as a new file.
func TestACodexMoveIsADeleteAndAnAdd(t *testing.T) {
	f := setup(t)
	own, line := f.own("A summary.")
	f.tv.Doc("topic", "Alpha", map[string]any{"kind": "concept"}, "## Definition\n\nx\n")
	f.tv.Commit()
	topic := "source-core/documents/Alpha.md"
	f.tv.Write("ingest/note.md", "A note.\n")
	move := func(from, to, hunk string) map[string]any {
		return map[string]any{"tool_name": "apply_patch", "tool_input": map[string]any{"command": "*** Begin Patch\n*** Update File: " + from + "\n*** Move to: " + to + "\n" + hunk + "*** End Patch"}}
	}
	for name, ev := range map[string]map[string]any{
		"a topic out with a line changed": move(topic, "ingest/x.md", "@@\n-x\n+y\n"),
		"a topic out":                     move(topic, "ingest/y.md", ""),
		"the session's own document out":  move(own, "ingest/z.md", ""),
		"its own Subagents line out":      move(own, "ingest/w.md", "@@\n-"+line+"\n"),
	} {
		if !denied(f.run("guard", ev)) {
			t.Errorf("%s: allowed", name)
		}
	}
	if denied(f.run("guard", move("ingest/note.md", "ingest/kept.md", ""))) {
		t.Error("a move of a note in ingest/ was refused")
	}
}

// An Edit's anchor may hold a code heading the edit leaves as it was.
func TestAnEditMayAnchorOnACodeHeading(t *testing.T) {
	f := setup(t)
	rel, line := f.own("A summary with quotes.")
	own := filepath.Join(f.tv.V.Root, rel)
	if out := f.run("guard", editNew(own, "A summary with quotes.\n\n## Subagents", "A summary with quotes, said better.\n\n## Subagents")); denied(out) {
		t.Fatalf("an Edit anchored on ## Subagents was refused: %s", out)
	}
	if !denied(f.run("guard", editNew(own, "## Subagents\n\n"+line, "## Subagents\n\n- forged · `000000`"))) {
		t.Error("an Edit of a ## Subagents line was allowed")
	}
}

// An Edit with straight quotes where the file has curly ones is judged as the host
// would apply it.
func TestAnEditIsMatchedWithCurlyAndStraightQuotesAlike(t *testing.T) {
	f := setup(t)
	rel, line := f.own("It said “go”.")
	own := filepath.Join(f.tv.V.Root, rel)
	if !strings.Contains(f.tv.Read(rel), "“go”") {
		t.Fatal("the session document lost its curly quotes, so the test proves nothing")
	}
	if !denied(f.run("guard", editNew(own, "It said \"go\".\n\n## Subagents\n\n"+line, "It said \"go\".\n\n## Subagents\n\n- forged · `000000`"))) {
		t.Error("an Edit spanning from straight-quoted text into ## Subagents was allowed")
	}
}

// A refusal of a command read out of quoted text says why the text counted as one.
func TestARefusalOfQuotedTextSaysWhy(t *testing.T) {
	f := setup(t)
	f.run("session-start", map[string]any{})
	bin := "atlas-" + "obsidian"
	out := f.run("guard", bash(`echo "`+bin+` hook prompt" | sh`))
	if !denied(out) || !strings.Contains(out, "read this from quoted text") {
		t.Fatalf("the refusal of quoted text: %s", out)
	}
	if out := f.run("guard", bash(bin+` hook prompt`)); strings.Contains(out, "quoted text") {
		t.Fatalf("a plain command's refusal speaks of quoted text: %s", out)
	}
}
