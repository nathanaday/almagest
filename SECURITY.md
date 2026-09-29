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
change, you read the preview and answer, and then the agent applies the change.

- The `change` tool refuses the agent's `apply` until you have sent a prompt after the
  proposal, in the session that proposed it. The tool checks this on the change document it
  is about to apply, while it holds the vault's lock.
- A subagent's report and a background task's notice arrive as prompts, but they do not
  count as your turn.
- A change with no recorded session, or with a proposal time that does not parse, is
  refused. You can still apply it with the Apply button in Obsidian.
- The gate does not read your prompt. The skill reads your answer and decides whether to
  apply.
- Two ways to apply have no gate: `atlas-obsidian change apply` in your terminal, and the
  Apply button in Obsidian.

### An agent edits files that code owns

The `guard` hook runs before each Write, Edit, MultiEdit, NotebookEdit, Codex
`apply_patch`, and Bash call. It refuses:

- an edit under `wiki/` or `changes/`, and an edit of `Atlas.md`, a `.base` file, or
  `.claude/settings.local.json`;
- an edit of another session's document, or of the fields and sections the hooks own in
  the agent's own session document;
- an edit of a thread document's frontmatter or lead callout;
- an edit inside a linked repository when the session has no open thread that covers the
  repository.

The guard uses the vault that holds the edited file, not the vault of the session's
folder. A session that runs outside the vault gets the same refusals.

### A read-only agent writes

The plugin's four read-only agents (`wiki-extract`, `wiki-draft`, `wiki-audit`,
`thread-review`) cannot use the edit tools, the atlas tools' write actions, or Bash. The one
exception is `thread-review`, which can run `git log`, `git diff`, and `git show`. The guard
refuses such a command when it holds a shell operator, `--output`, `-c`, `--ext-diff`, or
`--textconv`.

### An agent uses the shell to skip the gate

The guard refuses a Bash command that runs `atlas-obsidian change … apply` or
`atlas-obsidian hook`. It refuses the same commands under the old name `atlas`. The first would apply a change without the gate. The second would send the binary
a fake hook event, for example a fake prompt from you.

### Vault contents direct a write or a delete

- A change document records the paths an apply may write, so an interrupted apply can be
  undone at the next session start. Recovery restores only clean relative `.md` paths inside
  the vault, outside `.git`, `.obsidian`, and `.claude`, where no folder in the path is a
  link out of the vault. Recovery removes files through `os.Root`, which refuses a path
  outside the vault.
- Undo takes its paths from git history, not from frontmatter.
- Before Atlas uses a title as a file name, it removes path separators, leading dots, and
  the characters that break a wikilink (`[ ] # ^`). Atlas builds a session file name from
  the date and a hex id.

### Commands run with attacker-chosen arguments

- The binary runs git as an argument list, never through a shell. Paths follow `--`.
- Atlas commits with `--no-verify`, so the vault's own git hooks do not run.
- The Obsidian plugin runs the binary with `execFile`, without a shell.

### A different program runs in place of the binary

The wrapper script and the Obsidian plugin never search `PATH` for the binary.
The wrapper looks at `$ATLAS_BIN`, `$ATLAS_HOME/bin/atlas-obsidian` (default
`~/.atlas/bin/atlas-obsidian`), and `~/go/bin/atlas-obsidian`. The Obsidian plugin looks
at its setting, `~/.atlas/bin/atlas-obsidian`, and `~/go/bin/atlas-obsidian`.

### Vault text runs as code in Obsidian

The Obsidian plugin renders no HTML from vault text. It adds text to the page as text
nodes.

### Machine-specific files reach a shared vault

`.claude/settings.local.json` and Obsidian's workspace and graph files are listed in
`.git/info/exclude`, so they stay out of the vault's history.

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
- **The hook log.** `ATLAS_HOOK_LOG` records every hook event in full, including your
  prompts and tool output. Use it only to debug, and delete the file after use.
