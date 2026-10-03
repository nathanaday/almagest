# atlas-obsidian

Atlas: the Go module `github.com/nathanaday/atlas-obsidian` (binary `atlas-obsidian`), the agent
plugin `atlas-obsidian` in the repository's own marketplace, and the Obsidian plugin in
`obsidian/`. Read `README.md` first. This file holds what the code and the README do not
say.

8.0.0 replaced the plans of 7.x with threads and chords (`internal/thread`). A thread is
a stub, a spec, task lists, and verifications; code derives its status and no call sets
it. A chord orders threads, and code writes its canvas in `chords/`. Each skill's
contract and `skills/atlas/references/threads.md` are the record of the 8.0 design.

7.0.0 replaced the scope tree of 6.x with tags. Every document lies flat in
`wiki/documents/`, and code writes `views/`. `atlas-obsidian vault migrate` moves a 6.x
or a 7.x vault to the 8.0 layout in one commit (`internal/migrate`; `v8.go` is the step
from 7.x).
6.0.0 replaced the first design (5.x: one `atlas/<name>/` project folder in every
repository, and a terminal view). Nothing reads a 5.x file. The 5.x code is on the `v1`
branch and the `v1-final` tag.

## Sources of truth

| Thing | Location |
|---|---|
| The design: rules, document types, tools, hooks, skills | the SoftwareProjects vault (`~/Vaults/SoftwareProjects`), tag `tools/atlas-obsidian`: the design pages are sources there (start with `Atlas 7`), absorbed into topics |
| Each skill's contract | `skills/<name>/SKILL.md`, `skills/atlas/references/` |
| Each read-only agent | `agents/<name>.md` |
| The schemas of the twelve types | `internal/schema/schema.go` |

The design pages are the spec. When the code departs from them, the reason is below.

## Where the build departs from the design, and why

- **The setting is `tagging`, not `tags`.** `Atlas.md` holds `tagging: open | known`,
  and `vault init` takes `tagging`. `tags` is Obsidian's own property.
- **Event callouts are `[!event-<kind>]`**, so no event kind shares a callout name with
  a type.
- **`thread resolve` takes no text.** `became` and the `resolved` event are the record.
- **The board has a `waiting` group:** threads that come after a thread not yet verified.
- **A thread closes with no call.** It is closed when it is verified and an applied
  change absorbed its spec and its last verification (`Board.derive`). The gate
  (`sessions.UserAnswered`) makes such a change wait for the user's turn even with no
  writes, since its apply closes a thread.
- **A verification goes stale by hash.** It records `spec_hash` (the requirements and
  the rules) and `tasks_hash` (the ids of the tasks that are not dropped). A reworded
  goal changes neither.
- **The order of a chord lives on the stubs** (`chord`, `after`). The canvas is a view of
  it that the user may edit. The chord's `canvas` field holds the hash of the graph code
  wrote last, so a sync tells the user's edit from its own and leaves the edit alone
  until `chord canvas --save` or `--write`.
- **A change's base ignores what code derives** (`change.BaseHash`): code-owned fields,
  the lead callout, the code sections. A sync between a proposal and its apply refreshes
  a repository's git facts, and that was a conflict on every close of a thread. Apply
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
  The plugin settings of 8.0.2 and 8.0.3 (`agentCommand`, `terminal`, `terminalCommand`)
  move into the vault file once, and stay in `data.json` until the move succeeds.
- **A terminal launch fails where no one sees it** (osascript and `open` exit after the
  spawn), so `openTerminal` checks for the app first. `obsidian/scripts/probe-launch.mjs`
  opens a real terminal with a probe; record each result in TESTED.md.
- **The canvas legend is in the canvas bar, not on the canvas.** 8.0.2 to 8.1.0 put six
  groups (`atlas-legend-1` to `-6`) on each chord canvas; any canvas write removes them
  (`dropLegend`). The bar's swatches use `--color-*-rgb`, because the `--canvas-color-*`
  variables exist only inside a canvas.
- **thread-audit runs shell commands.** It checks work by running its tests. The guard
  refuses its edits and its atlas writes, not its shell.
- **A tag page that does not hold its tag's parent is a warning.** No document is lost
  by it; lint names the fix.
- **The migration warns about a live session, and does not refuse.** Its threads and
  tasks become plans in the first step, and threads in the second. A root plan takes the tag of every
  scope of its thread, area or repository.
- **`vault migrate` refuses a vault that has the 8.0 layout already**, and the guard
  refuses it from an agent's shell.
- **Packages:** the design's context package is `internal/brief`, since `context` is a
  standard Go package. `internal/derive` writes the code-owned parts of sources,
  repositories, and topics (lead callouts, the `atlas-repo` block, git facts).
- **Recovery needs the paths.** A change document gets a code-owned `paths` field while
  it is `applying`: every path the apply may write. Recovery restores those that are
  local documents (`Vault.Local`: a `.md` path that `Vault.Contain` accepts, outside
  `.git`, `.obsidian`, and `.claude` in any case), and skips the rest. Frontmatter comes
  from a pull or a shell too, and recovery runs at every session start. Apply removes the
  field when it ends.
- **Every path built from input is contained.** `Vault.Contain` refuses an absolute or
  unclean path, `..`, `.git` in any case, a name over 255 bytes, and a path whose folders
  or final link resolve outside the vault. Every write, remove, and move of `vault.Tx`
  passes it, so no document write lands outside the vault whatever its caller.
  `Vault.Write` with a fixed machine path (`.obsidian/`, `.claude/`) does not, so a user
  who links `.obsidian` to a shared folder keeps a working vault. Inbox files pass
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
  none: the terminal and Obsidian's Apply button are the user's. A change with no
  `session` waits for Obsidian.
- **Times carry seconds.** `proposed`, `last_prompt`, and an event's `at` are
  `2006-01-02T15:04:05`. With minutes, a yes typed in the minute of the proposal would not
  open the gate. Two events of one subject in one second get a strict order, so the
  status they derive is certain.
- **The gate counts only the user's turns.** A host sends a subagent's hand-back and a
  background task's notice as a UserPromptSubmit (`<agent-message …>`,
  `<task-notification …>`). The prompt hook sets `last_prompt` only for a real prompt
  (`hooks.UserTurn`). The live test found this hole.
- **The Stop hook blocks once for the agent's own debts.** An empty `## Description`, or
  work in a repository with no task checked and no `## Progress` line in the session
  document, returns `decision: block`
  with the reason, once per session (`reminded`), and never while `stop_hook_active`. A
  change that waits for the user is a `systemMessage`, because the user acts on it.
- **The description follows the section.** Every hook event copies the first line of
  `## Description` into `description`, whatever tool wrote it.
- **Shell writes reach the record, not the guard.** The design keeps Bash out of the edit
  rule. The touched hook adds a repository to the session's `repositories` when a Bash
  command with a write mark (a redirect, `sed -i`, `git commit`, …) runs in it or names
  it. The skills tell the agent to change files with Edit and Write.
- **The shell runs neither the apply command of `change`, `vault migrate`, nor
  `atlas-obsidian hook`.** The guard refuses them, in every folder: two skip the gate,
  the other forges a user's turn. It is no sandbox; a shell can still write any file. To
  try one by hand from a session, type the command with `!`.
- **The guard takes the vault above the file**, not the vault of the session's folder,
  so a session outside the vault gets the same refusals.
- **`thread-audit` names a repository by its path**: `git -C <path> …`, since it runs
  in the vault.
- **The host's own read-only agents are workers.** `Explore`, `Plan`,
  `claude-code-guide`, and `statusline-setup` get a line under `## Subagents`, like the
  plugin's four, and no document. Only the plugin's four are refused writes.
- **The harness settings keep no memory.** `SyncSettings(drop)` adds every repository
  path the documents name and removes only the paths the documents named before a write
  and no longer do. Apply and undo pass their before-list.
- **Machine files stay out of git** through `.git/info/exclude` (`views/`,
  `.claude/settings.local.json`, `.obsidian/workspace*.json`, `.obsidian/graph.json`), so
  init edits no file of the user's. `EnsureFolders` rewrites the entries on every write,
  and untracks an excluded file that an older vault tracked, in a commit of its own.
- **Titles also drop `[ ] # ^`**, which break a wikilink, `→`, which splits the old and
  new titles of a change heading, and control characters. A title the caller gives holds
  at most 150 bytes (`doc.CheckTitle`), so the titles code derives from it fit a file
  name; a captured file's name is cut instead, and one that cleans to nothing takes the
  source's id. " · " stays allowed: the heading parser finds the id at the end.
- **A create or a rename checks the disk** (`Vault.Occupied`), because the index compares
  titles by lower case only, and APFS also folds Unicode forms: "Café" in NFC and in NFD
  is one file. Capture numbers such a title; every other writer refuses it.
- **Match merges subjects that hit one document**, so one drafter writes each document.
- **A dropped thread does not block** the threads after it.
- **The binary is `atlas-obsidian`, not `atlas`.** Other programs install a binary named
  `atlas`, so that name could run the wrong program. 6.0 to 6.2 shipped `atlas`; the
  guard's shell rule still refuses both names.
- **The wrapper and the Obsidian plugin never search PATH or the system folders** for the
  binary, so another tool's binary never runs in its place. They look at `$ATLAS_BIN`
  (the wrapper only), `~/.atlas/bin/atlas-obsidian`, and `~/go/bin/atlas-obsidian`.

## Host facts, verified live on Claude Code 2.1.283 (2026-09-28)

- Hook events carry `session_id`, `cwd`, `hook_event_name`, `prompt_id`, and
  `permission_mode`. SessionStart has `source`; SessionEnd has `reason`; Stop has
  `stop_hook_active` and `last_assistant_message`.
- A subagent's SubagentStart, SubagentStop, and tool events carry `agent_id` and
  `agent_type` (`atlas-obsidian:thread-audit`), and the parent's `session_id`.
- PostToolUse gives `tool_response` for every tool. For an MCP tool it is a JSON string
  that holds the result's JSON; `hooks.decodeAll` reads both.
- `cwd` follows the agent's `cd`: after `cd repo && …`, later events carry the
  repository's folder.
- Plugin tools are `mcp__plugin_atlas-obsidian_atlas__<tool>`; skills are
  `atlas-obsidian:<skill>`.
- `ATLAS_HOOK_LOG=<file>` appends every hook event the binary receives, one JSON line
  each. Use it to check a host's events.

Obsidian 1.13.7, verified live (2026-10-01): in a Base filter, an expression on a
property a note lacks fails, and the note drops out: `!chord` and `!chord || chord == ""`
both hide a stub with no `chord`. `chord.isEmpty()` is true for a missing property and
for `""`. A hidden window renders nothing, so a DevTools screenshot hangs; bring the
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

Verified live on 8.0.0 (2026-09-30), Obsidian 1.13.7 and Claude Code 2.1.286: the thread
bar on a stub and its folded properties; the chord canvas, its bar, and Save order from a
redrawn arrow; `View · Threads`; and one thread from a request to closed in a scratch
vault with `claude -p`, where the agent used thread stub, spec, tasks, start, check,
verify (three rounds with thread-audit), finding, and the closing change.

Not yet verified: Codex's hook events (the guard reads `apply_patch` paths; the rest is
untested on Codex), the Notification types in a live session, and the rest of the Obsidian
plugin inside Obsidian (the Atlas navigator, repository panel, view folders, change bar,
badges, and sessions pane have not run in the app).

## Constraints

- Dependencies: `gopkg.in/yaml.v3` and the MCP Go SDK pinned at v1.4.0 (later versions
  need Go 1.25). Nothing else in Go.
- The SDK validates tool output against the schema it infers: a field without
  `omitempty` is required, so an optional pointer or a union needs `omitempty`.
- Every write takes `.git/atlas.lock`. The lock is not re-entrant: a write takes it once
  (`vault.Begin`, `change.Begin`, or `v.Lock()`), and inner functions assume it held.
- A document is found by id or title, never by a path a tool was given.
- Tests never touch a real `~/.atlas`: `testvault.New` sets `ATLAS_HOME`. They skip
  without git.
- Prose in skills, docs, and messages follows the user's global writing guide.

To try the migration on a copy of a real vault, copy the vault, then run
`ATLAS_MIGRATE_COPY=<copy> go test ./internal/migrate/ -run TestMigrateACopy -v`. It
prints the report, every lint finding, and each thread's status.

## Build, test, and try

```bash
make build        # build/atlas-obsidian
make install      # ~/.atlas/bin/atlas-obsidian, with the version of .claude-plugin/plugin.json
make test
make obsidian     # build the Obsidian plugin and copy it into the binary's template
```

The binary, both plugin manifests, the marketplace entry, and the Obsidian plugin share
one version; `internal/plugin` fails the build when they drift, or when the binary carries
an older Obsidian plugin than `obsidian/dist`.

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

A session started inside this checkout reports that the project MCP server
`${CLAUDE_PLUGIN_ROOT}/scripts/atlas-obsidian` failed to start: Claude Code reads the checkout's
own `.mcp.json` as a project server, and that variable is set only for plugins. The
message is noise.

## Not built yet

- The npm package that `npx atlas-obsidian setup` installs from (the Stack page). Setup
  runs from the binary for now.
- Capture from a URL (`origin: url`).
- The "Later" items of the Obsidian plugin page.
