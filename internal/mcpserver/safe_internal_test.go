package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type empty struct{}

func TestAPanicInOneToolLeavesTheServerAnswering(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "boom"}, safe("boom", func(context.Context, *mcp.CallToolRequest, empty) (*mcp.CallToolResult, map[string]any, error) {
		var m map[string]int
		m["x"]++ // a nil map: the kind of fault a nil document gave before
		return nil, nil, nil
	}))
	mcp.AddTool(server, &mcp.Tool{Name: "ping"}, safe("ping", func(context.Context, *mcp.CallToolRequest, empty) (*mcp.CallToolResult, map[string]any, error) {
		return nil, map[string]any{"ok": true}, nil
	}))
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	sess, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "boom", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("protocol error: %v", err)
	}
	text := ""
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	if !res.IsError || !strings.Contains(text, "boom failed inside almagest") {
		t.Fatalf("the panic came back as %+v", res)
	}
	res, err = sess.CallTool(ctx, &mcp.CallToolParams{Name: "ping", Arguments: map[string]any{}})
	if err != nil || res.IsError {
		t.Fatalf("the next call: %+v %v", res, err)
	}
}
