package mcpserver_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
)

func TestACaptureWhoseNameCleansToNothingKeepsTheServerAnswering(t *testing.T) {
	tv := testvault.New(t)
	tv.Write("inbox/###.txt", "notes with no name\n")
	tv.Write("inbox/"+strings.Repeat("n", 250)+".txt", "notes with a long name\n")
	c := connect(t, tv, tv.V.Root)
	out, _ := c.call("source", map[string]any{"action": "capture", "inbox": []any{"###.txt", strings.Repeat("n", 250) + ".txt"}}, false)
	captured, _ := out["captured"].([]any)
	if len(captured) != 2 {
		t.Fatalf("captured %v", out)
	}
	first := dig(captured[0].(map[string]any), "ref")
	id, _ := dig(first.(map[string]any), "id").(string)
	title, _ := dig(first.(map[string]any), "title").(string)
	if id == "" || title != id {
		t.Fatalf("the nameless capture took the title %q, id %q", title, id)
	}
	long, _ := dig(captured[1].(map[string]any), "ref", "title").(string)
	if long == "" || len(long) > 150 {
		t.Fatalf("the long name became a title of %d bytes", len(long))
	}
	if tv.Index().ByID(id) == nil {
		t.Fatal("the index does not hold the capture")
	}
	found, _ := c.call("search", map[string]any{"text": id, "types": []any{"source"}}, false)
	if hits, _ := found["hits"].([]any); len(hits) == 0 {
		t.Fatalf("search after the capture: %v", found)
	}
}

func TestTheMentionToolRefusesANoteOutsideTheVault(t *testing.T) {
	tv := testvault.New(t)
	victim := filepath.Join(filepath.Dir(tv.V.Root), "outside", "victim.md")
	tv.WriteFile(victim, "- [ ] @atlas track this\n")
	tv.Write("Ideas.md", "- [ ] @atlas track this\n")
	c := connect(t, tv, tv.V.Root)
	c.call("thread", map[string]any{"action": "stub", "text": "Track this.", "title": "Idea"}, false)
	_, msg := c.call("vault", map[string]any{"action": "mention", "note": "../outside/victim.md", "line": 1, "link": "Idea"}, true)
	if !strings.Contains(msg, "is no note of the vault") {
		t.Fatalf("the refusal: %s", msg)
	}
	if data, _ := os.ReadFile(victim); string(data) != "- [ ] @atlas track this\n" {
		t.Fatalf("the outside note changed: %q", data)
	}
	c.call("vault", map[string]any{"action": "mention", "note": "Ideas.md", "line": 1, "link": "Idea"}, false)
}

func TestNoReadFollowsALinkOutOfTheRepositoryOrTheVault(t *testing.T) {
	tv := testvault.New(t)
	away := filepath.Join(filepath.Dir(tv.V.Root), "outside")
	tv.WriteFile(filepath.Join(away, "rules.md"), "OUTSIDE-RULES\n")
	tv.WriteFile(filepath.Join(away, "notes.txt"), "TODO: OUTSIDE-MARKER\n")
	tv.WriteFile(filepath.Join(away, "creds.yml"), "OUTSIDE-CREDS\n")
	repo := tv.Repo("p3-edge", map[string]string{"main.go": "package main\n// TODO: score boxes\n"})
	for name, target := range map[string]string{"CLAUDE.md": filepath.Join(away, "rules.md"), "notes.txt": filepath.Join(away, "notes.txt")} {
		if err := os.Symlink(target, filepath.Join(repo, name)); err != nil {
			t.Fatal(err)
		}
	}
	g := gitx.Repo{Dir: repo}
	if err := g.AddAll(); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Commit("links out"); err != nil {
		t.Fatal(err)
	}
	tv.Doc("repository", "p3-edge", map[string]any{"path": repo}, "")
	creds := tv.Doc("source", "Creds", map[string]any{"file": "[[../../../outside/creds.yml]]", "sha256": "abcdef0123456789", "media": "text"}, "")
	tv.Commit()
	c := connect(t, tv, tv.V.Root)

	ctx, _ := c.call("context", map[string]any{"repository": "p3-edge"}, false)
	if data, _ := json.Marshal(ctx); strings.Contains(string(data), "OUTSIDE") {
		t.Fatalf("context read a file outside the repository: %s", data)
	}
	snap, _ := c.call("source", map[string]any{"action": "capture", "repository": "p3-edge"}, false)
	id, _ := dig(snap["captured"].([]any)[0].(map[string]any), "ref", "id").(string)
	report := tv.Read("wiki/assets/" + id + ".md")
	if strings.Contains(report, "OUTSIDE") || !strings.Contains(report, "score boxes") {
		t.Fatalf("the snapshot:\n%s", report)
	}
	for _, action := range []string{"chunks", "read"} {
		_, msg := c.call("source", map[string]any{"action": action, "doc": creds, "chunk": 1}, true)
		if !strings.Contains(msg, "names no file of wiki/assets/") {
			t.Errorf("%s: %s", action, msg)
		}
	}
}

func TestAWriteToolSaysWhereANoteFromViewsWent(t *testing.T) {
	tv := testvault.New(t)
	tv.Write("views/Draft.md", "# Draft\n\nMine.\n")
	c := connect(t, tv, tv.V.Root)
	out, _ := c.call("thread", map[string]any{"action": "stub", "text": "An idea.", "title": "Idea"}, false)
	moved, _ := out["moved_from_views"].([]any)
	if len(moved) != 1 {
		t.Fatalf("the result does not name the move: %v", out)
	}
	m := moved[0].(map[string]any)
	if m["from"] != "views/Draft.md" || m["to"] != "inbox/Draft.md" {
		t.Fatalf("moved %v", m)
	}
	if tv.Read("inbox/Draft.md") != "# Draft\n\nMine.\n" {
		t.Fatal("the note is not in the inbox")
	}
}
