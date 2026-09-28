package mcpserver_test

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/mcpserver"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/vault"
)

type client struct {
	t    *testing.T
	sess *mcp.ClientSession
}

func connect(t *testing.T, tv *testvault.T, dir string) *client {
	t.Helper()
	s := mcpserver.New(mcpserver.Options{Version: "test", Dir: dir, Getenv: func(k string) string {
		if k == vault.EnvHome {
			return tv.Home.Root
		}
		return ""
	}, Now: func() time.Time { return tv.Tick(time.Second) }})
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := s.MCP().Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	sess, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sess.Close() })
	return &client{t: t, sess: sess}
}

// call runs a tool; it fails the test on a tool error unless wantErr, and returns the
// structured result as a map, or the error text.
func (c *client) call(name string, args map[string]any, wantErr bool) (map[string]any, string) {
	c.t.Helper()
	res, err := c.sess.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		c.t.Fatalf("%s %v: protocol error %v", name, args["action"], err)
	}
	if res.IsError {
		var parts []string
		for _, content := range res.Content {
			if tc, ok := content.(*mcp.TextContent); ok {
				parts = append(parts, tc.Text)
			}
		}
		msg := strings.Join(parts, " ")
		if !wantErr {
			c.t.Fatalf("%s %v: %s", name, args["action"], msg)
		}
		return nil, msg
	}
	if wantErr {
		c.t.Fatalf("%s %v: no error", name, args["action"])
	}
	var out map[string]any
	data, _ := json.Marshal(res.StructuredContent)
	json.Unmarshal(data, &out)
	return out, ""
}

func dig(m map[string]any, keys ...string) any {
	var x any = m
	for _, k := range keys {
		mm, ok := x.(map[string]any)
		if !ok {
			return nil
		}
		x = mm[k]
	}
	return x
}

func TestEveryToolAndAction(t *testing.T) {
	tv := testvault.New(t)
	repo := tv.Repo("p3-edge", map[string]string{"CLAUDE.md": "Use gofmt.\n"})
	tv.Write("inbox/notes.md", "# Notes\n\nMotion scoring cuts false alarms.\n")
	c := connect(t, tv, tv.V.Root)

	tools, err := c.sess.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	want := mcpserver.ToolNames()
	sort.Strings(want)
	sort.Strings(names)
	if strings.Join(names, ",") != strings.Join(want, ",") || len(names) != 8 {
		t.Fatalf("tools %v", names)
	}

	st, _ := c.call("vault", map[string]any{}, false)
	if dig(st, "status", "vault", "name") != "Work" || len(dig(st, "status", "inbox").([]any)) != 1 {
		t.Fatalf("status %v", st)
	}

	// Link the repository through a change.
	pv, _ := c.call("change", map[string]any{"action": "propose", "title": "Link p3-edge", "writes": []any{
		map[string]any{"op": "create", "type": "area", "title": "p3", "fields": map[string]any{"description": "The p3 product."}},
		map[string]any{"op": "create", "type": "repository", "title": "p3-edge", "fields": map[string]any{"description": "The edge service.", "path": repo, "parent": "p3"}},
	}}, false)
	id := dig(pv, "ref", "id").(string)
	if dig(pv, "status") != "proposed" {
		t.Fatalf("propose %v", pv)
	}
	c.call("change", map[string]any{"id": id}, false)
	applied, _ := c.call("change", map[string]any{"action": "apply", "id": id}, false)
	if dig(applied, "commit") == "" || dig(applied, "status") != "applied" {
		t.Fatalf("apply %v", applied)
	}

	ctxOut, _ := c.call("context", map[string]any{"scope": "p3-edge"}, false)
	if dig(ctxOut, "repository", "branch") != "main" || len(dig(ctxOut, "instructions").([]any)) != 1 {
		t.Fatalf("context %v", ctxOut)
	}
	c.call("context", map[string]any{}, false)
	hits, _ := c.call("search", map[string]any{"text": "edge service", "types": []any{"repository"}}, false)
	if dig(hits, "total").(float64) != 1 {
		t.Fatalf("search %v", hits)
	}

	// A thread from stub to tasks.
	board, _ := c.call("thread", map[string]any{}, false)
	if dig(board, "board") == nil {
		t.Fatalf("board %v", board)
	}
	opened, _ := c.call("thread", map[string]any{"action": "open", "text": "Cut the false alarms.", "title": "Filter alarms", "scope": []any{"p3-edge"}}, false)
	thread := dig(opened, "view", "stub", "id").(string)
	c.call("thread", map[string]any{"action": "attach", "thread": thread}, false)
	c.call("thread", map[string]any{"action": "file", "thread": thread, "part": "spec", "text": "## Goal\n\nFewer alarms."}, false)
	c.call("thread", map[string]any{"action": "tasks", "thread": thread, "tasks": []any{map[string]any{"title": "Score boxes", "text": "## What\n\nScore.", "repository": "p3-edge"}}}, false)
	started, _ := c.call("thread", map[string]any{"action": "task", "thread": thread, "task": "T1", "do": "start"}, false)
	if dig(started, "commit") != nil {
		t.Fatal("start writes nothing")
	}
	c.call("thread", map[string]any{"action": "task", "thread": thread, "task": "T1", "do": "done", "result": "Done; commit abc."}, false)
	c.call("thread", map[string]any{"action": "set", "thread": thread, "priority": "high", "blocked": "waiting on data"}, false)
	shown, _ := c.call("thread", map[string]any{"action": "show", "thread": thread}, false)
	if dig(shown, "view", "next") != "receipt" || dig(shown, "view", "stub", "state", "priority") != "high" {
		t.Fatalf("show %v", shown)
	}
	c.call("thread", map[string]any{"action": "file", "thread": thread, "part": "receipt", "outcome": "completed", "text": "## Delivered\n\nScoring."}, false)
	c.call("thread", map[string]any{"action": "reopen", "thread": thread}, false)
	_, msg := c.call("thread", map[string]any{"action": "file", "thread": thread, "part": "plan", "text": "x"}, true)
	if !strings.Contains(msg, "file takes spec or receipt") {
		t.Fatalf("a refusal teaches: %s", msg)
	}

	// The pipeline's tools.
	captured, _ := c.call("source", map[string]any{"action": "capture", "inbox": []any{"notes.md"}, "scope": "p3-edge"}, false)
	src := dig(captured, "captured").([]any)[0].(map[string]any)
	srcID := dig(src, "ref", "id").(string)
	chunks, _ := c.call("source", map[string]any{"action": "chunks", "doc": srcID}, false)
	if len(dig(chunks, "chunks").([]any)) != 1 {
		t.Fatalf("chunks %v", chunks)
	}
	blob, _ := c.call("source", map[string]any{"action": "read", "doc": srcID, "chunk": 1}, false)
	if !strings.Contains(dig(blob, "blob", "content").(string), "Motion scoring") {
		t.Fatalf("read %v", blob)
	}
	m, _ := c.call("match", map[string]any{"items": []any{map[string]any{"doc": srcID, "chunk": 1, "items": []any{map[string]any{"type": "concept", "name": "Motion scoring", "claims": []any{map[string]any{"text": "cuts false alarms", "locator": "line 3"}}}}}}}, false)
	if dig(m, "subjects").([]any)[0].(map[string]any)["match"] != "new" {
		t.Fatalf("match %v", m)
	}
	f, _ := c.call("lint", map[string]any{}, false)
	if dig(f, "counts") == nil {
		t.Fatalf("lint %v", f)
	}
	synced, _ := c.call("vault", map[string]any{"action": "sync"}, false)
	if dig(synced, "synced") == nil {
		t.Fatalf("sync %v", synced)
	}

	// An unknown action and a missing document come back as tool errors.
	c.call("change", map[string]any{"action": "merge"}, true)
	c.call("change", map[string]any{"id": "chg-zzzzzz"}, true)
}

func TestMentionsAndInitFromTheServer(t *testing.T) {
	tv := testvault.New(t)
	tv.Write("Ideas.md", "- [ ] @atlas add these papers to the wiki\n- [x] @atlas done already\n")
	c := connect(t, tv, tv.V.Root)
	st, _ := c.call("vault", map[string]any{}, false)
	mentions := dig(st, "status", "mentions").([]any)
	if len(mentions) != 1 || dig(mentions[0].(map[string]any), "line").(float64) != 1 {
		t.Fatalf("mentions %v", mentions)
	}
	opened, _ := c.call("thread", map[string]any{"action": "open", "text": "add these papers to the wiki", "title": "Papers"}, false)
	_ = opened
	c.call("vault", map[string]any{"action": "mention", "note": "Ideas.md", "line": 1, "link": "Papers"}, false)
	if got := tv.Read("Ideas.md"); !strings.HasPrefix(got, "- [x] @atlas add these papers to the wiki → [[Papers]]") {
		t.Fatalf("closed: %q", got)
	}
	c.call("vault", map[string]any{"action": "mention", "note": "Ideas.md", "line": 1, "link": "Papers"}, true)

	other := connect(t, tv, tv.Dir)
	out, _ := other.call("vault", map[string]any{"action": "init", "name": "Home", "path": tv.Dir + "/home-notes"}, false)
	if dig(out, "status", "vault", "name") != "Home" {
		t.Fatalf("init %v", out)
	}
	byName, _ := other.call("vault", map[string]any{"vault": "Work"}, false)
	if dig(byName, "status", "vault", "name") != "Work" {
		t.Fatalf("a vault by name %v", byName)
	}
}
