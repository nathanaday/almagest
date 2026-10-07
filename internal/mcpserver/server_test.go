package mcpserver_test

import (
	"context"
	"encoding/json"
	"slices"
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
	tv.Write("ingest/notes.md", "# Notes\n\nMotion scoring cuts false alarms.\n")
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
	if strings.Join(want, ",") != "vault,search,context,match,source,change,checkout,lint" {
		t.Fatalf("ToolNames %v", want)
	}
	want = slices.Clone(want)
	sort.Strings(want)
	sort.Strings(names)
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("tools %v", names)
	}
	if list, _ := c.call("checkout", map[string]any{}, false); dig(list, "checkouts") != nil {
		t.Fatalf("checkout list in an empty vault: %v", list)
	}
	cands, _ := c.call("checkout", map[string]any{"action": "candidates", "text": "anything"}, false)
	_ = cands
	c.call("checkout", map[string]any{"action": "make"}, true)
	c.call("checkout", map[string]any{"action": "lend"}, true)
	for _, gone := range []string{"thread", "chord"} {
		if _, err := c.sess.CallTool(context.Background(), &mcp.CallToolParams{Name: gone, Arguments: map[string]any{}}); err == nil || !strings.Contains(err.Error(), "unknown tool") {
			t.Fatalf("%s: %v", gone, err)
		}
	}

	st, _ := c.call("vault", map[string]any{}, false)
	if dig(st, "status", "vault", "name") != "Work" || len(dig(st, "status", "ingest").([]any)) != 1 {
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
	work, _ := c.call("change", map[string]any{"action": "start", "kind": "repair", "title": "Repair links"}, false)
	workID := dig(work, "ref", "id").(string)
	if dig(work, "status") != "running" {
		t.Fatalf("start %v", work)
	}
	if p, _ := c.call("change", map[string]any{"action": "progress", "id": workID, "text": "read the lint findings"}, false); dig(p, "status") != "running" {
		t.Fatalf("progress %v", p)
	}
	c.call("change", map[string]any{"action": "reject", "id": workID, "reason": "not now"}, false)
	if _, msg := c.call("change", map[string]any{"action": "progress", "id": workID, "text": "more"}, true); !strings.Contains(msg, "stop the work") {
		t.Fatalf("progress after cancel: %s", msg)
	}
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
	if !tv.V.Exists("wiki-view/nav/work/p3/Tag · work › p3.md") {
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

	// The pipeline's tools.
	captured, _ := c.call("source", map[string]any{"action": "capture", "ingest": []any{"notes.md"}, "tags": []any{"work/p3/p3-edge"}}, false)
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
	c.call("vault", map[string]any{"action": "file"}, true)
	c.call("source", map[string]any{"action": "file"}, true)
}

func TestStatusAndInitFromTheServer(t *testing.T) {
	tv := testvault.New(t)
	tv.Write("ingest/paper.pdf", "%PDF")
	c := connect(t, tv, tv.V.Root)
	st, _ := c.call("vault", map[string]any{}, false)
	if ingest := dig(st, "status", "ingest").([]any); len(ingest) != 1 {
		t.Fatalf("ingest %v", ingest)
	}
	if dig(st, "status", "mentions") != nil {
		t.Fatalf("the status names mentions: %v", st)
	}
	c.call("vault", map[string]any{"action": "mention"}, true)

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

func TestTheServerTakesTheVaultFromAtlasVault(t *testing.T) {
	one, two := testvault.New(t), testvault.New(t)
	two.Doc("topic", "Only in the second vault", map[string]any{"kind": "overview"}, "")
	two.Commit()
	connectWith := func(envVault string) *client {
		s := mcpserver.New(mcpserver.Options{Version: "test", Dir: one.V.Root, Getenv: func(k string) string {
			switch k {
			case vault.EnvHome:
				return one.Home.Root
			case vault.EnvVault:
				return envVault
			}
			return ""
		}, Now: func() time.Time { return one.Tick(time.Second) }})
		st, ct := mcp.NewInMemoryTransports()
		if _, err := s.MCP().Connect(context.Background(), st, nil); err != nil {
			t.Fatal(err)
		}
		sess, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(context.Background(), ct, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { sess.Close() })
		return &client{t: t, sess: sess}
	}
	out, _ := connectWith(two.V.Root).call("search", map[string]any{"text": "second vault"}, false)
	if data, _ := json.Marshal(out); !strings.Contains(string(data), "Only in the second vault") {
		t.Fatalf("ATLAS_VAULT did not choose the second vault: %s", data)
	}
	if _, msg := connectWith("/no/such/vault").call("search", map[string]any{"text": "second vault"}, true); !strings.Contains(msg, "ATLAS_VAULT=/no/such/vault") {
		t.Fatalf("a bad ATLAS_VAULT: %s", msg)
	}
}
