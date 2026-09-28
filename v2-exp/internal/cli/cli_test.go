package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/cli"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/vault"
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

	if out := r.ok("", "vault"); !strings.Contains(out, "Work · ") || !strings.Contains(out, "Scopes: 0 areas, 0 repositories") {
		t.Fatalf("status:\n%s", out)
	}
	plan := `{"title": "Link p3-edge", "writes": [
		{"op": "create", "type": "area", "title": "p3", "fields": {"description": "The p3 product."}},
		{"op": "create", "type": "repository", "title": "p3-edge", "fields": {"description": "The edge service.", "path": "` + repo + `", "parent": "p3"}}]}`
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
	if out := r.ok("", "vault"); !strings.Contains(out, "Scopes: 1 area, 1 repository") {
		t.Fatalf("a plural of one:\n%s", out)
	}
	if out := r.ok("", "context", "p3-edge"); !strings.Contains(out, "Work → p3 → p3-edge") {
		t.Fatalf("context:\n%s", out)
	}
	if out := r.ok("", "search", "edge", "service", "--type", "repository"); !strings.Contains(out, "p3-edge") {
		t.Fatalf("search:\n%s", out)
	}

	out := r.ok("", "thread", "open", "Cut", "the", "false", "alarms", "--title", "Filter alarms", "--scope", "p3-edge", "--priority", "high")
	if !strings.Contains(out, "Filter alarms (thr-") {
		t.Fatalf("open:\n%s", out)
	}
	r.ok("## Goal\n\nFewer alarms.\n", "thread", "file", "Filter alarms", "spec")
	tasks := `[{"title": "Score boxes", "text": "## What\n\nScore.", "repository": "p3-edge"}]`
	tasksFile := filepath.Join(tv.Dir, "tasks.json")
	os.WriteFile(tasksFile, []byte(tasks), 0o644)
	r.ok("", "thread", "tasks", "Filter alarms", tasksFile)
	r.ok("", "thread", "task", "T1", "done", "--thread", "Filter alarms", "--result", "Scored.")
	r.ok("", "thread", "set", "Filter alarms", "--blocked", "data")
	if out := r.ok("", "thread"); !strings.Contains(out, "tasks (1)") || !strings.Contains(out, "blocked: data") {
		t.Fatalf("board:\n%s", out)
	}
	if out := r.ok("", "thread", "show", "Filter alarms"); !strings.Contains(out, "next: receipt") {
		t.Fatalf("show:\n%s", out)
	}

	out = r.ok("", "source", "capture", "--inbox", "notes.md", "--scope", "p3-edge")
	if !strings.Contains(out, "captured: notes (src-") {
		t.Fatalf("capture:\n%s", out)
	}
	if out := r.ok("", "source", "chunks", "notes"); !strings.Contains(out, "1/1  whole") {
		t.Fatalf("chunks:\n%s", out)
	}
	if out := r.ok("", "source", "read", "notes", "1"); !strings.Contains(out, "Motion scoring.") {
		t.Fatalf("read:\n%s", out)
	}
	if out := r.ok("", "match", "--pages", "notes"); !strings.Contains(out, `"subjects"`) {
		t.Fatalf("match:\n%s", out)
	}
	if out := r.ok("", "lint"); !strings.Contains(out, "documents") {
		t.Fatalf("lint:\n%s", out)
	}
	if out := r.ok("", "vault", "sync"); out == "" {
		t.Fatal("sync says what it did")
	}

	code, _, errOut := r.atlas("", "change", "apply", "chg-zzzzzz")
	if code != 1 || !strings.HasPrefix(errOut, "atlas: ") {
		t.Fatalf("an error: %d %q", code, errOut)
	}
	if code, _, _ := r.atlas("", "nothing"); code != 1 {
		t.Fatal("an unknown command fails")
	}
}

func TestHookCommandReadsStdin(t *testing.T) {
	tv := testvault.New(t)
	r := run{t: t, tv: tv}
	event := `{"session_id": "abcdef12-0000", "cwd": "` + tv.V.Root + `", "tool_name": "Write", "tool_input": {"file_path": "` + tv.V.Root + `/wiki/concepts/X.md"}}`
	if out := r.ok(event, "hook", "guard"); !strings.Contains(out, `"permissionDecision":"deny"`) {
		t.Fatalf("guard:\n%s", out)
	}
	if code, _, _ := r.atlas("", "hook", "nothing"); code != 1 {
		t.Fatal("an unknown hook fails")
	}
}
