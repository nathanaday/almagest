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

	out := r.ok("", "work", "stub", "Cut", "the", "false", "alarms", "--title", "Filter alarms", "--tag", "work/p3/p3-edge", "--priority", "high")
	if !strings.Contains(out, "Filter alarms (doc-") {
		t.Fatalf("stub:\n%s", out)
	}
	r.ok("## Goal\n\nFewer alarms.\n\n## Done when\n\n- half\n", "work", "promote", "Filter alarms", "-", "--kind", "plan", "--repository", "p3-edge")
	specs := `{"specs": [{"title": "Score boxes", "kind": "plan", "parent": "Filter alarms", "repositories": ["p3-edge"], "text": "## Done when\n\n- scored\n"}]}`
	specsFile := filepath.Join(tv.Dir, "specs.json")
	os.WriteFile(specsFile, []byte(specs), 0o644)
	r.ok("", "work", "spec", specsFile)
	r.ok("", "work", "start", "Score boxes")
	r.ok(`{"delivered": "Scored.", "verified": "Tests pass."}`, "work", "done", "Score boxes", "-")
	r.ok("", "work", "block", "Filter alarms", "--reason", "data")
	if out := r.ok("", "work"); !strings.Contains(out, "blocked (1)") || !strings.Contains(out, "blocked: data") {
		t.Fatalf("board:\n%s", out)
	}
	if out := r.ok("", "work", "show", "Filter alarms"); !strings.Contains(out, "next: done") || !strings.Contains(out, "part     Score boxes · done") {
		t.Fatalf("show:\n%s", out)
	}
	if !strings.Contains(tv.Read("wiki/documents/Score boxes.md"), "by: user") && !strings.Contains(tv.Read("wiki/documents/Score boxes.md"), "Done") {
		t.Fatal("the CLI acts as the user")
	}

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
	if !tv.V.Exists("views/View · Work.md") {
		t.Fatal("the views")
	}
	if code, _, errOut := r.atlas("", "vault", "migrate", "--dry-run"); code != 1 || !strings.Contains(errOut, "7.0 layout already") {
		t.Fatalf("a 7.0 vault: %d %s", code, errOut)
	}

	code, _, errOut := r.atlas("", "change", "apply", "chg-zzzzzz")
	if code != 1 || !strings.HasPrefix(errOut, "atlas: ") {
		t.Fatalf("an error: %d %q", code, errOut)
	}
	if code, _, _ := r.atlas("", "nothing"); code != 1 {
		t.Fatal("an unknown command fails")
	}
}

func TestMigrateCommand(t *testing.T) {
	tv := testvault.New(t)
	r := run{t: t, tv: tv}
	tv.Write("Atlas.md", strings.Replace(strings.Replace(tv.Read("Atlas.md"), "layout: 3", "layout: 2", 1), "tagging: open", "areas: manual", 1))
	tv.Write("wiki/p3/p3.md", "---\nid: are-p3aaaa\ntype: area\ncreated: 2026-09-01\nupdated: 2026-09-01\nparent: \"\"\ndescription: The p3 product.\n---\n\nAbout p3.\n")
	tv.Write("threads/Idea/Idea.md", "---\nid: thr-idea01\ntype: stub\ncreated: 2026-09-03\nupdated: 2026-09-03\nscope: [\"[[p3]]\"]\nstage: stub\n---\n\n## Stub\n\nAn idea.\n")
	tv.Commit()
	out := r.ok("", "vault", "migrate", "--dry-run")
	if !strings.Contains(out, "would make these moves") || !strings.Contains(out, "tag  p3 ← p3") {
		t.Fatalf("dry run:\n%s", out)
	}
	out = r.ok("", "vault", "migrate")
	if !strings.Contains(out, "Migrated Work to the 7.0 layout in one commit") {
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
