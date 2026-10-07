// Command almagest serves the Almagest tools and hooks, and runs every action from a shell.
package main

import (
	"os"

	"github.com/nathanaday/almagest/internal/cli"
)

func main() {
	os.Exit(cli.New().Run(os.Args[1:]))
}
