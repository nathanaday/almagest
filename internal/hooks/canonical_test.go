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
}
