package mcpserver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
