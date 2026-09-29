package scope_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/change"
	"github.com/nathanaday/atlas-obsidian/internal/core"
	"github.com/nathanaday/atlas-obsidian/internal/lint"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

func TestChainsCalloutsAndTheMap(t *testing.T) {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", nil)
	tv.Page("area", "work", nil, "")
	tv.Page("area", "p3", map[string]any{"parent": "[[work]]"}, "The p3 product.\n")
	tv.Page("repository", "p3-edge", map[string]any{"parent": "[[p3]]", "path": repo}, "")
	tv.Page("concept", "Motion scoring", map[string]any{"scope": "[[p3-edge]]"}, "")
	tv.Page("concept", "Vault wide", nil, "")
	synced, err := core.Sync(tv.V, testvault.Now)
	if err != nil {
		t.Fatal(err)
	}
	if len(synced.Scopes) != 6 {
		t.Fatalf("sync reports the pages it wrote: %v", synced.Scopes)
	}
	if got := tv.Read("wiki/work/p3/p3-edge/concepts/Motion scoring.md"); !strings.Contains(got, `chain: ["[[work]]", "[[p3]]", "[[p3-edge]]"]`) {
		t.Fatalf("chain:\n%s", got)
	}
	if got := tv.Read("wiki/concepts/Vault wide.md"); !strings.Contains(got, "chain: []") {
		t.Fatalf("a vault page has an empty chain:\n%s", got)
	}
	area := tv.Read("wiki/work/p3/p3.md")
	for _, want := range []string{`chain: ["[[work]]"]`, "> [!area] p3\n> [[Atlas|Work]] → [[work]] → **p3**", "> ```base\n> filters:", ">     - 'chain.contains(this.file.asLink())'", "The p3 product."} {
		if !strings.Contains(area, want) {
			t.Errorf("area lacks %q:\n%s", want, area)
		}
	}
	atlas := tv.Read("Atlas.md")
	if !strings.Contains(atlas, "> [!atlas] Work\n> - [[work]]\n>     - [[p3]]\n>         - [[p3-edge]]") {
		t.Fatalf("map:\n%s", atlas)
	}
	if tv.V.Context() != "Work notes: the p3 product." {
		t.Fatalf("the map is not the user's context: %q", tv.V.Context())
	}
	if f, _ := lint.Run(tv.Index(), lint.Options{Now: testvault.Now}); f.Counts[lint.Error] != 0 {
		t.Fatalf("lint: %+v", f.Findings)
	}
}

func TestANewParentMovesThePagesBelowInOneCommit(t *testing.T) {
	tv := testvault.New(t)
	tv.Page("area", "p3", nil, "")
	tv.Page("area", "work", nil, "")
	tv.Page("concept", "Motion scoring", map[string]any{"scope": "[[p3]]"}, "")
	if _, err := core.Sync(tv.V, testvault.Now); err != nil {
		t.Fatal(err)
	}
	tv.Commit()
	pv, err := change.Propose(tv.V, change.Plan{Title: "Move p3 under work", Writes: []change.Write{{Op: "modify", ID: tv.ID("p3"), Fields: map[string]any{"parent": "work"}}}}, tv.Tick(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := change.Apply(tv.V, pv.Ref.ID, tv.Tick(time.Minute), nil); err != nil {
		t.Fatal(err)
	}
	if tv.V.Exists("wiki/p3") {
		t.Fatal("the old folder goes")
	}
	if got := tv.Read("wiki/work/p3/concepts/Motion scoring.md"); !strings.Contains(got, `chain: ["[[work]]", "[[p3]]"]`) || !strings.Contains(got, `scope: "[[p3]]"`) {
		t.Fatalf("the page below moves with its area:\n%s", got)
	}
	if got := tv.Read("wiki/work/p3/p3.md"); !strings.Contains(got, `parent: "[[work]]"`) {
		t.Fatalf("the area moves into its parent's folder:\n%s", got)
	}
	tv.Clean()
	pv, _ = change.Propose(tv.V, change.Plan{Title: "Rename p3", Writes: []change.Write{{Op: "rename", ID: tv.ID("p3"), Title: "p3 product"}}}, tv.Tick(time.Minute))
	if _, err := change.Apply(tv.V, pv.Ref.ID, tv.Tick(time.Minute), nil); err != nil {
		t.Fatal(err)
	}
	if got := tv.Read("wiki/work/p3 product/concepts/Motion scoring.md"); !strings.Contains(got, `chain: ["[[work]]", "[[p3 product]]"]`) {
		t.Fatalf("a rename rewrites the chain:\n%s", got)
	}
	if !strings.Contains(tv.Read("Atlas.md"), ">     - [[p3 product]]") {
		t.Fatal("the map follows the rename")
	}
	tv.Clean()
}

func TestSyncRemovesTheOldScopeBase(t *testing.T) {
	tv := testvault.New(t)
	tv.Write("wiki/Scope.base", vault.OldScopeBaseContent())
	if _, err := core.Sync(tv.V, testvault.Now); err != nil {
		t.Fatal(err)
	}
	if tv.V.Exists("wiki/Scope.base") {
		t.Fatal("an untouched Scope.base goes")
	}
	reformatted := strings.ReplaceAll(vault.OldScopeBaseContent(), "'", "")
	tv.Write("wiki/Scope.base", reformatted)
	core.Sync(tv.V, testvault.Now)
	if tv.V.Exists("wiki/Scope.base") {
		t.Fatal("a Scope.base Obsidian only reformatted goes too")
	}
	tv.Write("wiki/Scope.base", strings.Replace(vault.OldScopeBaseContent(), "In this scope", "My view", 1))
	if _, err := core.Sync(tv.V, testvault.Now); err != nil {
		t.Fatal(err)
	}
	if !tv.V.Exists("wiki/Scope.base") {
		t.Fatal("an edited Scope.base stays")
	}
}

func TestSyncUpgradesAnUneditedThreadsBase(t *testing.T) {
	tv := testvault.New(t)
	current := tv.Read("threads/Threads.base")
	if !strings.Contains(current, "name: By area") {
		t.Fatal("init ships the By area view")
	}
	tv.Write("threads/Threads.base", vault.OldThreadsBase())
	if _, err := core.Sync(tv.V, testvault.Now); err != nil {
		t.Fatal(err)
	}
	if tv.Read("threads/Threads.base") != current {
		t.Fatal("an untouched old Threads.base becomes the current one")
	}
	edited := strings.Replace(vault.OldThreadsBase(), "name: Blocked", "name: Stuck", 1)
	tv.Write("threads/Threads.base", edited)
	if _, err := core.Sync(tv.V, testvault.Now); err != nil {
		t.Fatal(err)
	}
	if tv.Read("threads/Threads.base") != edited {
		t.Fatal("an edited Threads.base stays")
	}
}

func TestAScopePageListsItsOpenThreads(t *testing.T) {
	tv := testvault.New(t)
	tv.Page("area", "p3", nil, "")
	if _, err := core.Sync(tv.V, testvault.Now); err != nil {
		t.Fatal(err)
	}
	area := tv.Read("wiki/p3/p3.md")
	for _, want := range []string{">     name: Threads", `>         - 'type == "stub"'`, ">         - file.inFolder(\"wiki\")"} {
		if !strings.Contains(area, want) {
			t.Errorf("area lacks %q:\n%s", want, area)
		}
	}
}
