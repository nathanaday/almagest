package mcpserver_test

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/mcpserver"
	"github.com/nathanaday/atlas-obsidian/internal/testvault"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
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
	if strings.Join(names, ",") != strings.Join(want, ",") || len(names) != 9 {
		t.Fatalf("tools %v", names)
	}

	st, _ := c.call("vault", map[string]any{}, false)
	if dig(st, "status", "vault", "name") != "Work" || len(dig(st, "status", "inbox").([]any)) != 1 {
		t.Fatalf("status %v", st)
	}

	// Link the repository through a change.
	pv, _ := c.call("change", map[string]any{"action": "propose", "title": "Link p3-edge", "writes": []any{
		map[string]any{"op": "create", "type": "topic", "kind": "overview", "title": "P3", "fields": map[string]any{"description": "The p3 product.", "defines": "work/p3", "tags": []any{"work"}}},
		map[string]any{"op": "create", "type": "repository", "title": "p3-edge", "fields": map[string]any{"description": "The edge service.", "path": repo, "defines": "work/p3/p3-edge", "tags": []any{"work/p3"}}},
	}}, false)
	id := dig(pv, "ref", "id").(string)
	if dig(pv, "status") != "proposed" {
		t.Fatalf("propose %v", pv)
	}
	c.call("change", map[string]any{"id": id}, false)
	// The model's apply waits for the user of the session that proposed the change, even
	// from outside the vault and by another name for the change.
	path := dig(pv, "ref", "path").(string)
	session := "sessions/2026-09/2026-09-27 1432 a1b2c3"
	tv.Write(session+".md", "---\nid: ses-a1b2c3\ntype: session\nharness_id: a1b2c3\nlast_prompt: \"\"\n---\n")
	tv.Write(path, doc.SetField(tv.Read(path), "session", doc.Link("2026-09-27 1432 a1b2c3")))
	outside := connect(t, tv, t.TempDir())
	if _, msg := outside.call("change", map[string]any{"action": "apply", "id": dig(pv, "ref", "title").(string) + ".md", "vault": tv.V.Root}, true); !strings.Contains(msg, "wait for the user's yes") {
		t.Fatalf("the gate: %s", msg)
	}
	tv.Write(session+".md", "---\nid: ses-a1b2c3\ntype: session\nharness_id: a1b2c3\nlast_prompt: \""+vault.Stamp(tv.Tick(time.Minute))+"\"\n---\n")
	applied, _ := c.call("change", map[string]any{"action": "apply", "id": id}, false)
	if dig(applied, "commit") == "" || dig(applied, "status") != "applied" {
		t.Fatalf("apply %v", applied)
	}
	if !tv.V.Exists("views/tags/work/p3/Tag · work › p3.md") {
		t.Fatal("a write refreshes the views")
	}

	ctxOut, _ := c.call("context", map[string]any{"repository": "p3-edge"}, false)
	if dig(ctxOut, "repository", "branch") != "main" || len(dig(ctxOut, "instructions").([]any)) != 1 || len(dig(ctxOut, "pages").([]any)) != 1 {
		t.Fatalf("context %v", ctxOut)
	}
	c.call("context", map[string]any{"tags": []any{"work"}}, false)
	c.call("context", map[string]any{}, false)
	hits, _ := c.call("search", map[string]any{"text": "edge service", "types": []any{"repository"}}, false)
	if dig(hits, "total").(float64) != 1 {
		t.Fatalf("search %v", hits)
	}
	hits, _ = c.call("search", map[string]any{"tags": []any{"work"}}, false)
	if dig(hits, "total").(float64) != 2 || dig(hits, "facets", "tags", "work/p3/p3-edge") == nil {
		t.Fatalf("search by tag %v", hits)
	}

	// A thread from a stub to closed, through every action.
	board, _ := c.call("thread", map[string]any{}, false)
	if dig(board, "board") == nil {
		t.Fatalf("board %v", board)
	}
	stubbed, _ := c.call("thread", map[string]any{"action": "stub", "text": "Cut the false alarms.", "title": "Filter alarms", "tags": []any{"work/p3/p3-edge"}}, false)
	stub := dig(stubbed, "state", "thread", "id").(string)
	if len(dig(stubbed, "wrote").([]any)) != 1 || dig(stubbed, "state", "next", "skill") != "thread-spec" {
		t.Fatalf("stub %v", stubbed)
	}
	_, msg := c.call("thread", map[string]any{"action": "start", "thread": stub}, true)
	if !strings.Contains(msg, "write it first (thread-spec)") {
		t.Fatalf("a refusal teaches: %s", msg)
	}
	c.call("thread", map[string]any{"action": "spec", "thread": stub, "text": "## Goal\n\nFewer alarms.\n\n## Requirements\n\n- R1: Half as many alarms.\n\n## Knowledge\n\n- [[p3-edge]]\n"}, false)
	c.call("thread", map[string]any{"action": "tasks", "thread": stub, "repository": "p3-edge", "tasks": []any{map[string]any{"text": "Score boxes", "requirements": []any{"R1"}, "details": "In score.go."}}}, false)
	started, _ := c.call("thread", map[string]any{"action": "start", "thread": "Filter alarms"}, false)
	if dig(started, "started") != "Filter alarms" || len(dig(started, "events").([]any)) != 1 {
		t.Fatalf("start %v", started)
	}
	c.call("thread", map[string]any{"action": "block", "thread": stub, "reason": "waiting on data"}, false)
	c.call("thread", map[string]any{"action": "unblock", "thread": stub}, false)
	c.call("thread", map[string]any{"action": "set", "set": map[string]any{"doc": stub, "priority": "high"}}, false)
	c.call("thread", map[string]any{"action": "note", "thread": stub, "text": "A vendor call."}, false)
	loaded, _ := c.call("thread", map[string]any{"action": "load", "thread": stub}, false)
	if dig(loaded, "thread", "next", "step") != "run" || dig(loaded, "thread", "thread", "state", "priority") != "high" || dig(loaded, "thread", "handoff") != "Resume Atlas thread "+stub ||
		len(dig(loaded, "thread", "lists").([]any)) != 1 || len(dig(loaded, "thread", "knowledge").([]any)) != 1 || len(dig(loaded, "thread", "notes").([]any)) != 1 {
		t.Fatalf("load %v", loaded)
	}
	c.call("thread", map[string]any{"action": "check", "thread": stub, "task": "T1", "commits": []any{"abc1234"}}, false)
	failed, _ := c.call("thread", map[string]any{"action": "verify", "thread": stub, "scope": "p3-edge at abc1234", "results": []any{map[string]any{"requirement": "R1", "result": "fail", "evidence": "a third fewer"}}, "findings": []any{"The threshold is too low."}}, false)
	if dig(failed, "state", "next", "step") != "findings" {
		t.Fatalf("verify %v", failed)
	}
	c.call("thread", map[string]any{"action": "finding", "thread": stub, "finding": "F1", "outcome": "task", "repository": "p3-edge", "new_task": map[string]any{"text": "Raise the threshold", "requirements": []any{"R1"}}}, false)
	c.call("thread", map[string]any{"action": "check", "thread": stub, "task": "T2", "note": "raised in config"}, false)
	verified, _ := c.call("thread", map[string]any{"action": "verify", "thread": stub, "scope": "p3-edge at def5678", "results": []any{map[string]any{"requirement": "R1", "result": "pass", "evidence": "half as many in the log"}}}, false)
	if dig(verified, "state", "thread", "status") != "verified" || dig(verified, "state", "next", "skill") != "thread-close" {
		t.Fatalf("verified %v", verified)
	}
	idea, _ := c.call("thread", map[string]any{"action": "stub", "text": "Read the OTA paper", "title": "OTA paper"}, false)
	c.call("thread", map[string]any{"action": "drop", "thread": dig(idea, "state", "thread", "id"), "reason": "Not now."}, false)
	c.call("thread", map[string]any{"action": "reopen", "thread": "OTA paper", "reason": "Now."}, false)
	c.call("thread", map[string]any{"action": "resolve", "thread": "OTA paper", "became": []any{"p3-edge"}}, false)

	// A chord, through every action.
	made, _ := c.call("chord", map[string]any{"action": "create", "title": "Quiet alarms", "text": "Alarms the user trusts.", "tags": []any{"work/p3"}, "threads": []any{
		map[string]any{"thread": stub}, map[string]any{"title": "Tune per site", "text": "Each site gets its threshold.", "after": []any{"Filter alarms"}},
	}}, false)
	chord := dig(made, "view", "chord", "id").(string)
	if len(dig(made, "view", "threads").([]any)) != 2 || !tv.V.Exists("chords/Quiet alarms.canvas") {
		t.Fatalf("chord %v", made)
	}
	c.call("thread", map[string]any{"action": "stub", "text": "Report the alarm rate", "title": "Report"}, false)
	c.call("chord", map[string]any{"action": "add", "chord": chord, "thread": "Report", "after": []any{"Tune per site"}}, false)
	c.call("chord", map[string]any{"action": "order", "chord": chord, "order": []any{map[string]any{"thread": "Report", "after": []any{"Filter alarms"}}}}, false)
	c.call("chord", map[string]any{"action": "order", "chord": chord, "order": []any{map[string]any{"thread": "Filter alarms", "after": []any{"Report"}}}}, true)
	c.call("chord", map[string]any{"action": "remove", "chord": chord, "thread": "Report"}, false)
	c.call("chord", map[string]any{"action": "set", "set": map[string]any{"doc": chord, "priority": "high"}}, false)
	list, _ := c.call("chord", map[string]any{}, false)
	one, _ := c.call("chord", map[string]any{"action": "load", "chord": "Quiet alarms"}, false)
	if len(dig(list, "chords").([]any)) != 1 || dig(one, "chord", "handoff") != "Resume Atlas chord "+chord || dig(one, "chord", "chord", "state", "priority") != "high" || len(dig(one, "chord", "ready").([]any)) != 1 {
		t.Fatalf("chords %v %v", list, one)
	}
	c.call("chord", map[string]any{"action": "drop", "chord": chord, "reason": "Not this quarter."}, false)
	c.call("chord", map[string]any{"action": "reopen", "chord": chord}, false)

	// The pipeline's tools.
	captured, _ := c.call("source", map[string]any{"action": "capture", "inbox": []any{"notes.md"}, "tags": []any{"work/p3/p3-edge"}}, false)
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
	m, _ := c.call("match", map[string]any{"items": []any{map[string]any{"doc": srcID, "chunk": 1, "items": []any{map[string]any{"kind": "concept", "name": "Motion scoring", "claims": []any{map[string]any{"text": "cuts false alarms", "locator": "line 3"}}}}}}}, false)
	if dig(m, "subjects").([]any)[0].(map[string]any)["match"] != "new" {
		t.Fatalf("match %v", m)
	}
	c.call("match", map[string]any{"tags": []any{"work"}, "across": true}, false)
	f, _ := c.call("lint", map[string]any{"tags": []any{"work"}}, false)
	if dig(f, "counts") == nil {
		t.Fatalf("lint %v", f)
	}
	synced, _ := c.call("vault", map[string]any{"action": "sync"}, false)
	if dig(synced, "synced") == nil {
		t.Fatalf("sync %v", synced)
	}
	c.call("vault", map[string]any{"action": "sync", "views": true}, false)

	// An unknown action and a missing document come back as tool errors.
	c.call("change", map[string]any{"action": "merge"}, true)
	c.call("change", map[string]any{"id": "chg-zzzzzz"}, true)
	c.call("thread", map[string]any{"action": "file"}, true)
	c.call("chord", map[string]any{"action": "file"}, true)
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
	c.call("thread", map[string]any{"action": "stub", "text": "add these papers to the wiki", "title": "Papers"}, false)
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
