package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/cli"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
	"github.com/nathanaday/atlas-obsidian/internal/views"
)

type run struct {
	t  *testing.T
	tv *testvault.T
}

// atlas runs the command in the vault with stdin and returns its exit code, stdout, and
// stderr.
func (r run) atlas(stdin string, args ...string) (int, string, string) {
	r.t.Helper()
	var out, errOut bytes.Buffer
	c := &cli.CLI{In: strings.NewReader(stdin), Out: &out, Err: &errOut, Dir: r.tv.V.Root,
		Getenv: func(k string) string {
			if k == vault.EnvHome {
				return r.tv.Home.Root
			}
			return ""
		}, Now: func() time.Time { return r.tv.Tick(time.Second) }}
	code := c.Run(args)
	return code, out.String(), errOut.String()
}

func (r run) ok(stdin string, args ...string) string {
	r.t.Helper()
	code, out, errOut := r.atlas(stdin, args...)
	if code != 0 {
		r.t.Fatalf("atlas %s: exit %d: %s", strings.Join(args, " "), code, errOut)
	}
	return out
}

func TestCommands(t *testing.T) {
	tv := testvault.New(t)
	r := run{t: t, tv: tv}
	repo := tv.Repo("p3-edge", nil)
	tv.Write("ingest/notes.md", "# Notes\n\nMotion scoring.\n")

	if out := r.ok("", "vault"); !strings.Contains(out, "Work · ") || !strings.Contains(out, "Documents: 0 topic") {
		t.Fatalf("status:\n%s", out)
	}
	var status struct{ Status map[string]any }
	json.Unmarshal([]byte(r.ok("", "vault", "status", "--json")), &status)
	if _, ok := status.Status["threads"]; ok || status.Status["vault"] == nil || status.Status["documents"] == nil {
		t.Fatalf("status --json: %v", status)
	}
	plan := `{"title": "Link p3-edge", "writes": [
		{"op": "create", "type": "topic", "kind": "overview", "title": "P3", "fields": {"description": "The p3 product.", "defines": "work/p3"}},
		{"op": "create", "type": "repository", "title": "p3-edge", "fields": {"description": "The edge service.", "path": "` + repo + `", "defines": "work/p3/p3-edge", "tags": ["work/p3"]}}]}`
	var pv struct {
		Ref struct{ ID, Title string }
	}
	json.Unmarshal([]byte(r.ok(plan, "change", "propose", "-", "--json")), &pv)
	if !strings.HasPrefix(pv.Ref.ID, "chg-") {
		t.Fatalf("propose --json: %+v", pv)
	}
	if out := r.ok("", "change", "show", pv.Ref.Title); !strings.Contains(out, "proposed · 2 create") {
		t.Fatalf("show:\n%s", out)
	}
	if out := r.ok("", "change", "apply", pv.Ref.ID); !strings.Contains(out, "applied") || !strings.Contains(out, "commit ") {
		t.Fatalf("apply:\n%s", out)
	}
	if out := r.ok("", "vault"); !strings.Contains(out, "1 topic") || !strings.Contains(out, "Tags: work 2") {
		t.Fatalf("status after:\n%s", out)
	}
	if out := r.ok("", "context", "p3-edge"); !strings.Contains(out, "Page of work/p3: P3") || !strings.Contains(out, "Repository: ") {
		t.Fatalf("context:\n%s", out)
	}
	if out := r.ok("", "search", "edge", "service", "--type", "repository"); !strings.Contains(out, "p3-edge") {
		t.Fatalf("search:\n%s", out)
	}
	if out := r.ok("", "search", "--tag", "work"); !strings.Contains(out, "2 of 2") || !strings.Contains(out, "with: ") {
		t.Fatalf("search by tag:\n%s", out)
	}

	out := r.ok("", "source", "capture", "--ingest", "notes.md", "--tag", "work/p3/p3-edge")
	if !strings.Contains(out, "captured: notes (doc-") {
		t.Fatalf("capture:\n%s", out)
	}
	if out := r.ok("", "source", "chunks", "notes"); !strings.Contains(out, "1/1  whole") {
		t.Fatalf("chunks:\n%s", out)
	}
	if out := r.ok("", "source", "read", "notes", "1"); !strings.Contains(out, "Motion scoring.") {
		t.Fatalf("read:\n%s", out)
	}
	if out := r.ok("", "match", "--tag", "work", "--across"); !strings.Contains(out, `"subjects"`) {
		t.Fatalf("match:\n%s", out)
	}
	if out := r.ok("", "lint", "--tag", "work"); !strings.Contains(out, "documents") {
		t.Fatalf("lint:\n%s", out)
	}
	if out := r.ok("", "vault", "sync"); !strings.Contains(out, "Synced:") {
		t.Fatalf("sync:\n%s", out)
	}
	r.ok("", "vault", "sync", "--views")
	if !tv.V.Exists("wiki-view/View · Home.md") {
		t.Fatal("the views")
	}
	if code, _, errOut := r.atlas("", "vault", "migrate", "--dry-run"); code != 1 || !strings.Contains(errOut, "10.0 layout already") {
		t.Fatalf("a 10.0 vault: %d %s", code, errOut)
	}

	code, _, errOut := r.atlas("", "change", "apply", "chg-zzzzzz")
	if code != 1 || !strings.HasPrefix(errOut, "atlas: ") {
		t.Fatalf("an error: %d %q", code, errOut)
	}
	if code, _, _ := r.atlas("", "nothing"); code != 1 {
		t.Fatal("an unknown command fails")
	}
}

func TestThreadAndChordAreNoCommands(t *testing.T) {
	tv := testvault.New(t)
	r := run{t: t, tv: tv}
	for _, args := range [][]string{{"thread"}, {"thread", "stub", "An idea."}, {"chord", "list"}} {
		code, out, errOut := r.atlas("", args...)
		if code != 1 || out != "" || errOut != "atlas: no command \""+args[0]+"\"; atlas-obsidian help lists them\n" {
			t.Fatalf("%v: exit %d %q %q", args, code, out, errOut)
		}
	}
	if code, _, errOut := r.atlas("", "source", "capture", "--ingest", "x.md", "--resolves", "An idea"); code != 1 || !strings.Contains(errOut, "--resolves") {
		t.Fatalf("capture with --resolves: exit %d %q", code, errOut)
	}
	for _, args := range [][]string{{"help"}, {"help", "thread"}} {
		if out := r.ok("", args...); strings.Contains(out, "thread") || strings.Contains(out, "chord") || strings.Contains(out, "--resolves") {
			t.Fatalf("%v names a removed command:\n%s", args, out)
		}
	}
}

func TestMigrateCommand(t *testing.T) {
	tv := testvault.New(t)
	r := run{t: t, tv: tv}
	tv.Write("Atlas.md", strings.Replace(tv.Read("Atlas.md"), "layout: 6", "layout: 5", 1))
	topic := "---\nid: doc-p3aaaa\ntype: topic\nkind: overview\ndescription: P3.\ncreated: 2026-09-03T10:00:00\nupdated: 2026-09-03T10:00:00\n---\n\n## Summary\n\nThe stack, as drawn: ![[wiki/assets/diagram.png]]. Agents read wiki/documents.\n"
	tv.Write("wiki/documents/P3.md", topic)
	tv.Write("wiki/assets/diagram.png", "png")
	tv.Write("inbox/paper.pdf", "%PDF")
	tv.Write("views/View · Home.md", views.Notice+"\n\nHome.\n")
	tv.Write("views/My note.md", "mine\n")
	tv.Commit()
	out := r.ok("", "vault", "migrate", "--dry-run")
	for _, want := range []string{"from the 9.0 layout to 10.0 would make these moves", "move  wiki/documents/ → source-core/documents/: 1 file", "move  wiki/assets/ → source-core/originals/: 1 file", "move  inbox/ → ingest/: 1 file", "move  views/ → ingest/: 1 file", "remove  1 view"} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry run lacks %q:\n%s", want, out)
		}
	}
	if !tv.V.Exists("wiki/documents/P3.md") || tv.V.Exists("source-core/documents/P3.md") {
		t.Fatal("the dry run moved a file")
	}
	out = r.ok("", "vault", "migrate")
	if !strings.Contains(out, "Migrated Work from the 9.0 layout to 10.0 in one commit") {
		t.Fatalf("migrate:\n%s", out)
	}
	want := strings.Replace(topic, "![[wiki/assets/diagram.png]]", "![[source-core/originals/diagram.png]]", 1)
	if got := tv.Read("source-core/documents/P3.md"); !strings.Contains(got, strings.TrimPrefix(want[strings.Index(want, "## Summary"):], "")) {
		t.Fatalf("the topic:\n%s", got)
	}
	for _, rel := range []string{"source-core/originals/diagram.png", "ingest/paper.pdf", "ingest/My note.md"} {
		if !tv.V.Exists(rel) {
			t.Errorf("%s is missing", rel)
		}
	}
	for _, rel := range []string{"wiki", "inbox", "views"} {
		if tv.V.Exists(rel) {
			t.Errorf("%s/ is still there", rel)
		}
	}
	if !strings.Contains(tv.Read("Atlas.md"), "\nlayout: 6\n") || !tv.V.Exists("wiki-view/View · Home.md") {
		t.Fatalf("Atlas.md or the views:\n%s", tv.Read("Atlas.md"))
	}
}

func TestHookCommandReadsStdin(t *testing.T) {
	tv := testvault.New(t)
	r := run{t: t, tv: tv}
	event := `{"session_id": "abcdef12-0000", "cwd": "` + tv.V.Root + `", "tool_name": "Write", "tool_input": {"file_path": "` + tv.V.Root + `/source-core/documents/X.md"}}`
	if out := r.ok(event, "hook", "guard"); !strings.Contains(out, `"permissionDecision":"deny"`) {
		t.Fatalf("guard:\n%s", out)
	}
	if code, _, _ := r.atlas("", "hook", "nothing"); code != 1 {
		t.Fatal("an unknown hook fails")
	}
}

func TestConfigCommand(t *testing.T) {
	tv := testvault.New(t)
	r := run{t: t, tv: tv}
	r.ok("", "config", "set", "terminal", "wezterm", "--global")
	r.ok("", "config", "set", "agent_commands.claude", "claude-work")
	var view struct {
		Preferences struct {
			AgentCommand string `json:"agent_command"`
			Terminal     string
			Sources      map[string]string
		}
		Vault *struct {
			AgentCommands map[string]string `json:"agent_commands"`
		}
	}
	json.Unmarshal([]byte(r.ok("", "config", "--json")), &view)
	if view.Preferences.AgentCommand != "claude-work" || view.Preferences.Sources["agent_commands.claude"] != "vault" ||
		view.Preferences.Terminal != "wezterm" || view.Preferences.Sources["terminal"] != "global" || view.Vault == nil {
		t.Fatalf("config: %+v", view)
	}
	if _, err := os.Stat(filepath.Join(tv.V.Root, ".atlas", "config.json")); err != nil {
		t.Fatalf("the vault's file: %v", err)
	}
	r.ok("", "config", "set", "terminal", "terminal")
	if out := r.ok("", "config"); !strings.Contains(out, "terminal") || !strings.Contains(out, "vault") {
		t.Fatalf("show:\n%s", out)
	}
	r.ok("", "config", "unset", "terminal")
	json.Unmarshal([]byte(r.ok("", "config", "--json")), &view)
	if view.Preferences.Terminal != "wezterm" {
		t.Fatalf("unset falls back to the global file: %+v", view)
	}
	if code, _, errOut := r.atlas("", "config", "set", "terminal", "kitty"); code == 0 || !strings.Contains(errOut, "kitty") {
		t.Fatalf("a bad value: %d %s", code, errOut)
	}
}

// A write's JSON output names each note its views step moved out of wiki-view/, so a caller
// that reads only stdout, such as the Obsidian plugin, learns where it went.
func TestTheJSONOfAWriteNamesANoteMovedOutOfViews(t *testing.T) {
	tv := testvault.New(t)
	tv.Write("wiki-view/Draft.md", "# Draft\n\nMine.\n")
	r := run{t, tv}
	plan := `{"title": "Add Idea", "writes": [{"op": "create", "type": "topic", "kind": "overview", "title": "Idea", "fields": {"description": "An idea."}}]}`
	code, out, errOut := r.atlas(plan, "change", "propose", "-", "--json")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	var got struct {
		Moved []vault.Moved `json:"moved_from_wiki_view"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Moved) != 1 || got.Moved[0].From != "wiki-view/Draft.md" || got.Moved[0].To != "ingest/Draft.md" {
		t.Fatalf("moved_from_wiki_view %+v in:\n%s", got.Moved, out)
	}
	if !strings.Contains(errOut, "Moved wiki-view/Draft.md to ingest/Draft.md") {
		t.Fatalf("stderr: %s", errOut)
	}
}

func TestTheCLITakesTheVaultFromAtlasVault(t *testing.T) {
	one, two := testvault.New(t), testvault.New(t)
	two.Doc("topic", "Only in the second vault", map[string]any{"kind": "overview"}, "")
	two.Commit()
	runIn := func(env map[string]string, args ...string) (int, string, string) {
		var out, errOut bytes.Buffer
		c := &cli.CLI{In: strings.NewReader(""), Out: &out, Err: &errOut, Dir: one.V.Root,
			Getenv: func(k string) string {
				if k == vault.EnvHome {
					return one.Home.Root
				}
				return env[k]
			}, Now: func() time.Time { return one.Tick(time.Second) }}
		return c.Run(args), out.String(), errOut.String()
	}
	if _, out, _ := runIn(map[string]string{vault.EnvVault: two.V.Root}, "search", "second", "vault", "--json"); !strings.Contains(out, "Only in the second vault") {
		t.Fatalf("ATLAS_VAULT did not choose the second vault:\n%s", out)
	}
	if _, out, _ := runIn(map[string]string{vault.EnvVault: two.V.Root}, "search", "second", "vault", "--json", "--vault", one.V.Root); strings.Contains(out, "Only in the second vault") {
		t.Fatalf("--vault did not beat ATLAS_VAULT:\n%s", out)
	}
	if code, _, errOut := runIn(map[string]string{vault.EnvVault: "/no/such/vault"}, "search", "second", "vault"); code == 0 || !strings.Contains(errOut, "ATLAS_VAULT=/no/such/vault") {
		t.Fatalf("a bad ATLAS_VAULT: exit %d %s", code, errOut)
	}
}

func TestConfigFailsOnAnAtlasVaultThatNamesNoVault(t *testing.T) {
	tv := testvault.New(t)
	runIn := func(dir string, env map[string]string, args ...string) (int, string, string) {
		var out, errOut bytes.Buffer
		c := &cli.CLI{In: strings.NewReader(""), Out: &out, Err: &errOut, Dir: dir,
			Getenv: func(k string) string {
				if k == vault.EnvHome {
					return tv.Home.Root
				}
				return env[k]
			}, Now: func() time.Time { return tv.Tick(time.Second) }}
		return c.Run(args), out.String(), errOut.String()
	}
	outside := t.TempDir()
	if code, _, errOut := runIn(outside, map[string]string{vault.EnvVault: "/no/such/vault"}, "config"); code == 0 || !strings.Contains(errOut, "ATLAS_VAULT=/no/such/vault") {
		t.Fatalf("config with a bad ATLAS_VAULT: exit %d %s", code, errOut)
	}
	if code, out, errOut := runIn(outside, nil, "config"); code != 0 || !strings.Contains(out, "No vault here") {
		t.Fatalf("config outside a vault with no ATLAS_VAULT: exit %d %s %s", code, out, errOut)
	}
}

func TestConfigSetGlobalWritesNothingOnABadAtlasVault(t *testing.T) {
	tv := testvault.New(t)
	runIn := func(env map[string]string, args ...string) (int, string) {
		var out, errOut bytes.Buffer
		c := &cli.CLI{In: strings.NewReader(""), Out: &out, Err: &errOut, Dir: t.TempDir(),
			Getenv: func(k string) string {
				if k == vault.EnvHome {
					return tv.Home.Root
				}
				return env[k]
			}, Now: func() time.Time { return tv.Tick(time.Second) }}
		return c.Run(args), errOut.String()
	}
	before, _ := os.ReadFile(tv.Home.ConfigPath())
	for _, args := range [][]string{{"config", "set", "terminal", "wezterm", "--global"}, {"config", "unset", "terminal", "--global"}} {
		if code, errOut := runIn(map[string]string{vault.EnvVault: "/no/such/vault"}, args...); code == 0 || !strings.Contains(errOut, "ATLAS_VAULT=/no/such/vault") {
			t.Fatalf("%v with a bad ATLAS_VAULT: exit %d %s", args, code, errOut)
		}
		if after, _ := os.ReadFile(tv.Home.ConfigPath()); string(after) != string(before) {
			t.Fatalf("%v wrote the global file before it failed", args)
		}
	}
	if code, errOut := runIn(nil, "config", "set", "terminal", "wezterm", "--global"); code != 0 {
		t.Fatalf("config set --global outside a vault: exit %d %s", code, errOut)
	}
	if after, _ := os.ReadFile(tv.Home.ConfigPath()); !strings.Contains(string(after), "wezterm") {
		t.Fatal("config set --global wrote nothing")
	}
}

func TestConfigHintsGlobalOnlyWhenNoVaultWasNamed(t *testing.T) {
	tv := testvault.New(t)
	runIn := func(env map[string]string, args ...string) (int, string, string) {
		var out, errOut bytes.Buffer
		c := &cli.CLI{In: strings.NewReader(""), Out: &out, Err: &errOut, Dir: t.TempDir(),
			Getenv: func(k string) string {
				if k == vault.EnvHome {
					return tv.Home.Root
				}
				return env[k]
			}, Now: func() time.Time { return tv.Tick(time.Second) }}
		return c.Run(args), out.String(), errOut.String()
	}
	if code, _, errOut := runIn(map[string]string{vault.EnvVault: "/no/such/vault"}, "config", "set", "terminal", "ghostty"); code == 0 || strings.Contains(errOut, "--global") {
		t.Fatalf("a bad ATLAS_VAULT: exit %d, and the hint to add --global, which fails the same way: %s", code, errOut)
	}
	if code, _, errOut := runIn(nil, "config", "set", "terminal", "ghostty"); code == 0 || !strings.Contains(errOut, "add --global") {
		t.Fatalf("no vault anywhere: exit %d, want the --global hint: %s", code, errOut)
	}
	if code, out, errOut := runIn(map[string]string{vault.EnvVault: "  "}, "config"); code != 0 || !strings.Contains(out, "No vault here") {
		t.Fatalf("an ATLAS_VAULT of spaces counts as unset, as vault.Select reads it: exit %d %s %s", code, out, errOut)
	}
}
