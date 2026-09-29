package vault_test

import (
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/core"
	"github.com/nathanaday/atlas-obsidian/internal/lint"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
)

func page(id, typ, fields string) string {
	return "---\nid: " + id + "\ntype: " + typ + "\ncreated: 2026-09-27\nupdated: 2026-09-27\ndescription: x\n" + fields + "---\n"
}

func TestSyncMovesAnOldVaultIntoScopeFolders(t *testing.T) {
	tv := testvault.New(t)
	repo := tv.Repo("cs566-course", nil)
	tv.Write("wiki/areas/Machine Learning.md", page("are-aaaaa1", "area", "parent: \"\"\n"))
	tv.Write("wiki/areas/CS566.md", page("are-aaaaa2", "area", "parent: \"[[Machine Learning]]\"\n"))
	tv.Write("wiki/repositories/cs566-course.md", page("rep-aaaaa3", "repository", "parent: \"[[CS566]]\"\npath: "+repo+"\n"))
	tv.Write("wiki/concepts/Backprop.md", page("con-aaaaa4", "concept", "scope: \"[[CS566]]\"\n"))
	tv.Write("wiki/concepts/Everywhere.md", page("con-aaaaa5", "concept", "scope: \"\"\n"))
	tv.Write("wiki/sources/Lecture 1.md", page("src-aaaaa6", "source", "scope: \"[[cs566-course]]\"\nfile: \"[[src-aaaaa6.pdf]]\"\nsha256: abcdef\n"))
	tv.Write("wiki/sources/files/src-aaaaa6.pdf", "%PDF")
	tv.Commit()
	if got := tv.V.AreaParents(); got["cs566"] != "Machine Learning" {
		t.Fatalf("the fast reader reads an old vault: %v", got)
	}
	tv.Write("wiki/concepts/Backprop.md", tv.Read("wiki/concepts/Backprop.md")+"A hand edit.\n")

	if _, err := core.Sync(tv.V, testvault.Now); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		"wiki/Machine Learning/Machine Learning.md",
		"wiki/Machine Learning/CS566/CS566.md",
		"wiki/Machine Learning/CS566/cs566-course/cs566-course.md",
		"wiki/Machine Learning/CS566/concepts/Backprop.md",
		"wiki/Machine Learning/CS566/cs566-course/sources/Lecture 1.md",
		"wiki/concepts/Everywhere.md",
		"wiki/sources/files/src-aaaaa6.pdf",
	} {
		if !tv.V.Exists(p) {
			t.Errorf("%s is missing", p)
		}
	}
	if tv.V.Exists("wiki/areas") || tv.V.Exists("wiki/repositories") {
		t.Fatal("the old folders go")
	}
	log := tv.Log()
	if log[0] != "layout: move 5 pages into the folders of their scopes" || !strings.HasPrefix(log[1], "snapshot: ") {
		t.Fatalf("log %v", log)
	}
	if got := tv.Read("wiki/Machine Learning/CS566/concepts/Backprop.md"); !strings.Contains(got, "A hand edit.") || !strings.Contains(got, `chain: ["[[Machine Learning]]", "[[CS566]]"]`) {
		t.Fatalf("backprop:\n%s", got)
	}
	repos := tv.V.Repositories()
	if len(repos) != 1 || repos[0].Parent != "CS566" {
		t.Fatalf("the fast reader reads the folders: %+v", repos)
	}
	if f, _ := lint.Run(tv.Index(), lint.Options{Now: testvault.Now, Quick: true}); f.Counts[lint.Error] != 0 {
		t.Fatalf("lint: %+v", f.Findings)
	}
	if _, err := core.Sync(tv.V, testvault.Now); err != nil || tv.Log()[0] != log[0] {
		t.Fatal("a second sync moves nothing")
	}
}
