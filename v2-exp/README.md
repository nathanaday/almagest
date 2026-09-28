# Atlas V2 (experimental)

Status: in development. V1 (`atlas-obsidian`, at the repository's root) stays as it is.

## About

Working with agents on long projects fails in two ways. You delegate and lose track of
the work, or you spend your energy keeping documents current. Atlas keeps every document
of your work in one Obsidian vault, and the agents write most of it for you:

- what you know: the **wiki**, with a citation for every claim;
- what you do: the **threads**, one document per stage of each line of work;
- what your agents do now and did before: one **session** document per agent session;
- every edit an agent made to the wiki: a **change** document you approve.

You start an agent in the vault. It finds the repository you mean through the vault's
areas and repository pages, and it works there in plain view. You read and edit every
step in Obsidian.

## Quickstart

Atlas is three parts that share one version: the `atlas` binary (Go, one static file),
the agent plugin for Claude Code or Codex, and a thin Obsidian plugin that `atlas vault
init` puts in the vault.

### Prerequisites

- macOS or Linux, `git`, Go 1.24 to build.
- Claude Code (or Codex), and Obsidian.

### Build and install

```bash
cd v2-exp
make install        # builds ~/.atlas/bin/atlas
~/.atlas/bin/atlas setup                         # adds the agent plugin to Claude Code
~/.atlas/bin/atlas setup --agent codex           # or to Codex
```

To try a checkout without installing the plugin, start Claude Code with
`claude --plugin-dir /path/to/v2-exp`.

### Make a vault

Start Claude Code in an empty folder and say "set up atlas". Or from a shell:

```bash
atlas vault init --path ~/notes/work --name Work --areas few \
  --description "Work notes: the p3 product and the tools around it."
atlas open --register     # opens the vault in Obsidian; turn on the Atlas plugin once
```

## Usage

Start the agent in the vault and ask in plain words. The `atlas` skill routes each
request.

- "Link the repository at ~/code/p3-edge under a new area p3." The agent proposes a
  change; you say yes, or press Apply in Obsidian.
- "Describe p3-edge in the wiki." The agent snapshots the code and proposes the pages.
- "In p3-edge, score boxes by motion." The agent finds or opens a thread, writes a spec
  and tasks, stops for your yes at each, does the work, and files a receipt.
- "Ingest the inbox." Files you dropped in `inbox/` become cited wiki pages.
- "What should I work on?" The agent ranks the open threads.

The same actions work from a shell:

```bash
atlas vault                      # the state of the vault
atlas thread                     # the board
atlas search "remote update" --scope p3
atlas change show chg-r8m3tb     # a proposed change
atlas lint                       # the health check
```

In Obsidian, `threads/Threads.base` is the board, `sessions/Sessions.base` shows what
runs now, and `changes/Changes.base` lists the changes that wait for you.

## Patterns and conventions

The design is in [`atlas/atlas-obsidian/design/`](../atlas/atlas-obsidian/design/Atlas%20V2.md);
start with `Atlas V2.md`.

- Everything is a document with an id and a type. No database and no state folder.
- Code owns what code can derive: ids, stages, statuses, links, hashes, the first callout
  of each document. The model writes prose.
- The wiki changes only through a change document, and apply waits for your turn.
- An edit inside a linked repository needs an open thread that covers it.
- Hooks keep a document for every session, so the record does not depend on the model.

## Layout

- The binary: `cmd/atlas/`, `internal/` (one package per part; `internal/mcpserver` serves
  the eight tools, `internal/hooks` the nine hooks, `internal/cli` every command).
- The agent plugin: `skills/`, `agents/`, `hooks/hooks.json`, `.mcp.json`,
  `.claude-plugin/`, `.codex-plugin/`.
- The Obsidian plugin: `obsidian/` (TypeScript); `make obsidian` builds it into the
  binary.
- Notes for agents that work on this code: [CLAUDE.md](CLAUDE.md).
