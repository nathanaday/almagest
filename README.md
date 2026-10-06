# Atlas

Status: in development. The 5.x design (a project folder in every repository, and a
terminal view) is retired; its code is on the `v1` branch.

## About

Working with agents on long projects fails in two ways. You delegate and lose track of
the work, or you spend your energy keeping documents current. Atlas keeps every document
of your work in one Obsidian vault, and the agents write most of it for you:

- what you know: **topics**, **sources**, and **repositories**, with a citation for every
  claim;
- what your agents do now and did before: one **session** document per agent session;
- every edit an agent made to the knowledge: a **change** document you approve.

Every document of the wiki lies flat in `wiki/documents/`. Tags sort it, and a document
may hold many: a page tagged `cs513` and `self-driving` shows up under both, and a search
for both tags finds it. You start an agent in the vault. It finds the repository you
mean through its tags and its repository document, and it edits the code there. Its
session document records which repositories it touched. You read and edit every
document in Obsidian.

Atlas 8 also tracked work as threads and chords. Atlas 9.0 removed them. They now live in
the standalone project obsidian-threads (`~/projects/software/obsidian-threads`).

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

### Add the plugin to an agent

`setup` adds the agent plugin to Claude Code when Claude Code does not have it. It runs
these two commands, which you can also run yourself:

```bash
claude plugin marketplace add nathanaday/atlas-obsidian
claude plugin install atlas-obsidian@nathanaday-atlas-obsidian
```

Restart Claude Code to load the plugin. `atlas-obsidian doctor` checks the binary, the
plugin, and every vault. It also starts the plugin's MCP server as each agent runs it,
and fails when the server does not list the binary's tools.

**A second Claude Code account.** Claude Code keeps each account's plugins in its config
folder: `~/.claude`, or the folder that `CLAUDE_CONFIG_DIR` names. To add the plugin to
an account with its own config folder, run `setup` with that variable set:

```bash
CLAUDE_CONFIG_DIR="$HOME/.claude-other" ~/.atlas/bin/atlas-obsidian setup
```

Every account on the machine shares one binary (`~/.atlas/bin/atlas-obsidian`) and one
list of vaults (`~/.atlas/config.json`), so each account finds the same vaults.

**Update.** Each account holds its own copy of the plugin. After a new release, update
the plugin in each account, then the binary once:

```bash
claude plugin marketplace update nathanaday-atlas-obsidian
claude plugin update atlas-obsidian@nathanaday-atlas-obsidian
make install
```

For an account with its own config folder, set `CLAUDE_CONFIG_DIR` on the two `claude`
commands.

**Codex.** `setup --agent codex` adds the plugin to Codex when Codex does not have it:

```bash
codex plugin marketplace add nathanaday/atlas-obsidian
codex plugin add atlas-obsidian@nathanaday-atlas-obsidian
```

Codex runs a plugin's hooks only after you trust them. Without the hooks, the guard and
the session record are off. Open `/hooks` in Codex, trust the `atlas-obsidian` hooks,
and start a new session. Each install or update that changes the hooks needs your trust
again. `setup --agent codex` and `doctor` say how many hooks Codex runs.

To update the plugin in Codex, fetch the marketplace again and add the plugin again:

```bash
codex plugin marketplace upgrade nathanaday-atlas-obsidian
codex plugin remove atlas-obsidian@nathanaday-atlas-obsidian
codex plugin add atlas-obsidian@nathanaday-atlas-obsidian
```

Some parts of Atlas do not work on Codex yet:

- **No read-only agents.** Codex does not load a plugin's agents, so wiki-sync,
  wiki-review, and repo-ingest cannot send their workers, and the guard's read-only rule
  never applies.
- **No `waiting` status.** Codex has no Notification event, so a Codex session never
  shows `waiting`.
- **Linked repositories.** Atlas grants Claude Code write access to linked repositories
  through the vault's `.claude/settings.local.json`. Codex reads
  `sandbox_workspace_write.writable_roots` instead, and Atlas does not write it, so a
  Codex session may refuse a write in a linked repository, or ask before it.
- **`--allow-vault`** has no effect with `--agent codex`.

The Atlas design vault tracks these limits.

A vault of 8.x needs one migration to the 9.0 layout. Obsidian shows a notice. From a
shell, `atlas-obsidian vault migrate --dry-run` lists the moves, and
`atlas-obsidian vault migrate` makes them in one commit:

- every stub, spec, task list, verification, chord, and event document moves from
  `wiki/documents/` to `threads/`, with its file name and content unchanged;
- every canvas in `chords/` moves to `threads/`, and `chords/` goes;
- topics and repositories lose their `## Threads` section; topics and sources lose
  `from`; sessions lose `threads`, `specs`, `work`, `checked`, and `events`; changes lose `work`;
- `Atlas.md` loses `wikify`.

`threads/` is an archive that Atlas does not read. A link to a document there still
resolves. Search and lint skip the folder, and the guard does not refuse an edit there.
The migration refuses while a change is proposed: apply or reject it first. A vault
older than 8.0 migrates with Atlas 8.1.1 (tag `threads-final`) first, then with 9.0.

To try a checkout without installing the plugin, start Claude Code with
`claude --plugin-dir /path/to/atlas-obsidian`.

### Make a vault

Start Claude Code in an empty folder and say "set up atlas". Or from a shell:

```bash
atlas-obsidian vault init --path ~/notes/work --name Work --tagging open \
  --description "Work notes: the p3 product and the tools around it."
atlas-obsidian open --register --vault ~/notes/work   # opens it in Obsidian; turn on the Atlas plugin once
```

A command that acts on a vault takes `--vault` (a folder, or a vault's name), else the
vault that `ATLAS_VAULT` names, else the vault above the working folder.

### Agent preferences

Start agent reads four preferences, and Resume reads `terminal` and `terminal_command`:

| Key                     | Values                                                  | Default    |
| ----------------------- | ------------------------------------------------------- | ---------- |
| `agent`                 | `claude`, `codex`                                       | `claude`   |
| `agent_commands.<agent>`| the command as you type it in a shell, such as a shell function | the agent's name |
| `terminal`              | `terminal`, `iterm`, `wezterm`, `ghostty`, `custom`     | `terminal` |
| `terminal_command`      | for `custom`: a command with `{command}` for the agent's command | none |

Two files hold them. `~/.atlas/config.json` holds them for every vault. `.atlas/config.json`
in a vault overrides them, key by key: when both files set a key, the vault's value wins.
Set them in the Atlas settings in Obsidian, or from a shell:

```bash
atlas-obsidian config set terminal wezterm --global        # every vault
atlas-obsidian config set agent_commands.claude claude-work  # this vault: another account
atlas-obsidian config                                      # the result, and where each value comes from
atlas-obsidian config unset agent_commands.claude          # back to the global value
```

The terminal runs the command in your login shell, so your `PATH` and shell functions
apply. [TESTED.md](TESTED.md) lists the agent and terminal pairs we tested.

### Environment variables

| Variable | What it does |
| --- | --- |
| `ATLAS_HOME` | The machine folder: the binary, `config.json` with the list of vaults, and the global preferences. Default `~/.atlas`. |
| `ATLAS_VAULT` | The vault a command, the MCP server, or a hook uses when the call names none: a folder or a vault's name. |
| `ATLAS_BIN` | The binary that the wrapper script and the Codex server entry run, before `$ATLAS_HOME/bin` and `~/go/bin`. |
| `ATLAS_HOOK_LOG` | A file that gets every hook event in full, your prompts included. Use it only to debug. |
| `CLAUDE_CONFIG_DIR` | Claude Code's config folder, which `setup` and `doctor` read. Default `~/.claude`. |
| `CLAUDE_PROJECT_DIR` | The folder in which `atlas-obsidian mcp` looks for the vault, when set. |
| `OBSIDIAN_CONFIG_DIR` | The folder of Obsidian's `obsidian.json`, which `open --register` edits. |
| `CODEX_HOME` | Codex's own folder. Atlas does not read it, but the `codex` commands that `setup` and `doctor` run do. |

### Files in a vault

Besides `wiki/documents/`, Atlas writes these files in a vault:

- `Atlas.md`, the vault's own document, and the folders `inbox/`, `scratchpad/`,
  `sessions/`, `changes/`, `wiki/assets/`, and `views/`. `wiki/assets/` holds the
  captured originals and your attachments. `inbox/` holds files for the wiki to learn
  from; `scratchpad/` holds your notes and the ideas you ask an agent to note for later.
- `threads/`, in a vault that the 9.0 migration moved: the thread and chord documents of
  8.x. Atlas does not read it.
- `.obsidian/app.json`: `vault init` sends new attachments to `wiki/assets/` (unless you
  chose a folder) and keeps `views/` out of Obsidian's graph and search. It keeps every
  other key.
- `.obsidian/plugins/atlas/`: the Obsidian plugin.
- `.claude/settings.local.json`: the linked repositories, for Claude Code.
- `.git/info/exclude`: `views/`, `.claude/settings.local.json`, Obsidian's workspace and
  graph files, `.DS_Store`, and Atlas's temporary `.atlas-*` files stay out of the vault's
  history.
- `.atlas/config.json`: the vault's agent preferences. Git commits it, so a shared vault
  shares it; see [SECURITY.md](SECURITY.md).

## Usage

Start the agent in the vault and ask in plain words. The `atlas` skill routes each
request.

- "Link the repository at ~/code/p3-edge under the tag work/p3." The agent proposes a
  change; you say yes, or press Apply in Obsidian. The agent cannot apply a change that
  writes until you send a prompt in the session that proposed it. A change with no writes
  that only marks sources as absorbed applies at once. This gate holds against the atlas
  tools; it is not a sandbox against an agent's shell. [SECURITY.md](SECURITY.md) gives
  the full rule.
- "Describe p3-edge in the wiki." The agent snapshots the code and proposes the pages.
- "In p3-edge, score boxes by motion." The agent finds the repository, reads its
  instruction files and the policies of the wiki that apply to it, and edits the code.
  On long work it writes where the work stands under `## Progress` in its session
  document.
- "Note this: try a smaller backbone on p3-edge." The agent writes the idea in a note in
  `scratchpad/`.
- "Start agent" in Obsidian opens a terminal with your agent in the vault. The agent
  preferences choose the agent and the terminal (see
  [Agent preferences](#agent-preferences)). The sessions pane in the right sidebar shows
  the open sessions and the ones that closed in the last two hours, with Resume.
- "Ingest the inbox." Files you dropped in `inbox/` become cited wiki pages.
- "What waits for me?" The agent lists the proposed changes, the mentions, the inbox,
  and the sources that wait for the wiki.
- "What do we know about my cs513 self-driving project?" The agent searches the
  documents that hold both tags.

The same actions work from a shell:

```bash
atlas-obsidian vault                      # the state of the vault
atlas-obsidian search "remote update" --tag work/p3
atlas-obsidian change show chg-r8m3tb     # a proposed change
atlas-obsidian lint                       # the health check
```

In Obsidian, `views/` holds the notes that code writes: Home, Timeline, Library,
Repositories, one timeline per month under `views/timeline/`, and one view per tag under
`views/tags/`. The Atlas navigator narrows the documents one tag at a time.
`sessions/Sessions.base` shows what runs now, and `changes/Changes.base` lists the
changes that wait for you.

## Patterns and conventions

The design lives in the maintainer's Atlas vault, outside this repository. These are
its core rules:

- Everything is a document with an id and a type. No database and no state folder.
- Code owns what code can derive: ids, statuses, links, hashes, git facts, the first
  callout of each document, the views. The model writes prose.
- Knowledge changes only through a change document, and an agent's apply of a change that
  writes waits for your turn.
- An agent edits a linked repository directly. The session record lists the repositories
  each session touched.
- Hooks keep a document for every session, so the record does not depend on the model.

## Layout

- The binary: `cmd/atlas-obsidian/`, `internal/` (one package per part;
  `internal/mcpserver` serves the seven tools, `internal/hooks` the nine hooks,
  `internal/cli` every command).
- The agent plugin: `skills/`, `agents/`, `hooks/hooks.json`, `.mcp.json`,
  `.claude-plugin/`, `.codex-plugin/`, and `scripts/atlas-obsidian`, the wrapper that finds
  the binary for the hooks and the MCP server. `.agents/plugins/marketplace.json` is a second
  marketplace entry, added with Codex support.
- The Obsidian plugin: `obsidian/` (TypeScript); `make obsidian` builds it into the
  binary.
- `v7-design/`: the design pages of 7.0, kept for reference.
- Notes for agents that work on this code: [CLAUDE.md](CLAUDE.md).
