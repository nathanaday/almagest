// Package cli is the almagest command: one subcommand per tool action, the hooks, the MCP
// server, and the commands no tool needs (setup, doctor, version, open).
// Every command reaches the same function its tool does.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/nathanaday/almagest/internal/core"
	"github.com/nathanaday/almagest/internal/vault"
)

// Version is the binary's version; the build stamps it.
var Version = "dev"

// Protocol is the version of what the Obsidian plugin reads: the commands it runs, their
// flags, and their JSON. It goes up when one of them changes in a way an older plugin
// would misread, and the plugin names the update that each side needs.
const Protocol = 1

// CLI is one run of the command.
type CLI struct {
	// moved are the notes the views step of this command moved out of wiki-view/.
	moved  []vault.Moved
	In     io.Reader
	Out    io.Writer
	Err    io.Writer
	Getenv func(string) string
	Now    func() time.Time
	Dir    string
}

// views writes the views after a write, and tells the user, on stderr so JSON output
// stays clean, where each note found in wiki-view/ went.
func (c *CLI) views(v *vault.Vault, now time.Time) {
	moved, _ := core.Views(v, now)
	c.moved = append(c.moved, moved...)
	for _, m := range moved {
		fmt.Fprintln(c.Err, core.StrayLine(m))
	}
}

// New is the command as a process runs it.
func New() *CLI {
	dir, _ := os.Getwd()
	return &CLI{In: os.Stdin, Out: os.Stdout, Err: os.Stderr, Getenv: os.Getenv, Now: time.Now, Dir: dir}
}

const usageHead = `almagest: one vault for what you know, and what your agents did.

Usage:
`

const usageTail = `
A command that acts on a vault takes --vault (a path, or a name from ~/.almagest/config.json),
else $ALMAGEST_VAULT, else the vault above the working folder. setup's --vault is the folder
of a new vault, vault init takes --path, and doctor, version, help, hook, and mcp take no
--vault.
--json prints JSON from: vault, search, context, source, change, checkout, wikify, journal, lint, config.
match always prints JSON.
No --json: setup, open, doctor, version, help, hook, mcp.
`

// commands are the usage of each command, in the order help lists them.
var commands = []struct{ name, usage string }{
	{"vault", `  almagest vault [status] | sync [--views] | snapshot | trash PATH | migrate [--dry-run]
                       | init [--path FOLDER | FOLDER] --name N [--description D] [--tagging open|known]
`},
	{"search", `  almagest search TEXT [--type T]... [--kind K]... [--tag T]... [--status S]... [--repository R] [--limit N]
`},
	{"context", `  almagest context [REPOSITORY] [--tag T]... [--path P]
`},
	{"match", `  almagest match --items FILE.json | --docs ID... [--tag T]... [--across]
`},
	{"source", `  almagest source capture [--ingest NAME]... | [--text FILE --title T [--locator URL]] | [--repository R]
                                [--tag T]... [--new-tags]
  almagest source chunks DOC
  almagest source read DOC CHUNK
`},
	{"change", `  almagest change propose FILE.json [--id ID] | show ID | apply ID | reject ID --reason R | undo ID
                        | start --kind ingest|repair|draft [--title T] [--file NAME]... | progress ID TEXT...
`},
	{"checkout", `  almagest checkout [list] | candidates TEXT... [--tag T]... [--type T]... [--limit N]
                          | make FILE.json | return FOLDER
`},
	{"wikify", `  almagest wikify start PATH | mark PATH FILE.json
`},
	{"journal", `  almagest journal [list] | publish VOLUME
                                                   a journal volume is a folder directly under journals/
`},
	{"lint", `  almagest lint [--tag T]...
`},
	{"hook", `  almagest hook EVENT                        a hook; reads the event JSON on stdin
`},
	{"mcp", `  almagest mcp                               the MCP server, over stdio
`},
	{"config", `  almagest config [show] | set KEY VALUE [--global] | unset KEY [--global]
                                                   agent preferences: the vault's file wins over ~/.almagest/config.json
`},
	{"setup", `  almagest setup [--agent claude|codex] [--no-plugin] [--plugin-source SOURCE]
                       [--vault FOLDER --name N [--description D] [--tagging open|known] [--allow-vault]]
                                                   installs the binary and the agent plugin, and makes a first vault
`},
	{"doctor", `  almagest doctor                            checks the binary, each agent's plugin and server, and every vault
`},
	{"version", `  almagest version [--json]                          the version, and the protocol the Obsidian plugin reads
`},
	{"open", `  almagest open [DOC] [--register]
`},
	{"help", `  almagest help | COMMAND --help
`},
}

// usage is the whole usage, which help prints.
func usage() string {
	var b strings.Builder
	b.WriteString(usageHead)
	for _, c := range commands {
		b.WriteString(c.usage)
	}
	b.WriteString(usageTail)
	return b.String()
}

// commandUsage is one command's usage, or "" for a name help does not list.
func commandUsage(name string) string {
	for _, c := range commands {
		if c.name == name {
			return "Usage:\n" + c.usage + usageTail
		}
	}
	return ""
}

// wantsHelp reports whether an argument before -- asks for help.
func wantsHelp(argv []string) bool {
	for _, a := range argv {
		if a == "--" {
			return false
		}
		if a == "--help" || a == "-h" {
			return true
		}
	}
	return false
}

// Run runs the command and returns its exit code.
func (c *CLI) Run(argv []string) int {
	if len(argv) == 0 {
		fmt.Fprint(c.Out, usage())
		return 0
	}
	cmd, rest := argv[0], argv[1:]
	// Help runs nothing: a command reads --help as an option, and setup, vault init,
	// hook, and mcp act at once.
	if wantsHelp(rest) {
		if u := commandUsage(cmd); u != "" {
			fmt.Fprint(c.Out, u)
			return 0
		}
	}
	var err error
	switch cmd {
	case "vault":
		err = c.vaultCmd(rest)
	case "search":
		err = c.searchCmd(rest)
	case "context":
		err = c.contextCmd(rest)
	case "match":
		err = c.matchCmd(rest)
	case "source":
		err = c.sourceCmd(rest)
	case "change":
		err = c.changeCmd(rest)
	case "lint":
		err = c.lintCmd(rest)
	case "journal":
		err = c.journalCmd(rest)
	case "checkout":
		err = c.checkoutCmd(rest)
	case "wikify":
		err = c.wikifyCmd(rest)
	case "hook":
		return c.hookCmd(rest)
	case "mcp":
		err = c.mcpCmd()
	case "config":
		err = c.configCmd(rest)
	case "setup":
		err = c.setupCmd(rest)
	case "doctor":
		return c.doctorCmd(rest)
	case "version", "--version", "-v":
		err = c.emit(parse(rest), map[string]any{"version": Version, "protocol": Protocol}, func(w io.Writer) {
			fmt.Fprintln(w, "almagest "+Version)
		})
	case "open":
		err = c.openCmd(rest)
	case "help", "--help", "-h":
		if u := commandUsage(argOr(rest, 0)); u != "" {
			fmt.Fprint(c.Out, u)
			break
		}
		fmt.Fprint(c.Out, usage())
	default:
		err = fmt.Errorf("no command %q; almagest help lists them", cmd)
	}
	if err != nil {
		fmt.Fprintln(c.Err, "almagest: "+err.Error())
		var coded exitError
		if errors.As(err, &coded) {
			return coded.code
		}
		return 1
	}
	return 0
}

func argOr(argv []string, i int) string {
	if i < len(argv) {
		return argv[i]
	}
	return ""
}

// args are a command's positional arguments and its --flags.
type args struct {
	pos   []string
	flags map[string][]string
}

// parse reads --name value, --name=value, and the boolean flags named in bools.
func parse(argv []string, bools ...string) args {
	a := args{flags: map[string][]string{}}
	isBool := map[string]bool{"json": true}
	for _, b := range bools {
		isBool[b] = true
	}
	for i := 0; i < len(argv); i++ {
		s := argv[i]
		if s == "--" {
			a.pos = append(a.pos, argv[i+1:]...)
			break
		}
		name, ok := strings.CutPrefix(s, "--")
		if !ok || name == "" {
			a.pos = append(a.pos, s)
			continue
		}
		if k, v, eq := strings.Cut(name, "="); eq {
			a.flags[k] = append(a.flags[k], v)
			continue
		}
		if isBool[name] || i+1 >= len(argv) {
			a.flags[name] = append(a.flags[name], "true")
			continue
		}
		a.flags[name] = append(a.flags[name], argv[i+1])
		i++
	}
	return a
}

func (a args) get(name string) string {
	if v := a.flags[name]; len(v) > 0 {
		return v[len(v)-1]
	}
	return ""
}

func (a args) has(name string) bool { return len(a.flags[name]) > 0 }

func (a args) list(name string) []string { return a.flags[name] }

func (a args) arg(i int) string {
	if i < len(a.pos) {
		return a.pos[i]
	}
	return ""
}

func (c *CLI) home() vault.Home { return vault.HomeFrom(c.Getenv) }

func (c *CLI) open(a args) (*vault.Vault, error) {
	return vault.Select(a.get("vault"), c.Dir, c.home(), c.Getenv(vault.EnvVault))
}

func (c *CLI) index(a args) (*vault.Index, error) {
	v, err := c.open(a)
	if err != nil {
		return nil, err
	}
	return vault.Load(v)
}

// emit prints an entity as JSON with --json, else through the human printer.
func (c *CLI) emit(a args, v any, human func(w io.Writer)) error {
	if a.has("json") || human == nil {
		if len(c.moved) > 0 {
			v = withMoved(v, c.moved)
		}
		enc := json.NewEncoder(c.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	human(c.Out)
	return nil
}

// withMoved adds moved_from_wiki_view to a JSON result, whatever its type, so a caller that
// reads only stdout (the Obsidian plugin) learns where each note went.
func withMoved(v any, moved []vault.Moved) any {
	data, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out map[string]any
	if json.Unmarshal(data, &out) != nil || out == nil {
		return v
	}
	out["moved_from_wiki_view"] = moved
	return out
}

func (c *CLI) readJSON(file string, into any) error {
	var data []byte
	var err error
	if file == "-" || file == "" {
		data, err = io.ReadAll(c.In)
	} else {
		data, err = os.ReadFile(file)
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, into)
}

func (c *CLI) readText(file string) (string, error) {
	if file == "" || file == "-" {
		data, err := io.ReadAll(c.In)
		return string(data), err
	}
	data, err := os.ReadFile(file)
	return string(data), err
}

// exitError is an error with its own exit code.
type exitError struct {
	code int
	err  error
}

func (e exitError) Error() string { return e.err.Error() }

func short(sha string) string { return sha[:min(10, len(sha))] }

// count is a number and its noun, singular for one.
func count(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}
