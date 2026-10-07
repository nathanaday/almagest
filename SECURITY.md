# Security

This page lists what Atlas trusts, what it guards against and how, and what it does not
guard against.

## Reporting a vulnerability

Report it privately through **Report a vulnerability** on the repository's Security tab.
Do not open a public issue for it.

## What Atlas is

Atlas runs on your machine. It has three parts:

- the `atlas-obsidian` binary, which serves the agent's tools over stdio and runs the
  hooks;
- the agent plugin, which holds the skills, the agents, and the hook configuration;
- the Obsidian plugin.

The binary does not listen on a port. The Obsidian plugin makes no network requests.

## Trust model

Atlas trusts:

- you, in your terminal and in Obsidian;
- the host (Claude Code or Codex), which runs the hooks and reports your prompts.

Atlas does not trust:

- **The agent.** An agent can make a mistake, or follow instructions it read in a captured
  source, a wiki page, or a diff.
- **The contents of the vault.** Pages, frontmatter, captured sources, and change documents
  can come from a `git pull` of a shared vault, or from a shell command.
- **The contents of a linked repository.**

## What Atlas guards against, and how

### An agent changes the wiki without your yes

The agent writes to the wiki only through a change document. The agent proposes the
change, you read the preview and answer, and then the agent applies the change. Or you
press Approve in the change document and apply it yourself.

- The `change` tool refuses the agent's `apply` until you have sent a prompt after the
  proposal, in the session that proposed it. The tool checks this on the change document it
  is about to apply, while it holds the vault's lock.
- A subagent's report and a background task's notice arrive as prompts, but they do not
  count as your turn.
- One kind of change needs no prompt: a change with no writes. It only marks sources as
  absorbed, so the agent applies it in the same turn, with or without a recorded session.
- The tool refuses a change that names no session, a change whose proposal time does not
  parse, and a change whose session ended, or is lost, with no prompt from you after the
  proposal. No later prompt reaches that session. Press Approve in the change document
  to apply such a change, or ask an agent to propose it again from a live session.
- The gate does not read your prompt. The skill reads your answer and decides whether to
  apply.
- Two ways to apply have no gate, because you take each one yourself:
  `atlas-obsidian change apply` in your terminal, and Approve in the change document in
  Obsidian. Approve runs that same command as you. Cancel runs `atlas-obsidian change
  reject`. A change that `atlas-obsidian change propose` writes from a terminal names no
  session, so only these two apply it.
- A work document (a change with status `running`) holds no writes, and apply refuses
  any change that is not proposed. The agent proposes into it later, and the gate above holds for that
  proposal. Cancel on a running document rejects it, and the `change` tool then refuses
  the agent's progress and proposal.
- `atlas-obsidian vault trash` is your safe delete; the Obsidian plugin runs it as you.
  It moves nothing while a file links the one you delete. A topic, a source, or a
  repository leaves through a change that the command applies at once as your own
  action, so the change records it and `change undo` brings it back. Any other file
  moves to `trash/` in a commit of its own. The `change` tool refuses an agent's undo of
  a change that names no session: your safe delete, your Return, and a change proposed
  from a terminal. You undo those in a terminal.
- `atlas-obsidian journal publish` is your act too; the Obsidian plugin's Publish runs
  it as you. It captures a journal volume as a pending source, an edition. In the
  volume it writes only the publication history, and changes no note. The agent then absorbs the edition through a change, and the
  gate above holds for it. The `source` tool cannot write `origin: journal`.
- The `checkout` tool changes no knowledge document. `make` writes only under
  `checkout/`: the copies, the reading list, and the ledger. `return` proposes a change
  of the originals and never applies it. That change names no session, so the `change`
  tool refuses the agent's apply, and you decide with Approve or Cancel in the change
  document. Each write of the change carries the original's hash at the checkout as its
  base: `return` leaves out a copy whose original changed since, and apply refuses a
  write whose original changed after the proposal.
- The `wikify` tool changes no knowledge document. `start` copies a note into
  `scratchpad/` as `<name> · wikified.md` and never changes the original. It refuses a
  file that is not markdown, `Atlas.md`, and a note under `source-core/`, `changes/`,
  `sessions/`, `wiki-view/`, `trash/`, or `.obsidian/`. `mark` writes only a wikified
  copy: a note directly in `scratchpad/` whose name holds ` · wikified` and ends in
  `.md`. It refuses any other note. Neither commits. A topic for a new subject enters
  the wiki only through a change, and the gate above holds for it. The guard refuses
  an agent's edit of a wikified copy, so only `wikify mark` writes its marks.

### An agent edits files that code owns

The `guard` hook runs before each Write, Edit, MultiEdit, NotebookEdit, Codex
`apply_patch`, and Bash call, and before each call of the atlas tools that can write:
`change`, `source`, `vault`, `checkout`, and `wikify`. The other four, `search`,
`context`, `match`, and `lint`, only read, and the guard does not see them. In a vault, it refuses an agent's
edit of:

- `source-core/documents/`: a new document, which the change and source tools make; and
  a topic, a source, or a repository, which change only through a change.
- `source-core/originals/`, and any other folder under `source-core/`.
- `changes/` and `wiki-view/`, which code writes.
- `journals/`: your own writing, since 10.0. No agent edits a file there, the
  publication history that code writes included.
- `trash/`: what safe delete and a change's remove took out. You empty it.
- `checkout/`: the copies, reading lists, and ledger that the `checkout` tool writes.
  You read and edit the copies; `return` proposes your edits as a change.
- `Atlas.md`, a `.base` file, and `.claude/settings.local.json`.
- `sessions/`, except the Description, Progress, and Summary sections of the agent's own
  session document.
- `.atlas/config.json`, anything under `.obsidian/plugins/atlas/`, and the machine's
  `config.json` in `~/.atlas` (or `$ATLAS_HOME`). These files decide what Atlas runs; see
  [A shared vault changes what runs](#a-shared-vault-changes-what-runs).

The guard does not refuse an edit inside a linked repository, or in `threads/`, the
archive that the 9.0 migration writes.

The guard judges a path as the disk names it. It follows links, and it matches each folder
name without regard to case, so another spelling of a path meets the rule of the real file.
It uses the vault that holds the edited file, not the vault of the session's folder. So a
session that runs outside the vault gets the same refusals. A file in no vault meets only
the rule on the machine's `config.json`.

### A read-only agent writes

The plugin's three read-only agents are `wiki-audit`, `wiki-draft`, and `wiki-extract`.
For each of them, the guard refuses the edit tools, every shell command, and every atlas
call except the calls that only read: `search`, `context`, `match`, and `lint`;
`vault status`; `change show`; `source` with `chunks` or `read`; and `checkout` with
`list` or `candidates`. The guard counts a
call that this list does not name as a write.

### An agent uses the shell to skip the gate or to change what runs

The guard reads a Bash command as bash and zsh read it: quotes, escapes, separators, brace
lists, redirects, process substitutions, and a quoted string that a shell or `eval` runs.
It refuses a command that runs one of these, as `atlas-obsidian` or under the old name
`atlas`:

- `change … apply`, which applies a change without the gate;
- `change … undo`, which takes back a change, your own act among them;
- `hook`, which sends the binary a fake hook event, for example a fake prompt from you;
- `vault … migrate`, which rewrites the whole vault;
- `vault … trash`, your safe delete, which applies a remove without the gate;
- `journal … publish`, which copies a journal volume into the wiki's sources; only you
  decide when a journal is published;
- `config set` or `config unset` of `terminal_command` or `agent_commands`, the commands
  that Atlas runs.

A word that the shell builds when the command runs, such as a variable or `$(…)`, is out
of the guard's reach. You can run each of these commands yourself, in a terminal or with
`!` in a session.

### A shared vault changes what runs

Two files in a vault decide what Atlas runs:

- `.atlas/config.json` holds `terminal_command`, which opens the terminal for Start agent
  and Resume, and `agent_commands`, the agent command that Start agent runs. The vault's
  values win over the machine's `~/.atlas/config.json`.
- `.obsidian/plugins/atlas/data.json` holds `binaryPath`, the binary that the Obsidian
  plugin runs. The plugin uses a value that is not empty as it is, with no check.

`.git/info/exclude` does not list these files. So the Obsidian plugin's quiet snapshot,
or the next Atlas write, commits them with your hand edits, and a `git pull` of a shared
vault brings in another person's values. The Obsidian plugin runs `terminal_command` and
the agent command through a shell, as written. Trust a shared vault's `.atlas/` and `.obsidian/` folders as you
trust code: read a change to them before you pull it.

The guard refuses an agent's edit of these files and of the machine's config file, and the
shell commands that set the two keys. It does not cover a `git pull`, or a shell command
that writes the file.

### Vault contents direct a write or a delete

- A change document records the paths an apply may write, so an interrupted apply can be
  undone at the next session start. Recovery restores only clean relative `.md` paths inside
  the vault, outside `.git`, `.obsidian`, and `.claude`, where no folder in the path is a
  link out of the vault. Recovery removes files through `os.Root`, which refuses a path
  outside the vault.
- Undo takes its paths from git history, not from frontmatter.
- A change's remove deletes no file. Apply moves the document to
  `trash/<date>/<its path>` and records that path for recovery; undo moves it back.
- Before Atlas uses a title as a file name, it removes path separators, leading dots, and
  the characters that break a wikilink (`[ ] # ^`). Atlas builds a session file name from
  the date and a hex id.

### Commands run with attacker-chosen arguments

- The binary runs git as an argument list, never through a shell. Paths follow `--`.
- Atlas commits with `--no-verify`, so the vault's own git hooks do not run.
- The Obsidian plugin runs the binary with `execFile`, without a shell.

### A different program runs in place of the binary

The wrapper script, the Codex server entry, and the Obsidian plugin never search `PATH` for
the binary.

- The wrapper and the Codex entry run the first executable of `$ATLAS_BIN`,
  `$ATLAS_HOME/bin/atlas-obsidian` (default `~/.atlas/bin/atlas-obsidian`), and
  `~/go/bin/atlas-obsidian`.
- The Obsidian plugin runs its `binaryPath` setting when it is set. Otherwise it runs the
  first of `~/.atlas/bin/atlas-obsidian` and `~/go/bin/atlas-obsidian` that exists. It does
  not read `ATLAS_BIN` or `ATLAS_HOME`.

### Vault text runs as code in Obsidian

The Obsidian plugin renders no HTML from vault text. It adds text to the page as text
nodes.

### Machine-specific files reach a shared vault

`.git/info/exclude` lists `wiki-view/`, `.claude/settings.local.json`,
`.obsidian/workspace.json`, `.obsidian/workspace-mobile.json`, `.obsidian/graph.json`,
`.DS_Store`, and Atlas's temporary `.atlas-*` files, so they stay out of the vault's
history. It does not list `.atlas/config.json` or the Obsidian plugin's `data.json`; see
[A shared vault changes what runs](#a-shared-vault-changes-what-runs).

### Dependencies

- Go: `gopkg.in/yaml.v3` and the MCP Go SDK, nothing else.
- npm: exact versions, locked with integrity hashes. Only the build of the Obsidian
  plugin uses them.

## What Atlas does not guard against

- **An agent's shell.** A shell command can write any file you can, including the vault's
  documents. The guard checks a shell command only against the rules above. To limit the shell,
  use the host's permission settings.
- **Content you pull from someone else.** A shared vault's pages and a linked repository's
  files become text the agent reads. Review changes from other people before an agent works
  on them.
- **Your answer.** The gate checks that you sent a prompt after the proposal. It does not
  check that the prompt said yes.
- **Two machines that write one vault.** Each write takes a lock on `.git/atlas.lock`
  (flock). The lock works between the processes of one machine only. A vault that a sync
  service shares between machines gets no exclusion, so two machines can write at once.
- **The hook log.** `ATLAS_HOOK_LOG` records every hook event in full, including your
  prompts and tool output. Use it only to debug, and delete the file after use.
