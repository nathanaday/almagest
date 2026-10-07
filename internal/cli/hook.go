package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/nathanaday/almagest/internal/hooks"
	"github.com/nathanaday/almagest/internal/mcpserver"
)

// hookCmd runs one hook, as the host calls it.
func (c *CLI) hookCmd(argv []string) int {
	if len(argv) == 0 {
		var names []string
		for n := range hooks.Events {
			names = append(names, n)
		}
		sort.Strings(names)
		fmt.Fprintln(c.Err, "almagest hook takes one of: "+strings.Join(names, ", "))
		return 1
	}
	env := hooks.Env{Getenv: c.Getenv, Now: c.Now}
	if err := hooks.Run(argv[0], c.In, c.Out, env); err != nil {
		fmt.Fprintln(c.Err, "almagest hook "+argv[0]+": "+err.Error())
		return 1
	}
	return 0
}

// mcpCmd serves the MCP tools over stdio.
func (c *CLI) mcpCmd() error {
	dir := c.Getenv("CLAUDE_PROJECT_DIR")
	if dir == "" {
		dir = c.Dir
	}
	return mcpserver.New(mcpserver.Options{Version: Version, Dir: dir, Getenv: c.Getenv, Now: c.Now}).Serve(context.Background())
}
