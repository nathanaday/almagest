# atlas-obsidian

Atlas: the Go module `github.com/nathanaday/atlas-obsidian` (binary `atlas-obsidian`), the agent
plugin `atlas-obsidian` in the repository's own marketplace, and the Obsidian plugin in
`obsidian/`. Read `README.md` first. This file holds what the code and the README do not
say.

9.0.0 removed threads and chords, which 8.0.0 had put in place of the plans of 7.x. They
are a standalone reference project now, obsidian-threads (`~/projects/software/obsidian-threads`),
and the tag `threads-final` is the last commit that has them. Atlas keeps the knowledge
base: sources, repositories, topics, tags, views, sessions, and changes.
`atlas-obsidian vault migrate` moves an 8.x vault to the 9.0 layout in one commit
(`internal/migrate`): the thread documents and the chord canvases go to `threads/`
unchanged, and the documents that stay lose the fields and sections that served threads.
A vault older than 8.0 migrates with 8.1.1 first.

7.0.0 replaced the scope tree of 6.x with tags. Every document lies flat in
`wiki/documents/`, and code writes `views/`.
6.0.0 replaced the first design (5.x: one `atlas/<name>/` project folder in every
repository, and a terminal view). Nothing reads a 5.x file. The 5.x code is on the `v1`
branch and the `v1-final` tag.

## Sources of truth

| Thing | Location |
|---|---|
| The design: rules, document types, tools, hooks, skills | the SoftwareProjects vault (`~/Vaults/SoftwareProjects`), tag `tools/atlas-obsidian`: the design pages are sources there (start with `Atlas 7`), absorbed into topics |
| Each skill's contract | `skills/<name>/SKILL.md`, `skills/atlas/references/` |
| Each read-only agent | `agents/<name>.md` |
| The schemas of the six types | `internal/schema/schema.go` |

The design pages are the spec. When the code departs from them, the reason is below.

## Where the build departs from the design, and why

- **The setting is `tagging`, not `tags`.** `Atlas.md` holds `tagging: open | known`,
  and `vault init` takes `tagging`. `tags` is Obsidian's own property.
- **A change's base ignores what code derives** (`change.BaseHash`): code-owned fields,
  the lead callout, the code sections. A sync between a proposal and its apply refreshes
  a repository's git facts, and that was a conflict on every apply. Apply
  keeps the document's current code-owned fields (`keepDerived`).
- **A session records its agent's process and conversation.** The hooks walk up from
  their own process to the nearest `claude` or `codex` and keep its id in `pid`, and keep
  `transcript_path` in `transcript`. A sync ends a live session whose process is gone,
  and the sessions pane shows a session as open only while its process runs. Resume
  reads the conversation's first `cwd` and its config folder from the transcript, and
  finds the transcript of an older session by its id under `~/.claude*/projects/`.
- **Resume names a config folder only when it is not `~/.claude`.** Claude Code keys its
  login to the folder it was told: `CLAUDE_CONFIG_DIR=~/.claude claude` reports "Not
  logged in" on a machine logged in through the default. Verified 2026-10-01 on 2.1.286.
- **Agent preferences live in two files, and the binary merges them.** `~/.atlas/config.json`
  (`preferences`) and `<vault>/.atlas/config.json` (`atlas.vault-config.v1`); the vault
  wins per key (`vault.Merge`). Both decode strictly, so a typo is an error. The plugin
  reads and writes them only through `atlas-obsidian config --json`, never the files.
  The plugin settings of 8.0.2 (`agentCommand`, `terminal`, `terminalCommand`)
  move into the vault file once, and stay in `data.json` until the move succeeds.
- **A terminal launch fails where no one sees it** (osascript and `open` exit after the
  spawn), so `openTerminal` checks for the app first. `obsidian/scripts/probe-launch.mjs`
  opens a real terminal with a probe; record each result in TESTED.md.
- **A tag page that does not hold its tag's parent is a warning.** No document is lost
  by it; lint names the fix.
- **The migration warns about a live session, and does not refuse.** An 8.x agent in it
  still calls the thread tool, which 9.0 does not have. It refuses while a change is
  proposed, and when a file is in the way in `threads/`.
- **`threads/` is an archive Atlas does not read.** The index keeps its files as link
  targets, as it does the scratchpad's (`vault.Unread`), so a topic's link to an archived
  spec is no dead link. A rename does not rewrite the links inside it, as it does not in
  the scratchpad. The guard does not judge edits there, and lint does not lint it. A
  note of an archived type left in `wiki/documents` is the lint error `archived`.
- **A topic's `## Origin` is a plain section.** Promote wrote it from a stub's idea; 9.0
  has no promote, and the section stays the user's text.
- **`vault migrate` refuses a vault that has the 9.0 layout already**, and the guard
  refuses it from an agent's shell.
- **Packages:** the design's context package is `internal/brief`, since `context` is a
  standard Go package. `internal/derive` writes the code-owned parts of sources,
  repositories, and topics (lead callouts, the `atlas-repo` block, git facts).
- **Recovery needs the paths.** A change document gets a code-owned `paths` field while
  its apply is in flight: every path the apply may write. The field, not the status, is
  the mark: the file says `applied` for the derive step and keeps `paths` until the
  commit lands, and the commit records the final document, without `paths`, from the
  index (`Tx.Stage`). The derive step adds each path to `paths` before it writes it
  (`inFlight`). When the `Atlas-Change` commit exists, recovery takes the document from
  that commit, after it commits an edit made to the document since. Otherwise it records
  its base revision (HEAD as the crash left it) in a `recovering` field before anything
  else, commits the listed paths that differ from `HEAD` as `recovery: N files as found
  after a crash` (`CommitOnly`: those paths alone), puts the listed local documents
  (`Vault.Local`: a `.md` path that `Vault.Contain` accepts, outside `.git`, `.obsidian`,
  and `.claude` in any case) back from the base, and sets the
  change to proposed without `paths` or `recovering`. So a recovery that a crash stops
  partway does the same again.
  A `recovering` value that is not the full id of a commit in the history (or `none` in a
  repository with no commit) stops recovery before it changes anything.
  Both ways it unstages what it restored and the change document. Frontmatter comes from
  a pull or a shell too, and recovery runs at every session start.
- **A write that ends without its commit is put back.** `vault.Tx` keeps each path's
  bytes, or a link's target, before its first touch (`Keep`), and refuses a path it cannot
  keep. Every write defers `tx.End(&err)`: a failure before the commit rolls back and the
  error says so, like a failed `Commit`: "the vault is back as it was", or the paths it
  could not put back. A path saved since the write wrote it (Obsidian, an agent's Edit)
  stays as saved, and the message names it; `Tx.Settle` records what a git checkout (undo)
  or the migration left at its kept paths, so their later saves count too. Rollback
  unstages once the commit staged the paths, or once `Tx.Indexed` said a git checkout did
  (undo). The migration keeps every
  path it touches and commits with `Tx.CommitAll`.
- **`writeAtomic` syncs** the file before the rename and the folder after it. On macOS
  that is `F_FULLFSYNC`, about 0.2 s for a write. `writeAtomicIf` runs a check
  after the sync, right before the rename.
- **A derived write keeps a newer save.** Every sync (derive and git facts) writes
  through a `vault.Guard`: `NewGuard(idx, v or tx)`, whose `Write` calls
  `WriteIfUnchanged` with the bytes the index read. It compares before the write and again right
  before the rename, skips a file saved in between, and records it in `Skipped`; the next
  sync derives it. `SyncSettings` merges again when the harness wrote the file meanwhile.
- **The views sync never deletes a user's note.** A `.md` file in `views/` that no view
  stands for and that lacks `views.Notice` moves to `inbox/` under a free name, and every
  write says so: `moved_from_views` in each MCP write tool's result and in the JSON of each
  CLI write (with a line on stderr), `strays` in a sync and in the migration's report, and
  a notice in the plugin, whose runner reads `moved_from_views` from any result.
- **Every path built from input is contained.** `Vault.Contain` refuses an absolute or
  unclean path, `..`, `.git` in any case, a name over 255 bytes, and a path whose folders
  or final link resolve outside the vault. `Vault.Write`, `WriteIfChanged`, and `Remove`
  run it by default, so a caller cannot forget it; `vault.Tx` checks before it marks a
  path, and the moves of the misplaced-file sync and of the migration check both paths.
  The one unchecked writer, `WriteMachineIfChanged`, takes only fixed paths under
  `.obsidian/` and `.claude/`, so a user who links `.obsidian` to a shared folder keeps a
  working vault; `settings.go` and `prefs.go` write their machine files with
  `writeAtomic` directly. Inbox files pass
  `Vault.InboxFile` (a regular file kept in `inbox/`), a mention's note must be one the
  index holds, and a repository's files and a source's captured file are read through
  `os.Root`. Go 1.24's `os.Root` has no rename, so writes check the path before the
  atomic rename instead.
- **Each MCP tool handler recovers a panic** (`safe`), so one bad call returns an error
  and the server keeps serving.
- **The change tool keeps the gate, not the guard.** `change.Apply` takes a `Gate`; the
  MCP server passes `sessions.UserAnswered`, which reads the change's `session` and that
  session's `last_prompt`. It judges the document Apply resolved, under the lock, so no
  other name for the change and no other working folder gets past it. The CLI passes
  none: the terminal and Obsidian's Apply button are the user's. A change with no writes,
  which only absorbs sources, needs no answer and applies at once. Any other change with no `session`, or whose session ended with no prompt after the
  proposal, waits for Obsidian or the terminal; `change propose` from the CLI records no
  session.
- **Times carry seconds.** `proposed` and `last_prompt` are `2006-01-02T15:04:05`. With
  minutes, a yes typed in the minute of the proposal would not open the gate.
- **The gate counts only the user's turns.** A host sends a subagent's hand-back and a
  background task's notice as a UserPromptSubmit (`<agent-message …>`,
  `<task-notification …>`). The prompt hook sets `last_prompt` only for a real prompt
  (`hooks.UserTurn`). The live test found this hole.
- **The Stop hook blocks once for the agent's own debts.** An empty `## Description` in
  a session that touched a repository or proposed a change returns `decision: block` with
  the reason, once per session (`reminded`), and never while `stop_hook_active`. A
  change that waits for the user is a `systemMessage`, because the user acts on it.
- **The description follows the section.** Every hook event copies the first line of
  `## Description` into `description`, whatever tool wrote it.
- **An edit in a linked repository needs no ceremony.** 8.x refused it without a started
  thread with an open task; the user found that it blocked everyday work. The guard
  judges only files inside a vault, by the vault above the file.
- **Shell writes reach the record, not the guard.** The touched hook adds a repository to the session's `repositories` when a Bash
  command with a write mark (a redirect, `sed -i`, `git commit`, …) runs in it or names
  it. The skills tell the agent to change files with Edit and Write.
- **The shell runs neither the apply command of `change`, `vault migrate`, nor
  `atlas-obsidian hook`.** The guard refuses them, in every folder: two skip the gate,
  the other forges a user's turn. It is no sandbox; a shell can still write any file. To
  try one by hand from a session, type the command with `!`. `shellCommands`
  (`internal/hooks/shell.go`) reads the line as bash and zsh do: quotes, `$'…'` escapes
  (`\x{…}` too), backslashes and backslash-newlines, brace lists and ranges,
  separators, process substitutions (`<(…)` runs a command of its own), and redirects
  whose targets leave the words (`&>`, `{fd}>`, `<<<`, quoted targets); zsh's `=name`
  counts as the name. A quoted word or a heredoc body is read as a command only in a
  pipeline that runs a string (`runsString`: a runner such as sh, bash, eval, or ssh,
  in any case, where the command name stands, after assignments and wrappers like sudo,
  xargs, or timeout; `env -S`; find's `-exec sh`), as in `sh -c "…"` or `echo "…" | sh`;
  elsewhere a commit message or a grep pattern that names a refused subcommand is text,
  and a refusal of quoted text says why it counted. Brace expansion stops past 256
  words, counted before it expands. The binary's name compares without case, and
  options drop out; an option's value stays, so `apply` and `migrate` count anywhere
  after their subcommand. A word built at run time (a variable, `$(…)`, a glob, xargs)
  is out of its reach. `config set` and `config unset` of `terminal_command` or
  `agent_commands` are refused too.
- **The guard takes the vault above the file**, not the vault of the session's folder,
  so a session outside the vault gets the same refusals. A file in no vault meets only
  the rule on the home's `config.json`.
- **The guard judges the path the disk names.** `canonical` resolves links on the part
  that exists and spells each part as its folder entry, so `ATLAS.md`, `Wiki/documents/…`,
  a repository in another case, or a link into the vault meet the rule of the real file.
  A session document's code sections come from `doc.SectionOffsets`, which skips fenced
  headings, and an Edit, a MultiEdit, or a patch is judged by the document it leaves (`Input.leaves`,
  `codeChanged`): the frontmatter, the lead, and each code section come out as they went
  in, with each code heading as often as before by `doc.Headings`. An Edit that applies is judged by that alone, so its anchor may hold a
  code heading it keeps; its old text matches with curly and straight quotes as one, as
  the host's Edit may. A Codex patch is read as Codex 0.155.1 reads it: file markers with
  whitespace before them, context and removed lines that match with trailing whitespace
  ignored, every match counting, a hunk with neither at the end of the file; a hunk that
  matches nowhere is refused and the refusal says so. A move is judged as a delete of
  its source and a new file at its target.
- **The files that decide what runs are the user's.** The guard refuses an edit of the
  home's `config.json` and the vault's `.atlas/config.json` (`terminal_command`,
  `agent_commands`), and of anything under `.obsidian/plugins/atlas/`; fixed names
  compare without case, so a folder that does not exist yet cannot carry another case.
- **A read-only agent makes only the calls that read.** `readActions` lists them per
  tool; any other action, or a tool the list does not know, is a write.
- **The PreToolUse matcher anchors itself** (`^(…)$`) and names the atlas server of both
  hosts (`mcp__plugin_atlas-obsidian_atlas__…`, Codex's `mcp__atlas__…`): hosts test it
  unanchored.
- **Every hook waits for the lock half its timeout at most** (`hooks.Deadlines`,
  `Vault.LockWithin`, which bounds the in-process mutex too), then exits 1 with the held
  lock named, before the host kills it. The plugin test holds each deadline against
  `hooks.json`; SessionEnd's timeout is 3 s.
- **The host's own quiet agents are workers.** `Explore`, `Plan`, `claude-code-guide`,
  and `statusline-setup` get a line under `## Subagents`, like the plugin's three, and no
  document (`quietAgents`). `statusline-setup` has Edit; its edits meet the path rules
  like any agent's but stay off the session's record. Only the plugin's three are refused
  writes.
- **The harness settings keep no memory.** `SyncSettings(drop)` adds every repository
  path the documents name and removes only the paths the documents named before a write
  and no longer do. Apply and undo pass their before-list.
- **Machine files stay out of git** through `.git/info/exclude` (`vault.Excluded`:
  `views/`, `.claude/settings.local.json`, `.obsidian/workspace.json`,
  `.obsidian/workspace-mobile.json`, `.obsidian/graph.json`, `.DS_Store`, and the
  temporary `.atlas-*` files), so
  init edits no file of the user's. `EnsureFolders` rewrites the entries on every write,
  and untracks an excluded file that an older vault tracked, in a commit of its own.
- **A Base that equals a shipped copy upgrades in a commit of its own.** `Begin` writes
  the current copy after the snapshot and before the write, and commits it alone
  (`layout: upgrade <paths>`, `CommitOnly`), so no snapshot calls it a hand edit and no
  undo counts it. A refused commit puts the old copy back and the write goes on; the
  upgrade commit stays when the write then fails. A sync does not upgrade. Each shipped
  copy lives in `template/old/` (6.5), `template/old/7.0/` (7.x), and
  `template/old/8.1/` (8.x).
- **Titles also drop `[ ] # ^`**, which break a wikilink, `→`, which splits the old and
  new titles of a change heading, and control characters. A title the caller gives holds
  at most 150 bytes (`doc.CheckTitle`), so the titles code derives from it fit a file
  name; a captured file's name is cut instead, and one that cleans to nothing takes the
  source's id. " · " stays allowed: the heading parser finds the id at the end.
- **A create or a rename checks the disk** (`Vault.Occupied`), because the index compares
  titles by lower case only, and APFS also folds Unicode forms: "Café" in NFC and in NFD
  is one file. Capture numbers such a title; every other writer refuses it.
- **Match merges the hits of one kind on one document** (`mergeHits`), so one drafter
  writes that document. A near or a new subject is never merged, and neither are two
  subjects of different kinds that hit one document.
- **The binary is `atlas-obsidian`, not `atlas`.** Other programs install a binary named
  `atlas`, so that name could run the wrong program. 6.0 to 6.2 shipped `atlas`; the
  guard's shell rule still refuses both names.
- **The wrapper, the Codex entry, and the Obsidian plugin never search PATH or the system
  folders** for the binary, so another tool's binary never runs in its place. The wrapper
  and the Codex entry run the first executable of `$ATLAS_BIN`,
  `${ATLAS_HOME:-~/.atlas}/bin/atlas-obsidian`, and `~/go/bin/atlas-obsidian`. The plugin
  runs its `binaryPath` setting as given when it is set (`helpers.ts`), else the first of
  `~/.atlas/bin/atlas-obsidian` and `~/go/bin/atlas-obsidian` that exists; it reads
  neither `ATLAS_BIN` nor `ATLAS_HOME`.
- **One rule chooses the vault** (`vault.Select`): the vault a call names (`--vault`, or
  a tool's `vault` input), else `$ATLAS_VAULT` (a path or a machine-file name), else the
  vault above the working folder. The CLI, the MCP server, and the hooks call it; a bad
  `ATLAS_VAULT` is an error in the first two, and a hook falls back to the folder.
- **`--help` and `-h` run nothing.** `Run` answers them from the command's usage entry
  before dispatch, since `parse` would read `--help` as an option and `setup`, `vault
  init`, `hook`, and `mcp` act at once. A test (`usage_test.go`) holds every option a
  command reads to its usage entry.
- **Codex runs its own copy of the wrapper's lookup.** `.codex-plugin/plugin.json` holds
  the atlas server inline: `/bin/sh -c` with the same three candidates, no `cwd`, and
  `env_vars` for `ATLAS_BIN`, `ATLAS_HOME`, and `ATLAS_VAULT`. Codex 0.155.1 expands no
  placeholder in a plugin's MCP config, ignores a root `plugin.json`, and resolves a
  `cwd` against the plugin's cache folder, where the server would find no vault.
  `internal/plugin/codex_test.go` runs the entry and the wrapper side by side and
  requires the same result. Claude Code keeps reading `.mcp.json`.
- **`doctor` starts each host's server as the host runs it** (`host.Entry`,
  `host.Probe`): Codex's transport from `codex mcp list --json`, Claude Code's
  `.mcp.json` from the install path with `${CLAUDE_PLUGIN_ROOT}` expanded, with only
  `HOME`, `PATH`, `env`, and `env_vars`, and compares the tools with
  `mcpserver.ToolNames`. A `${` in the command fails; a script in the args may hold its
  own. Codex hook trust comes from `codex app-server`'s `hooks/list`
  (`host.CodexHookTrust`), since only Codex knows the trusted hash; `setup --agent
  codex` prints the same line on every run.

## Host facts, verified live on Claude Code 2.1.283 (2026-09-28)

- Hook events carry `session_id`, `cwd`, `hook_event_name`, `prompt_id`, and
  `permission_mode`. SessionStart has `source`; SessionEnd has `reason`; Stop has
  `stop_hook_active` and `last_assistant_message`.
- A subagent's SubagentStart, SubagentStop, and tool events carry `agent_id` and
  `agent_type` (`atlas-obsidian:wiki-audit`), and the parent's `session_id`.
- PostToolUse gives `tool_response` for every tool. For an MCP tool it is a JSON string
  that holds the result's JSON; `hooks.decodeAll` reads both.
- `cwd` follows the agent's `cd`: after `cd repo && …`, later events carry the
  repository's folder.
- Plugin tools are `mcp__plugin_atlas-obsidian_atlas__<tool>`; skills are
  `atlas-obsidian:<skill>`.
- `ATLAS_HOOK_LOG=<file>` appends every hook event the binary receives, one JSON line
  each. Use it to check a host's events.

Obsidian 1.13.7, verified live (2026-10-01): in a Base filter, an expression on a
property a note lacks fails, and the note drops out: `!x` and `!x || x == ""` both hide a
note with no `x`. `x.isEmpty()` is true for a missing property and for `""`. A hidden window renders nothing, so a DevTools screenshot hangs; bring the
second instance to the front by its pid with `osascript` first.

Obsidian 1.13.7, verified live (2026-09-28): the graph colors. A group's
`path:/^(?:…)$/` regex colors nodes, `view.dataEngine.setOptions({colorGroups})` recolors
an open graph, and a hidden window pauses timers and rendering.

To drive Obsidian without touching the user's app, run a second instance with its own data
folder: copy `obsidian-<version>.asar` from `~/Library/Application Support/obsidian/` into
the folder, write `obsidian.json` there with one vault, and run
`open -n -a Obsidian --args --user-data-dir=<folder> --remote-debugging-port=9333`. Then
evaluate JavaScript and take screenshots through the DevTools protocol at
`localhost:9333/json/list`, after `Page.bringToFront`. The settings open as a second page
of that list; choose the page whose title does not begin with "Settings". Point the
plugin at the build under test: set `binaryPath` in the plugin's settings.

Verified with Codex 0.155.1 in a scratch `CODEX_HOME` (2026-10-04): the plugin's server
entry, its start in a vault, and `doctor`'s server and hook trust lines (TESTED.md).

Verified live in Obsidian (TESTED.md): Start agent, Resume from the sessions pane, and the
settings tab.

Not yet verified: Codex's hook events in a session (the guard reads `apply_patch` paths;
the rest is untested on Codex), an atlas tool called from a Codex session, the
Notification types in a live session, and the rest of the Obsidian plugin inside Obsidian
(the Atlas navigator, repository panel, view folders, change bar, and badges have not run
in the app).

## Constraints

- Dependencies: `gopkg.in/yaml.v3` and the MCP Go SDK pinned at v1.4.0 (later versions
  need Go 1.25). Nothing else in Go.
- The SDK validates tool output against the schema it infers: a field without
  `omitempty` is required, so an optional pointer or a union needs `omitempty`.
- Every write takes `.git/atlas.lock`. The lock is not re-entrant: a write takes it once
  (`vault.Begin`, `vault.BeginWrite`, or `v.Lock()`), and inner functions assume it held.
- A document is found by id or title, never by a path a tool was given.
- Tests never touch a real `~/.atlas`: `testvault.New` sets `ATLAS_HOME`. They skip
  without git.
- Prose in skills, docs, and messages follows the user's global writing guide.

To try the migration on a copy of a real vault, copy the vault, then run
`ATLAS_MIGRATE_COPY=<copy> go test ./internal/migrate/ -run TestMigrateACopy -v`. It
prints the report and every lint finding.

## Build, test, and try

```bash
make build        # build/atlas-obsidian
make install      # ~/.atlas/bin/atlas-obsidian, with the version of .claude-plugin/plugin.json
make test
make obsidian     # build the Obsidian plugin and copy it into the binary's template
```

The binary, both plugin manifests, the marketplace entry, and the Obsidian plugin share
one version; the `internal/plugin` tests fail when they drift, or when the binary carries
an older Obsidian plugin than `obsidian/dist`. `make test` runs them; `make build` and
`make install` do not.

End to end in a scratch vault, without touching the real machine folder:

```bash
export ATLAS_HOME=/tmp/atlas-home ATLAS_BIN=$PWD/build/atlas-obsidian ATLAS_HOOK_LOG=/tmp/hooks.log
atlas-obsidian vault init --path /tmp/work --name Work
cd /tmp/work && claude -p "…" --plugin-dir /path/to/atlas-obsidian \
  --allowedTools "mcp__plugin_atlas-obsidian_atlas__*,Read,Grep,Glob,Skill,Edit,Agent"
claude -p --continue "yes" --plugin-dir …     # the user's answer at a gate
```

## Release

The installed plugin is a git clone of this repository's `main` at a commit.
`claude plugin update` fetches a new one only when the version in
`.claude-plugin/plugin.json` and `marketplace.json` went up, and the marketplace clone
fetches from GitHub, so push `main` first:

```bash
claude plugin marketplace update nathanaday-atlas-obsidian
claude plugin update atlas-obsidian@nathanaday-atlas-obsidian
make install
```

A session started inside this checkout may report that the project MCP server
`${CLAUDE_PLUGIN_ROOT}/scripts/atlas-obsidian` failed to start. Claude Code reads the
checkout's own `.mcp.json` as a project server, and that variable is set only for plugins.
It starts that server only when the checkout's untracked `.claude/settings.local.json`
enables it (`enabledMcpjsonServers` or `enableAllProjectMcpServers`). The plugin's own
server still runs; remove the project server from that file to silence the error.

## Not built yet

- The npm package that `npx atlas-obsidian setup` installs from (the Stack page). Setup
  runs from the binary for now.
- Fetching a page from a URL. `source capture --text FILE --locator URL` already records
  `origin: url`; nothing fetches the page.
- The "Later" items of the Obsidian plugin page.
