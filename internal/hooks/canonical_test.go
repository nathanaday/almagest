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
