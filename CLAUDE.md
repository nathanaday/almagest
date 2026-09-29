# atlas-obsidian

Atlas: the Go module `github.com/nathanaday/atlas-obsidian` (binary `atlas`), the agent
plugin `atlas-obsidian` in the repository's own marketplace, and the Obsidian plugin in
`obsidian/`. Read `README.md` first. This file holds what the code and the README do not
say.

6.0.0 replaced the first design (5.x: one `atlas/<name>/` project folder in every
repository, and a terminal view) with this one. Nothing reads a 5.x file. The 5.x code is
on the `v1` branch and the `v1-final` tag.

## Sources of truth

| Thing | Location |
|---|---|
| The design: rules, document types, tools, hooks, skills | `docs/design/` (start with `Atlas V2.md`) |
| Each skill's contract | `skills/<name>/SKILL.md`, `skills/atlas/references/` |
| Each read-only agent | `agents/<name>.md` |
| The schemas of the thirteen types | `internal/schema/schema.go` |

The design pages are the spec. When the code departs from them, the reason is below.

## Where the build departs from the design, and why

- **Recovery needs the paths.** A change document gets a code-owned `paths` field while
  it is `applying`: every path the apply may write. Recovery restores exactly those.
  Apply removes the field when it ends.
- **Times carry seconds.** `proposed` and `last_prompt` are `2006-01-02T15:04:05`. With
  minutes, a yes typed in the minute of the proposal would not open the gate.
- **The gate counts only the user's turns.** A host sends a subagent's hand-back and a
  background task's notice as a UserPromptSubmit (`<agent-message …>`,
  `<task-notification …>`). The prompt hook sets `last_prompt` only for a real prompt
  (`hooks.UserTurn`). The live test found this hole.
- **The Stop hook blocks once for the agent's own debts.** An empty `## Description` or
  a missing progress line returns `decision: block` with the reason, once per session
  (`reminded`), and never while `stop_hook_active`. A change that waits for the user is a
  `systemMessage`, because the user acts on it.
- **The description follows the section.** Every hook event copies the first line of
  `## Description` into `description`, whatever tool wrote it.
- **Shell writes reach the record, not the guard.** The design keeps Bash out of the
  thread rule. The touched hook adds a repository to the session's `repositories` when a
  Bash command with a write mark (a redirect, `sed -i`, `git commit`, …) runs in it or
  names it. The skills tell the agent to change files with Edit and Write.
- **`thread-review` may name a repository**: `git -C <path> log|diff|show`, since it runs
  in the vault.
- **The host's own read-only agents are workers.** `Explore`, `Plan`,
  `claude-code-guide`, and `statusline-setup` get a line under `## Subagents`, like the
  plugin's four, and no document. Only the plugin's four are refused writes.
- **The harness settings keep no memory.** `SyncSettings(drop)` adds every repository
  path the pages name and removes only the paths the pages named before a write and no
  longer do. Apply and undo pass their before-list.
- **Machine files stay out of git** through `.git/info/exclude`
  (`.claude/settings.local.json`, `.obsidian/workspace*.json`, `.obsidian/graph.json`), so
  init edits no file of the user's. `EnsureFolders` rewrites the entries on every write,
  and untracks an excluded file that an older vault tracked, in a commit of its own.
- **Scopes are browsable without folders.** `scope` stays the one record of where a
  page belongs; folders follow the type. Code derives from it: `chain` on every wiki
  page (its scope and the areas above it, top first; an area's or repository's own
  ancestors), a lead callout on each area and repository page with the path from the
  vault and an inline `base` view filtered on `chain.contains(this.file.asLink())`, and a
  map of the graph as Atlas.md's lead callout. `scope.Heal` keeps them current: sync
  writes them, and apply, undo, and capture include them in their commits, so a renamed
  or re-parented area moves the pages below it in one commit. Folders or tags were
  rejected: each is a second record of scope that a rename or a new parent must rewrite.
  The view is inline, not a Base file, because a Base opened alone has no `this` and
  shows nothing; 0.1.0's `wiki/Scope.base` is removed by sync when unedited.
- **Threads carry a chain, and the board has a canvas.** A stub's `chain` is each of
  its scopes and the areas above each, so an area page and the By area view find a
  thread scoped to a repository below the area. `threads/Threads.canvas` is a card per
  open thread in a group per first scope. `scope.Derive` writes both, so every commit that
  heals scopes (apply, undo, capture, and each `thread` call) keeps them current. Code
  owns the cards on the grid; a card off the grid, a group's position, the user's nodes,
  and edges between live nodes survive a sync. `Threads.base` of 0.1.0 is replaced by
  sync when unedited (`vault.upgradeBases`).
- **Titles also drop `[ ] # ^`**, which break a wikilink.
- **The vault's name is not a link target.** Obsidian resolves `[[work]]` to a file named
  `work`, and the design's own example has an area `work` in a vault `Work`. The vault is
  an empty `scope` or `parent`.
- **Match merges subjects that hit one page**, so one drafter writes each page.
- **A dropped dependency does not block** a task's readiness.
- **The wrapper and the Obsidian plugin never search PATH or the system folders** for
  `atlas`: another tool installs a binary of that name there. They look at `$ATLAS_BIN`
  (the wrapper only), `~/.atlas/bin/atlas`, and `~/go/bin/atlas`.

## Host facts, verified live on Claude Code 2.1.283 (2026-09-28)

- Hook events carry `session_id`, `cwd`, `hook_event_name`, `prompt_id`, and
  `permission_mode`. SessionStart has `source`; SessionEnd has `reason`; Stop has
  `stop_hook_active` and `last_assistant_message`.
- A subagent's SubagentStart, SubagentStop, and tool events carry `agent_id` and
  `agent_type` (`atlas-obsidian:thread-review`), and the parent's `session_id`.
- PostToolUse gives `tool_response` for every tool. For an MCP tool it is a JSON string
  that holds the result's JSON; `hooks.decodeAll` reads both.
- `cwd` follows the agent's `cd`: after `cd repo && …`, later events carry the
  repository's folder.
- Plugin tools are `mcp__plugin_atlas-obsidian_atlas__<tool>`; skills are
  `atlas-obsidian:<skill>`.
- `ATLAS_HOOK_LOG=<file>` appends every hook event the binary receives, one JSON line
  each. Use it to check a host's events.

Obsidian 1.13.7, verified live (2026-09-28): the graph colors. A group's
`path:/^(?:…)$/` regex colors nodes, `view.dataEngine.setOptions({colorGroups})` recolors
an open graph, and a hidden window pauses timers and rendering.

To drive Obsidian without touching the user's app, run a second instance with its own data
folder: copy `obsidian-<version>.asar` from `~/Library/Application Support/obsidian/` into
the folder, write `obsidian.json` there with one vault, and run
`open -n -a Obsidian --args --user-data-dir=<folder> --remote-debugging-port=9333`. Then
evaluate JavaScript and take screenshots through the DevTools protocol at
`localhost:9333/json/list`, after `Page.bringToFront`.

Not yet verified: Codex's hook events (the guard reads `apply_patch` paths; the rest is
untested on Codex), the Notification types in a live session, and the rest of the Obsidian
plugin inside Obsidian (the change bar, the badges, and the sessions pane have not run in
the app).

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

## Build, test, and try

```bash
make build        # build/atlas
make install      # ~/.atlas/bin/atlas, with the version of .claude-plugin/plugin.json
make test
make obsidian     # build the Obsidian plugin and copy it into the binary's template
```

The binary, both plugin manifests, the marketplace entry, and the Obsidian plugin share
one version; `internal/plugin` fails the build when they drift, or when the binary carries
an older Obsidian plugin than `obsidian/dist`.

End to end in a scratch vault, without touching the real machine folder:

```bash
export ATLAS_HOME=/tmp/atlas-home ATLAS_BIN=$PWD/build/atlas ATLAS_HOOK_LOG=/tmp/hooks.log
atlas vault init --path /tmp/work --name Work
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
`${CLAUDE_PLUGIN_ROOT}/scripts/atlas` failed to start: Claude Code reads the checkout's
own `.mcp.json` as a project server, and that variable is set only for plugins. The
message is noise.

## Not built yet

- The npm package that `npx atlas-obsidian setup` installs from (the Stack page). Setup
  runs from the binary for now.
- Capture from a URL (`origin: url`).
- The "Later" items of the Obsidian plugin page.
