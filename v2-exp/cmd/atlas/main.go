// Command atlas serves the Atlas tools and hooks, and runs every action from a shell.
package main

import (
	"os"

	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/cli"
)

func main() {
	os.Exit(cli.New().Run(os.Args[1:]))
}
