package vault_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

func TestInitWritesTheLayoutAndOneCommit(t *testing.T) {
	tv := testvault.New(t)
	v := tv.V
	if v.Name() != "Work" || v.Areas() != "few" || v.StaleHours() != 12 {
		t.Fatalf("settings: %s %s %d", v.Name(), v.Areas(), v.StaleHours())
	}
	if got := strings.Join(v.Wikify(), ","); got != "source,spec,receipt" {
		t.Fatalf("wikify %s", got)
	}
	for _, rel := range []string{"threads/Threads.base", "sessions/Sessions.base", "changes/Changes.base", "wiki/Wiki.base", ".obsidian/plugins/atlas/manifest.json", "wiki/sources/files", "inbox", "scratchpad"} {
		if !v.Exists(rel) {
			t.Errorf("missing %s", rel)
		}
	}
	if log := tv.Log(); len(log) != 1 || log[0] != "setup: Work" {
		t.Fatalf("log %v", log)
	}
	tv.Clean()
	cfg, err := tv.Home.Load()
	if err != nil || !cfg.Lists(v.Root) {
		t.Fatalf("config %v %v", cfg, err)
	}
	if _, err := vault.Init(vault.InitOptions{Path: v.Root, Name: "Again"}, tv.Home, testvault.Now); err == nil {
		t.Fatal("init refuses a folder that is a vault")
	}
	inner := filepath.Join(v.Root, "notes")
	if _, err := vault.Init(vault.InitOptions{Path: inner, Name: "Inner"}, tv.Home, testvault.Now); err == nil || !strings.Contains(err.Error(), "inside the repository") {
		t.Fatalf("init refuses a folder inside another repository: %v", err)
	}
}

func TestInitAdoptsAFolderOfNotes(t *testing.T) {
	tv := testvault.New(t)
	dir := filepath.Join(tv.Dir, "notes")
	tv.WriteFile(filepath.Join(dir, "Ideas.md"), "my ideas\n")
	tv.WriteFile(filepath.Join(dir, ".gitignore"), "secret\n")
	v, err := vault.Init(vault.InitOptions{Path: dir, Name: "Notes"}, tv.Home, testvault.Now)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, ".gitignore")); string(got) != "secret\n" {
		t.Fatalf("init edited a file of the user's: %q", got)
	}
	if v.Areas() != "manual" {
		t.Fatalf("the default area setting is manual: %s", v.Areas())
	}
	if !v.Git().Tracked("Ideas.md") {
		t.Fatal("the user's notes are in the first commit")
	}
}

func TestFindFromAVaultAndFromALinkedRepository(t *testing.T) {
	tv := testvault.New(t)
	sub := filepath.Join(tv.V.Root, "threads")
	v, err := vault.Find(sub, tv.Home)
	if err != nil || v.Root != tv.V.Root {
		t.Fatalf("find above: %v %v", v, err)
	}
	repo := tv.Repo("p3-cloud", nil)
	if _, err := vault.Find(repo, tv.Home); err == nil {
		t.Fatal("an unlinked repository has no vault")
	}
	tv.Page("repository", "p3-cloud", map[string]any{"path": repo}, "")
	v, err = vault.Find(filepath.Join(repo, "src"), tv.Home)
	if err != nil || v.Root != tv.V.Root {
		t.Fatalf("find through a repository page: %v %v", v, err)
	}
	if v, err := vault.Resolve("work", "/", tv.Home); err != nil || v.Root != tv.V.Root {
		t.Fatalf("resolve by name: %v", err)
	}
}

func TestIndexResolvesIdsTitlesAndAliases(t *testing.T) {
	tv := testvault.New(t)
	area := tv.Page("area", "p3", nil, "")
	id := tv.Page("concept", "Self-supervised learning", map[string]any{"aliases": []string{"SSL"}, "scope": "[[p3]]"}, "## Definition\n\nText with a [[p3]] link.\n")
	tv.Write("scratchpad/Draft.md", "a draft\n")
	tv.Write("Ideas.md", "---\ntags: [x]\n---\nmine\n")
	idx := tv.Index()
	for _, key := range []string{id, "Self-supervised learning", "[[self-supervised learning]]", "SSL"} {
		d, err := idx.Resolve(key)
		if err != nil || d.ID() != id {
			t.Fatalf("resolve %q: %v", key, err)
		}
	}
	if _, err := idx.Resolve("Nothing"); err == nil {
		t.Fatal("an unknown title is refused")
	}
	if typ, err := idx.TypeOfLink("[[p3]]"); err != nil || typ != "area" {
		t.Fatalf("type of link: %s %v", typ, err)
	}
	if typ, _ := idx.TypeOfLink("Draft"); typ != "file" {
		t.Fatalf("a scratchpad note is a link target: %q", typ)
	}
	if len(idx.Notes) != 1 || idx.Notes[0].Path != "Ideas.md" {
		t.Fatalf("notes: %v", idx.Notes)
	}
	d := idx.ByID(id)
	if got := idx.ScopeIDs(d); len(got) != 1 || got[0] != area {
		t.Fatalf("scope ids %v", got)
	}
	if !idx.InScope(d, area) || idx.InScope(d, "are-zzzzzz") {
		t.Fatal("in scope")
	}
	if ref := idx.Ref(d); ref.Title != "Self-supervised learning" || ref.Path != "wiki/p3/concepts/Self-supervised learning.md" {
		t.Fatalf("ref %+v", ref)
	}
}

func TestPendingFollowsAbsorbedHashes(t *testing.T) {
	tv := testvault.New(t)
	src := tv.Page("source", "DINOv2", map[string]any{"sha256": "3f9c1e2a7b8d44", "file": "[[x.pdf]]"}, "")
	idx := tv.Index()
	if !idx.Pending(idx.ByID(src)) {
		t.Fatal("a captured source is pending")
	}
	tv.Write("changes/2026-09/2026-09-27 Ingest.md", "---\nid: chg-aaaaaa\ntype: change\nstatus: applied\n---\n\n## Absorbed\n\n| Document | Id | Hash |\n|---|---|---|\n| [[DINOv2]] | "+src+" | 3f9c1e2a7b8d |\n")
	idx = tv.Index()
	if idx.Pending(idx.ByID(src)) {
		t.Fatal("an applied change absorbed it")
	}
}

func TestLockIsExclusive(t *testing.T) {
	tv := testvault.New(t)
	unlock, err := tv.V.Lock()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan bool)
	go func() {
		u, err := tv.V.Lock()
		if err == nil {
			u()
		}
		done <- true
	}()
	select {
	case <-done:
		t.Fatal("a second lock waited for nothing")
	default:
	}
	unlock()
	<-done
}

func TestAWriteUntracksTheGraphSettings(t *testing.T) {
	tv := testvault.New(t)
	tv.Write(".obsidian/graph.json", `{"colorGroups":[]}`)
	// A vault made before the graph settings were excluded tracks them.
	if out, err := exec.Command("git", "-C", tv.V.Root, "add", "-f", ".obsidian/graph.json").CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	tv.Commit()
	if !tv.V.Git().Tracked(".obsidian/graph.json") {
		t.Fatal("the setup did not track the graph settings")
	}
	tx, err := vault.Begin(tv.V, nil)
	if err != nil {
		t.Fatal(err)
	}
	tx.Close()
	if tv.V.Git().Tracked(".obsidian/graph.json") {
		t.Fatal("the graph settings are still tracked")
	}
	if tv.Read(".obsidian/graph.json") != `{"colorGroups":[]}` {
		t.Fatal("untracking removed the file")
	}
	if log := tv.Log(); log[0] != "untrack machine files: .obsidian/graph.json" {
		t.Fatalf("log %v", log)
	}
	tv.Write(".obsidian/graph.json", `{"colorGroups":[{"query":"path:/^(?:a\\.md)$/"}]}`)
	tv.Clean()
}

func TestSyncSettingsKeepsOtherKeys(t *testing.T) {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", nil)
	tv.Write(".claude/settings.local.json", `{"model": "x", "permissions": {"allow": ["Bash(ls)"], "additionalDirectories": ["/mine", "/old"]}}`)
	tv.Page("repository", "p3-edge", map[string]any{"path": repo}, "")
	wrote, err := tv.V.SyncSettings([]string{"/old"})
	if err != nil || !wrote {
		t.Fatalf("sync %v %v", wrote, err)
	}
	var got struct {
		Model       string
		Permissions struct {
			Allow                 []string
			AdditionalDirectories []string
		}
	}
	if err := json.Unmarshal([]byte(tv.Read(".claude/settings.local.json")), &got); err != nil {
		t.Fatal(err)
	}
	if got.Model != "x" || len(got.Permissions.Allow) != 1 || strings.Join(got.Permissions.AdditionalDirectories, ",") != "/mine,"+repo {
		t.Fatalf("settings %+v", got)
	}
	if wrote, _ := tv.V.SyncSettings(nil); wrote {
		t.Fatal("a second sync writes nothing")
	}
	entries, _ := tv.V.Git().Status()
	for _, e := range entries {
		if strings.HasPrefix(e.Path, ".claude") {
			t.Fatalf("the harness settings are kept out of git: %v", e)
		}
	}
}

func TestOpenNote(t *testing.T) {
	tv := testvault.New(t)
	tv.Write(".obsidian/workspace.json", `{"main":{"id":"a","type":"split","children":[{"id":"b","type":"tabs","children":[{"id":"leaf1","type":"leaf","state":{"type":"markdown","state":{"file":"threads/X/X — Spec.md"}}}]}]},"active":"leaf1","lastOpenFiles":["Other.md"]}`)
	if got := tv.V.OpenNote(); got != "threads/X/X — Spec.md" {
		t.Fatalf("open note %q", got)
	}
}
