package vault_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/almagest/internal/testvault"
	"github.com/nathanaday/almagest/internal/vault"
)

func TestInitWritesTheLayoutAndOneCommit(t *testing.T) {
	tv := testvault.New(t)
	v := tv.V
	if v.Name() != "Work" || v.Tagging() != "open" || v.StaleHours() != 12 || v.LayoutVersion() != vault.Layout || v.CheckLayout() != nil {
		t.Fatalf("settings: %s %s %d %d", v.Name(), v.Tagging(), v.StaleHours(), v.LayoutVersion())
	}
	if strings.Contains(tv.Read(vault.Marker), "wikify") {
		t.Fatalf("Almagest.md names wikify:\n%s", tv.Read(vault.Marker))
	}
	for _, rel := range []string{"tool/sessions/Sessions.base", "changes/Changes.base", ".obsidian/app.json", "tool/source-core/documents", "tool/source-core/originals", "wiki-view", "ingest", "scratchpad", "journals"} {
		if !v.Exists(rel) {
			t.Errorf("missing %s", rel)
		}
	}
	var app map[string]any
	if err := json.Unmarshal([]byte(tv.Read(".obsidian/app.json")), &app); err != nil || app["attachmentFolderPath"] != "tool/source-core/originals" || !strings.Contains(fmt.Sprint(app["userIgnoreFilters"]), "wiki-view/") || !strings.Contains(fmt.Sprint(app["userIgnoreFilters"]), "tool/trash/") {
		t.Fatalf("app settings %v %v", app, err)
	}
	if log := tv.Log(); len(log) != 1 || log[0] != "setup: Work" {
		t.Fatalf("log %v", log)
	}
	tv.Write("wiki-view/View · Home.md", "derived\n")
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
	tv.WriteFile(filepath.Join(dir, ".obsidian/app.json"), `{"attachmentFolderPath": "files", "vimMode": true}`)
	v, err := vault.Init(vault.InitOptions{Path: dir, Name: "Notes", Tagging: "known"}, tv.Home, testvault.Now)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, ".gitignore")); string(got) != "secret\n" {
		t.Fatalf("init edited a file of the user's: %q", got)
	}
	if v.Tagging() != "known" {
		t.Fatalf("tagging %s", v.Tagging())
	}
	if !v.Git().Tracked("Ideas.md") {
		t.Fatal("the user's notes are in the first commit")
	}
	app, _ := os.ReadFile(filepath.Join(dir, ".obsidian/app.json"))
	if !strings.Contains(string(app), `"attachmentFolderPath": "files"`) || !strings.Contains(string(app), "vimMode") || !strings.Contains(string(app), "wiki-view/") {
		t.Fatalf("init keeps the user's app settings: %s", app)
	}
	if _, err := vault.Init(vault.InitOptions{Path: filepath.Join(tv.Dir, "x"), Name: "X", Tagging: "many"}, tv.Home, testvault.Now); err == nil {
		t.Fatal("an unknown tagging mode is refused")
	}
}

func TestFindFromAVaultAndFromALinkedRepository(t *testing.T) {
	tv := testvault.New(t)
	sub := filepath.Join(tv.V.Root, "wiki", "documents")
	v, err := vault.Find(sub, tv.Home)
	if err != nil || v.Root != tv.V.Root {
		t.Fatalf("find above: %v %v", v, err)
	}
	repo := tv.Repo("p3-cloud", nil)
	if _, err := vault.Find(repo, tv.Home); err == nil {
		t.Fatal("an unlinked repository has no vault")
	}
	tv.Doc("repository", "p3-cloud", map[string]any{"path": repo, "defines": "work/p3/p3-cloud", "tags": []string{"work/p3"}}, "")
	v, err = vault.Find(filepath.Join(repo, "src"), tv.Home)
	if err != nil || v.Root != tv.V.Root {
		t.Fatalf("find through a repository document: %v %v", v, err)
	}
	if r := v.Repositories(); len(r) != 1 || r[0].Defines != "work/p3/p3-cloud" || r[0].Tags[0] != "work/p3" {
		t.Fatalf("repositories %+v", r)
	}
	if v, err := vault.Resolve("work", "/", tv.Home); err != nil || v.Root != tv.V.Root {
		t.Fatalf("resolve by name: %v", err)
	}
	tv.Doc("repository", "gone", map[string]any{"path": repo, "unlinked": true}, "")
	for _, r := range tv.V.Repositories() {
		if r.Title == "gone" && r.Path != "" {
			t.Fatal("an unlinked repository names no path")
		}
	}
}

func TestIndexResolvesIdsTitlesAliasesAndTags(t *testing.T) {
	tv := testvault.New(t)
	tv.Doc("topic", "CS513", map[string]any{"kind": "overview", "defines": "school/cs513", "tags": []string{"school"}}, "")
	id := tv.Doc("topic", "Self-supervised learning", map[string]any{"kind": "concept", "aliases": []string{"SSL"}, "tags": []string{"school/cs513/hw1", "ml"}}, "## Definition\n\nText with a [[CS513]] link.\n")
	tv.Write("scratchpad/Draft.md", "a draft\n")
	tv.Write("Ideas.md", "---\ntags: [x]\n---\nmine\n")
	tv.Write("notes/Stray.md", "---\nid: doc-zzzzzz\ntype: topic\nkind: concept\n---\nmoved by hand\n")
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
	if typ, err := idx.TypeOfLink("[[CS513]]"); err != nil || typ != "topic" {
		t.Fatalf("type of link: %s %v", typ, err)
	}
	if typ, _ := idx.TypeOfLink("Draft"); typ != "file" {
		t.Fatalf("a scratchpad note is a link target: %q", typ)
	}
	if len(idx.Notes) != 1 || idx.Notes[0].Path != "Ideas.md" || len(idx.Misplaced) != 1 || idx.ByID("doc-zzzzzz") != nil {
		t.Fatalf("notes %v misplaced %v", idx.Notes, idx.Misplaced)
	}
	d := idx.ByID(id)
	if !vault.Holds(d, "school") || !vault.Holds(d, "school/cs513", "ml") || vault.Holds(d, "school/cs51") {
		t.Fatal("holds")
	}
	c := idx.TagCounts()
	if c["school"] != 2 || c["school/cs513"] != 2 || c["school/cs513/hw1"] != 1 || c["ml"] != 1 {
		t.Fatalf("counts %v", c)
	}
	if p := idx.TagPage("school/cs513"); p == nil || p.Title() != "CS513" {
		t.Fatal("tag page")
	}
	if got := idx.TagChildren("school"); len(got) != 1 || got[0] != "school/cs513" {
		t.Fatalf("children %v", got)
	}
	if ref := idx.Ref(d); ref.Title != "Self-supervised learning" || ref.Path != "tool/source-core/documents/Self-supervised learning.md" || ref.Kind != "concept" || len(ref.Tags) != 2 {
		t.Fatalf("ref %+v", ref)
	}
}

func TestPendingFollowsAbsorbedHashes(t *testing.T) {
	tv := testvault.New(t)
	src := tv.Doc("source", "DINOv2", map[string]any{"sha256": "3f9c1e2a7b8d44", "file": "[[x.pdf]]"}, "")
	other := tv.Doc("source", "Notes", map[string]any{"sha256": "aaaaaaaaaaaa11", "file": "[[y.md]]"}, "")
	topic := tv.Doc("topic", "Plan", map[string]any{"kind": "concept"}, "## Definition\n\nDo it.\n")
	repo := tv.Doc("repository", "p3-edge", map[string]any{"path": "/y"}, "## What it is\n\nx\n")
	idx := tv.Index()
	for _, id := range []string{src, other} {
		if !idx.Pending(idx.ByID(id)) {
			t.Fatalf("%s is pending: a source no change absorbed", idx.ByID(id).Title())
		}
	}
	for _, id := range []string{topic, repo} {
		if idx.Pending(idx.ByID(id)) {
			t.Fatalf("%s is not pending: only a source is", idx.ByID(id).Title())
		}
	}
	if vault.Hash(idx.ByID(src)) != "3f9c1e2a7b8d44" {
		t.Fatalf("a source's hash is its sha256: %s", vault.Hash(idx.ByID(src)))
	}
	tv.Write("changes/2026-09/2026-09-27 Ingest.md", "---\nid: chg-aaaaaa\ntype: change\nstatus: applied\n---\n\n## Absorbed\n\n| Document | Id | Hash |\n|---|---|---|\n| [[DINOv2]] | "+src+" | 3f9c1e2a7b8d |\n| [[Notes]] | "+other+" | 0000000000aa |\n")
	idx = tv.Index()
	if idx.Pending(idx.ByID(src)) {
		t.Fatal("an applied change absorbed it")
	}
	if !idx.Pending(idx.ByID(other)) {
		t.Fatal("a change that absorbed another hash leaves a source pending")
	}
	if got := idx.PendingDocs(); len(got) != 1 || got[0].ID() != other {
		t.Fatalf("pending docs %v", got)
	}
	// A new file under the same source is pending again.
	tv.Write("tool/source-core/documents/DINOv2.md", strings.Replace(tv.Read("tool/source-core/documents/DINOv2.md"), "3f9c1e2a7b8d44", "99887766554433", 1))
	if idx := tv.Index(); !idx.Pending(idx.ByID(src)) {
		t.Fatal("a source with a new hash is pending")
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
	tv.Doc("repository", "p3-edge", map[string]any{"path": repo}, "")
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
	tv.Write(".obsidian/workspace.json", `{"main":{"id":"a","type":"split","children":[{"id":"b","type":"tabs","children":[{"id":"leaf1","type":"leaf","state":{"type":"markdown","state":{"file":"tool/source-core/documents/X.md"}}}]}]},"active":"leaf1","lastOpenFiles":["Other.md"]}`)
	if got := tv.V.OpenNote(); got != "tool/source-core/documents/X.md" {
		t.Fatalf("open note %q", got)
	}
}

func TestSelectTakesTheNamedVaultThenAlmagestVaultThenTheFolder(t *testing.T) {
	one, two := testvault.New(t), testvault.New(t)
	if v, err := vault.Select("", one.V.Root, one.Home, two.V.Root); err != nil || v.Root != two.V.Root {
		t.Fatalf("ALMAGEST_VAULT as a path: %v %v", v, err)
	}
	if v, err := vault.Select("", "/", one.Home, "work"); err != nil || v.Root != one.V.Root {
		t.Fatalf("ALMAGEST_VAULT as a name: %v %v", v, err)
	}
	if v, err := vault.Select(one.V.Root, two.V.Root, one.Home, two.V.Root); err != nil || v.Root != one.V.Root {
		t.Fatalf("a named vault beats ALMAGEST_VAULT: %v %v", v, err)
	}
	if v, err := vault.Select("", one.V.Root, one.Home, ""); err != nil || v.Root != one.V.Root {
		t.Fatalf("no ALMAGEST_VAULT: the folder's vault: %v %v", v, err)
	}
	if _, err := vault.Select("", one.V.Root, one.Home, filepath.Join(t.TempDir(), "none")); err == nil || !strings.Contains(err.Error(), "ALMAGEST_VAULT=") {
		t.Fatalf("a bad ALMAGEST_VAULT: %v", err)
	}
}
