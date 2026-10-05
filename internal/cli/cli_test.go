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
	tv.Write("inbox/notes.md", "# Notes\n\nMotion scoring.\n")

	if out := r.ok("", "vault"); !strings.Contains(out, "Work · ") || !strings.Contains(out, "Documents: 0 topic") {
		t.Fatalf("status:\n%s", out)
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

	out := r.ok("", "thread", "stub", "Cut", "the", "false", "alarms", "--title", "Filter alarms", "--tag", "work/p3/p3-edge", "--priority", "high")
	if !strings.Contains(out, "Filter alarms (doc-") || !strings.Contains(out, "next: the thread has no spec with requirements (thread-spec)") {
		t.Fatalf("stub:\n%s", out)
	}
	r.ok("## Goal\n\nFewer alarms.\n\n## Requirements\n\n- R1: Half as many alarms.\n", "thread", "spec", "Filter alarms", "-")
	file := func(name, content string) string {
		p := filepath.Join(tv.Dir, name)
		os.WriteFile(p, []byte(content), 0o644)
		return p
	}
	r.ok("", "thread", "tasks", "Filter alarms", file("tasks.json", `[{"text": "Score boxes", "requirements": ["R1"], "details": "In score.go."}]`), "--repository", "p3-edge")
	r.ok("", "thread", "start", "Filter alarms")
	r.ok("", "thread", "block", "Filter alarms", "--reason", "data")
	if out := r.ok("", "thread"); !strings.Contains(out, "blocked (1)") || !strings.Contains(out, "blocked: data") || !strings.Contains(out, "0/1 tasks") {
		t.Fatalf("board:\n%s", out)
	}
	r.ok("", "thread", "unblock", "Filter alarms")
	if out := r.ok("", "thread", "load", "Filter alarms"); !strings.Contains(out, "[ ] T1: Score boxes (R1)") || !strings.Contains(out, "hand off Resume Atlas thread doc-") || !strings.Contains(out, "next: the next open task is T1") {
		t.Fatalf("load:\n%s", out)
	}
	if out := r.ok("", "thread", "check", "Filter alarms", "T1", "--commit", "abc1234", "--note", "scored"); !strings.Contains(out, "unverified · 1/1 tasks") {
		t.Fatalf("check:\n%s", out)
	}
	verification := file("verify.json", `{"scope": "p3-edge at abc1234", "results": [{"requirement": "R1", "result": "pass", "evidence": "the log"}], "findings": ["The threshold is fixed."]}`)
	r.ok("", "thread", "verify", "Filter alarms", verification)
	if out := r.ok("", "thread", "finding", "Filter alarms", "F1", "--outcome", "accepted", "--reason", "Fine for now."); !strings.Contains(out, "verified · 1/1 tasks") || !strings.Contains(out, "(thread-close)") {
		t.Fatalf("finding:\n%s", out)
	}
	if got := tv.Read("wiki/documents/Filter alarms · Verification 1.md"); !strings.Contains(got, "by: user") || !strings.Contains(got, "- [x] F1: The threshold is fixed. → accepted: Fine for now.") {
		t.Fatalf("the CLI acts as the user:\n%s", got)
	}
	r.ok("", "thread", "set", "Filter alarms", "--priority", "low")
	r.ok("", "thread", "note", "Filter alarms", "--text", "A vendor call.")

	chord := file("chord.json", `{"title": "Quiet alarms", "text": "Alarms the user trusts.", "threads": [{"thread": "Filter alarms"}, {"title": "Tune per site", "text": "Each site gets its threshold.", "after": ["Filter alarms"]}]}`)
	if out := r.ok("", "chord", "create", chord); !strings.Contains(out, "chord Quiet alarms (doc-") || !strings.Contains(out, "Tune per site (doc-") || !strings.Contains(out, "after Filter alarms · ready") {
		t.Fatalf("chord:\n%s", out)
	}
	r.ok("", "thread", "stub", "Report the rate", "--title", "Report")
	r.ok("", "chord", "add", "Quiet alarms", "Report", "--after", "Tune per site")
	r.ok("", "chord", "order", "Quiet alarms", file("order.json", `[{"thread": "Report", "after": ["Filter alarms"]}]`))
	if out := r.ok("", "chord", "canvas", "Quiet alarms"); !strings.Contains(out, "shows the order the stubs hold") {
		t.Fatalf("canvas:\n%s", out)
	}
	canvas := tv.Read("chords/Quiet alarms.canvas")
	tv.Write("chords/Quiet alarms.canvas", strings.Replace(canvas, `"edges": [`, `"edges": [{"id": "mine", "fromNode": "`+idOf(t, tv, "Tune per site")+`", "toNode": "`+idOf(t, tv, "Report")+`"},`, 1))
	if out := r.ok("", "chord", "canvas", "Quiet alarms"); !strings.Contains(out, "put Report after Tune per site") {
		t.Fatalf("canvas status:\n%s", out)
	}
	r.ok("", "chord", "canvas", "Quiet alarms", "--save")
	if got := tv.Read("wiki/documents/Report.md"); !strings.Contains(got, `after: ["[[Filter alarms]]", "[[Tune per site]]"]`) {
		t.Fatalf("saved order:\n%s", got)
	}
	r.ok("", "chord", "canvas", "Quiet alarms", "--tidy")
	r.ok("", "chord", "remove", "Quiet alarms", "Report")
	if out := r.ok("", "chord"); !strings.Contains(out, "chord Quiet alarms") || strings.Contains(out, "Report") {
		t.Fatalf("chords:\n%s", out)
	}
	if out := r.ok("", "chord", "load", "Quiet alarms"); !strings.Contains(out, "next: [[Filter alarms]] is verified") {
		t.Fatalf("chord load:\n%s", out)
	}
	r.ok("", "chord", "drop", "Quiet alarms", "--reason", "Later.")
	r.ok("", "chord", "reopen", "Quiet alarms")
	r.ok("", "thread", "drop", "Report", "--reason", "No.")
	r.ok("", "thread", "reopen", "Report")
	r.ok("", "thread", "resolve", "Report", "--became", "p3-edge")

	out = r.ok("", "source", "capture", "--inbox", "notes.md", "--tag", "work/p3/p3-edge")
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
	if !tv.V.Exists("views/View · Threads.md") {
		t.Fatal("the views")
	}
	if code, _, errOut := r.atlas("", "vault", "migrate", "--dry-run"); code != 1 || !strings.Contains(errOut, "8.0 layout already") {
		t.Fatalf("an 8.0 vault: %d %s", code, errOut)
	}

	code, _, errOut := r.atlas("", "change", "apply", "chg-zzzzzz")
	if code != 1 || !strings.HasPrefix(errOut, "atlas: ") {
		t.Fatalf("an error: %d %q", code, errOut)
	}
	if code, _, _ := r.atlas("", "nothing"); code != 1 {
		t.Fatal("an unknown command fails")
	}
}

func idOf(t *testing.T, tv *testvault.T, title string) string {
	t.Helper()
	d, err := tv.Index().Resolve(title)
	if err != nil {
		t.Fatal(err)
	}
	return d.ID()
}

func TestMigrateCommand(t *testing.T) {
	tv := testvault.New(t)
	r := run{t: t, tv: tv}
	tv.Write("Atlas.md", strings.Replace(strings.Replace(tv.Read("Atlas.md"), "layout: 4", "layout: 2", 1), "tagging: open", "areas: manual", 1))
	tv.Write("wiki/p3/p3.md", "---\nid: are-p3aaaa\ntype: area\ncreated: 2026-09-01\nupdated: 2026-09-01\nparent: \"\"\ndescription: The p3 product.\n---\n\nAbout p3.\n")
	tv.Write("threads/Idea/Idea.md", "---\nid: thr-idea01\ntype: stub\ncreated: 2026-09-03\nupdated: 2026-09-03\nscope: [\"[[p3]]\"]\nstage: stub\n---\n\n## Stub\n\nAn idea.\n")
	tv.Commit()
	out := r.ok("", "vault", "migrate", "--dry-run")
	if !strings.Contains(out, "would make these moves") || !strings.Contains(out, "tag  p3 ← p3") {
		t.Fatalf("dry run:\n%s", out)
	}
	out = r.ok("", "vault", "migrate")
	if !strings.Contains(out, "Migrated Work from the 6.x layout to 8.0 in one commit") {
		t.Fatalf("migrate:\n%s", out)
	}
	if got := tv.Read("wiki/documents/Idea.md"); !strings.Contains(got, "tags: [p3]") || !strings.Contains(tv.Read("Atlas.md"), "tagging: known") {
		t.Fatalf("migrated:\n%s", got)
	}
}

func TestHookCommandReadsStdin(t *testing.T) {
	tv := testvault.New(t)
	r := run{t: t, tv: tv}
	event := `{"session_id": "abcdef12-0000", "cwd": "` + tv.V.Root + `", "tool_name": "Write", "tool_input": {"file_path": "` + tv.V.Root + `/wiki/documents/X.md"}}`
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

// A write's JSON output names each note its views step moved out of views/, so a caller
// that reads only stdout, such as the Obsidian plugin, learns where it went.
func TestTheJSONOfAWriteNamesANoteMovedOutOfViews(t *testing.T) {
	tv := testvault.New(t)
	tv.Write("views/Draft.md", "# Draft\n\nMine.\n")
	r := run{t, tv}
	code, out, errOut := r.atlas("", "thread", "stub", "An idea.", "--title", "Idea", "--json")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	var got struct {
		Moved []vault.Moved `json:"moved_from_views"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Moved) != 1 || got.Moved[0].From != "views/Draft.md" || got.Moved[0].To != "inbox/Draft.md" {
		t.Fatalf("moved_from_views %+v in:\n%s", got.Moved, out)
	}
	if !strings.Contains(errOut, "Moved views/Draft.md to inbox/Draft.md") {
		t.Fatalf("stderr: %s", errOut)
	}
}

func TestTheCLITakesTheVaultFromAtlasVault(t *testing.T) {
	one, two := testvault.New(t), testvault.New(t)
	two.Doc("stub", "Only in the second vault", nil, "## Idea\n\nx\n")
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
	if _, out, _ := runIn(map[string]string{vault.EnvVault: two.V.Root}, "thread", "list", "--json"); !strings.Contains(out, "Only in the second vault") {
		t.Fatalf("ATLAS_VAULT did not choose the second vault:\n%s", out)
	}
	if _, out, _ := runIn(map[string]string{vault.EnvVault: two.V.Root}, "thread", "list", "--json", "--vault", one.V.Root); strings.Contains(out, "Only in the second vault") {
		t.Fatalf("--vault did not beat ATLAS_VAULT:\n%s", out)
	}
	if code, _, errOut := runIn(map[string]string{vault.EnvVault: "/no/such/vault"}, "thread", "list"); code == 0 || !strings.Contains(errOut, "ATLAS_VAULT=/no/such/vault") {
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
