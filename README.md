# Atlas

Status: in development. The 5.x design (a project folder in every repository, and a
terminal view) is retired; its code is on the `v1` branch.

## About

Working with agents on long projects fails in two ways. You delegate and lose track of
the work, or you spend your energy keeping documents current. Atlas keeps every document
of your work in one Obsidian vault, and the agents write most of it for you:

- what you know: **topics**, **sources**, and **repositories**, with a citation for every
  claim;
- what you do: **threads**, each one piece of work from idea to closed: a **stub** (its
  front page), a **spec** (what must be true), **task lists** (the steps, as check
  boxes), and **verifications** (the work checked against the spec); and **chords**,
  which put the threads of one goal in order;
- what your agents do now and did before: one **session** document per agent session;
- every edit an agent made to the knowledge: a **change** document you approve.

Every document lies flat in `wiki/documents/`. Tags sort it, and a document may hold
many: a page tagged `cs513` and `self-driving` shows up under both, and a search for both
tags finds it. You start an agent in the vault. It finds the repository you mean through
its tags and its repository document, and it works there in plain view. You read and
edit every step in Obsidian.

## Quickstart

Atlas is three parts that share one version: the `atlas-obsidian` binary (Go, one
static file), the agent plugin for Claude Code or Codex, and a thin Obsidian plugin that
`atlas-obsidian vault init` puts in the vault. The binary is not named `atlas`, because
other programs install a binary of that name.

### Prerequisites

- macOS or Linux, `git`, Go 1.24 to build.
- Claude Code (or Codex), and Obsidian.

### Build and install

```bash
make install                                      # builds ~/.atlas/bin/atlas-obsidian
~/.atlas/bin/atlas-obsidian setup                 # adds the agent plugin to Claude Code
~/.atlas/bin/atlas-obsidian setup --agent codex   # or to Codex
```

The plugin finds the binary without `PATH`. To run `atlas-obsidian` from a shell, add
its folder to `PATH` in your shell profile:

```bash
export PATH="$HOME/.atlas/bin:$PATH"
```

An install of 6.0 to 6.2 named the binary `atlas`. After you update, delete
`~/.atlas/bin/atlas`.

A vault of 6.x or 7.x needs one migration to the 8.0 layout. Obsidian shows a notice.
From a shell, `atlas-obsidian vault migrate --dry-run` lists every move, and
`atlas-obsidian vault migrate` makes them in one commit. Each plan of 7.x becomes a
thread and keeps its id, its title, and its file.

To try a checkout without installing the plugin, start Claude Code with
`claude --plugin-dir /path/to/atlas-obsidian`.

### Make a vault

Start Claude Code in an empty folder and say "set up atlas". Or from a shell:

```bash
atlas-obsidian vault init --path ~/notes/work --name Work --tagging open \
  --description "Work notes: the p3 product and the tools around it."
atlas-obsidian open --register   # opens it in Obsidian; turn on the Atlas plugin once
```

## Usage

Start the agent in the vault and ask in plain words. The `atlas` skill routes each
request.

- "Link the repository at ~/code/p3-edge under the tag work/p3." The agent proposes a
  change; you say yes, or press Apply in Obsidian. The agent cannot apply a change in the
  turn that proposed it. This gate holds against the atlas tools; it is not a sandbox
  against an agent's shell.
- "Describe p3-edge in the wiki." The agent snapshots the code and proposes the pages.
- "In p3-edge, score boxes by motion." The agent plants a thread, writes its spec, and
  stops for your yes. Then it writes the tasks, does them, and checks each one with its
  commits. A read-only agent verifies the work against each requirement. The thread
  closes when you apply the change that writes what it built into the wiki.
- "Resume Atlas thread doc-aswqa3." Each stub holds this line to copy. The agent loads
  the spec, the open tasks, and the open findings in one call, and goes on.
- "Make a chord: train a vehicle detection model with YOLO." The agent splits the goal
  into threads and orders them. Obsidian shows the order as a canvas you can redraw.
- "Ingest the inbox." Files you dropped in `inbox/` become cited wiki pages.
- "What should I work on?" The agent reads the board and ranks the open threads.
- "What do we know about my cs513 self-driving project?" The agent searches the
  documents that hold both tags.

The same actions work from a shell:

```bash
atlas-obsidian vault                      # the state of the vault
atlas-obsidian thread                     # the board: threads and chords
atlas-obsidian thread load doc-aswqa3     # one thread: what is missing, and the next step
atlas-obsidian search "remote update" --tag work/p3
atlas-obsidian change show chg-r8m3tb     # a proposed change
atlas-obsidian lint                       # the health check
```

In Obsidian, `views/` holds the notes that code writes: Home, Threads, Timeline, Library,
Repositories, and one view per tag under `views/tags/`. `View · Threads` shows each
chord with its threads in order. `chords/` holds one canvas per chord. The Atlas navigator narrows the documents one
tag at a time. `sessions/Sessions.base` shows what runs now, and `changes/Changes.base`
lists the changes that wait for you.

## Patterns and conventions

The design lives in the maintainer's Atlas vault, outside this repository. These are
its core rules:

- Everything is a document with an id and a type. No database and no state folder.
- Code owns what code can derive: ids, statuses, links, hashes, git facts, the first
  callout of each document, the views. The model writes prose.
- Knowledge changes only through a change document, and apply waits for your turn.
  The `thread` and `chord` tools write threads, chords, and events.
- A thread's status is derived, never set: from its check boxes, its verification, and
  the applied change that absorbs it. Each thread document holds only its own sections.
- An edit inside a linked repository needs a started thread with an open task for it.
- Hooks keep a document for every session, so the record does not depend on the model.

## Layout

- The binary: `cmd/atlas-obsidian/`, `internal/` (one package per part;
  `internal/mcpserver` serves the nine tools, `internal/hooks` the nine hooks,
  `internal/cli` every command).
- The agent plugin: `skills/`, `agents/`, `hooks/hooks.json`, `.mcp.json`,
  `.claude-plugin/`, `.codex-plugin/`.
- The Obsidian plugin: `obsidian/` (TypeScript); `make obsidian` builds it into the
  binary.
- Notes for agents that work on this code: [CLAUDE.md](CLAUDE.md).
