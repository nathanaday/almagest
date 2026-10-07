package cli_test

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/almagest/internal/cli"
	"github.com/nathanaday/almagest/internal/vault"
)

// --help prints a command's usage and runs nothing, even for the commands that act at
// once: setup copies the binary, vault init makes a vault, hook reads stdin.
func TestHelpRunsNothing(t *testing.T) {
	calls := [][]string{
		{"vault", "init"}, {"vault", "sync"}, {"vault", "migrate"}, {"vault", "snapshot"}, {"search", "x"},
		{"context"}, {"match"}, {"source", "capture"}, {"change", "apply", "x"}, {"lint"}, {"hook", "session-start"},
		{"mcp"}, {"config", "set", "agent", "codex"}, {"setup"}, {"doctor"}, {"version"}, {"open"}, {"help"},
	}
	for _, call := range calls {
		for _, flag := range []string{"--help", "-h"} {
			dir, home := t.TempDir(), t.TempDir()
			var out, errOut bytes.Buffer
			c := &cli.CLI{In: strings.NewReader("{}"), Out: &out, Err: &errOut, Dir: dir,
				Getenv: func(k string) string {
					if k == vault.EnvHome {
						return home
					}
					return ""
				}, Now: time.Now}
			args := append(append([]string{}, call...), flag)
			if code := c.Run(args); code != 0 {
				t.Fatalf("%v: exit %d: %s", args, code, errOut.String())
			}
			if !strings.Contains(out.String(), "almagest "+call[0]) {
				t.Fatalf("%v printed no usage of %s:\n%s", args, call[0], out.String())
			}
			for _, d := range []string{dir, home} {
				if entries, _ := os.ReadDir(d); len(entries) != 0 {
					t.Fatalf("%v wrote %v", args, entries)
				}
			}
		}
	}
	var out bytes.Buffer
	c := &cli.CLI{Out: &out, Err: &out, Getenv: func(string) string { return "" }, Now: time.Now}
	if code := c.Run([]string{"nosuch", "--help"}); code == 0 || !strings.Contains(out.String(), `no command "nosuch"`) {
		t.Fatalf("an unknown command with --help: %d %s", code, out.String())
	}
}
