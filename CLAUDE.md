# atlas-obsidian

A Go binary and a plugin for Claude Code and Codex. The binary makes a folder of work a
project, which holds a wiki and the threads of the work in one folder, and
serves the MCP tools either agent uses in it; the plugin carries the skills and
hooks. The atlas side lists every project and shows them in a terminal view.

Read `README.md` first. This file holds what the code and README do not say.

Atlas V2 is built beside V1 in `v2-exp/`: its own Go module and its own plugin
(`atlas-obsidian-v2-exp`). Work on V2 reads `v2-exp/CLAUDE.md`; nothing below applies
to it, and V1 stays as it is.

## Sources of truth

| Thing | Location |
|---|---|
| The current design: one entity. A project holds its wiki and its threads in one folder | `docs/v4-design.md` |
| Threads: a project's state as stub, spec, plan, receipt documents | `docs/threads-design.md` |
| Core design and the reasons behind it: the engine, one operation one commit | `docs/core-design.md` |
| Members: a project mirrors other projects' wikis under `wiki/projects/` | `docs/members-design.md` |
| History, not contracts: the designs this one replaced | `docs/v3-design.md`, `docs/v2-design.md`, `docs/atlas-design.md`, `docs/tasks-design.md`, `docs/stubs-design.md`, `docs/spec.md`, `docs/superpowers/` |
| The skill system: three categories, the map, the workflow, who owns what, the form of a skill | `docs/skills.md` |
| Each skill's contract | `skills/<name>/SKILL.md`, `skills/wiki/references/`, `skills/thread/references/` |

## Why the project exists

claude-obsidian established a workflow the author wants to keep: an inbox,
immutable captured sources, one reviewed operation per change, pages that cite
their sources. Its Python core and its approval ritual (copy a hash, repeat a
timestamp) got in the way, and nothing showed many vaults at once. Atlas keeps
the workflow, replaces the core with Go and MCP, uses git in the work as the
safety net, and adds the view across projects.

Sharing knowledge between projects was the reason v2 and v3 split a knowledge
base from a project. A week of use showed the split cost more than it paid:
two vaults per project, two inboxes, a link to maintain, and a decision before
every session. v4 merged them. The mission returns as its own feature, built
over projects that each hold a wiki; see "Left for later" in
`docs/v4-design.md`.

## Five rules

1. **One operation, one commit, in the wiki.** `plan` validates, the user sees
   the preview, `apply` commits. There is no other write path into `wiki/`; a
   PreToolUse hook refuses Write and Edit there. Every git command an operation
   runs is scoped to `project.EngineScope` (`wiki/`, `.raw/`, `inbox/`,
   `project.json`), so an apply, the manual commit before it, and an undo never
   see the user's code or a thread document.
2. **The folder is the user's.** Apply commits hand edits in the wiki as
   `manual` operations first, so a rollback never touches what the user typed in
   Obsidian. Every other folder is the user's outright.
3. **A thread's stage is never set:** it is the furthest document that exists
   (stub, spec, plan, receipt). The `thread` tool files a document, and that is
   the only way a thread moves, so every change of state is a page the user can
   see. A receipt closes the thread and its card moves to `threads/archive/`. A
   thread names its phase; a phase never lists its threads and has no status.
   Threads are an opt-in (`threads: true` in `project.json`); off, every
   thread function returns `threads.ErrOff` and nothing reads `threads/`.
4. **Code owns what code can derive.** `wiki/log.md`, the source ledger, the
   thread cards, the first callout of every document, the board, and the
   mirrors under `wiki/projects/` and `threads/projects/`. The model writes
   prose; it never targets a derived page. The guard refuses the cards and the
   board (the pages directly under `threads/`), `project.json`, both mirrors,
   and a new file written straight into a stage folder.
5. **Ids travel; paths stay.** `project.json` holds no path; the atlas config
   holds every project's work folder and nothing else. A project heals its own
   entry when a session starts in it (`manage.RegisterProject`).
   `~/.atlas-obsidian/state/registry.json` is derived, and `refresh` rebuilds it
   in full.

## One entity, two halves

`atlas/<name>/` holds both. The line between them is not the folder; it is who
may write:

| Half | Paths | Who writes |
|---|---|---|
| The wiki | `wiki/`, `.raw/`, the source ledger, `project.json` | `plan` then `apply`, `capture`, and `project.UpdateConfig` for the identity file |
| The threads | `threads/` | the `thread` and `phase` tools write the cards, the frontmatter, the callouts, and the board; the model writes a document's prose with Edit; `mirror.Sync` writes `threads/projects/` |
| Neither | `inbox/`, `ideas/`, `.obsidian/` | the user; `stage` and `capture` for `inbox/` |

## Tool, skill, or hook

Three carriers, and a new capability splits across them rather than picking one.

1. A tool is a fact or a commit. Code owns whatever two correct runs must
   answer the same way (`status`, `route`, `threads`, `history`, `lint`, `overlap`) and
   every path that changes bytes (`capture`, `plan`, `apply`, `undo`,
   `thread`, `phase`, `stub`, `project`). No tool writes prose; `thread` files
   the text the model gives it.
2. A skill is a procedure and a policy: which depth to read at, what counts
   as adequate evidence, how to cite, when to stop, which skill comes next.
   A skill is advice. When the model must be refused instead of advised, the
   rule belongs in a tool or in a hook, which is why `PreToolUse` guards
   `wiki/` and no skill asks nicely.
3. Tools and skills do not pair one to one, and naming them alike is the
   trap. `plan` and `apply` serve every writing skill, and `thread` serves
   every stage skill; `thread-work` calls the stage skills, not a tool of its
   own. Tools are nouns and
   stay few, because every description sits in every session's context;
   skills are verbs and load when they trigger. v4 folded `vault` and `mode`
   into `project` for that reason: one entity to create and edit, and a mode is
   one more field of its identity file. A wanted `wiki-query` tool means the
   split has not happened yet: the code part of querying is candidate
   selection (`search`) and a source's standing in the ledger, and the skill
   keeps the rest.
4. Skills are named `<noun>-<verb>` on three nouns: `atlas`, `wiki`,
   `thread`, each with a home skill named for the noun. A new capability
   finds its noun first. `internal/plugin` tests the skills against
   `hooks.SkillMap`, their links, and every name the code and the docs use;
   a rename that misses one fails `make test`.

## Three layers, one backend

`view` is the whole atlas as one screen, for seeing and launching only; every
command is one thing; the atlas tools (`atlas`, `project`, `settings`, `stage`,
and the thread tools) are the same things from a Claude Code session. The rule:
an action lives once, as a function in `internal/manage`, `internal/refresh`,
`internal/project`, `internal/threads`, `internal/txn`, or `internal/capture`.
The CLI exposes it as one subcommand. The TUI and the tools reach it through
`actions.Atlas`, which `actions.Bind` builds in one place. The TUI and the tools
are subsets of the CLI, never the reverse: a new key or a new tool gets a
command in the same change, and `docs/usage.md` carries the table that maps
them.

## Layout

```
.claude-plugin/         plugin.json and marketplace.json; this repo is its own marketplace
.mcp.json               the atlas MCP server: scripts/atlas mcp
scripts/atlas           sh wrapper that finds the installed binary
hooks/hooks.json        SessionStart context, PreToolUse guard, PostToolUse touch, Stop warning
skills/                 one directory per skill, named <noun>-<verb>; references live under the home of their half
agents/                 read-only workers: wiki-ingest (a source or a range of one), wiki-review (a section), thread-review (a diff against a spec)
cmd/atlas-obsidian/       main
internal/cli/           argument parsing and one method per subcommand
internal/actions/       every atlas action as one struct of functions, and Bind, the one place it is built
internal/wizard/        the setup flow
internal/project/       the one entity: atlas/<name>/project.json, the layout of both halves, templates/, Init, Open, Locate, FindAbove, Save, UpdateConfig, the engine's git scope, mode routing, page skeletons
internal/place/         where a session is: the project, from anywhere inside the work, and the config heal
internal/gitx/          the git commands the core needs
internal/txn/           plans, preview, apply, recovery, undo, history
internal/threads/       threads over plain files: cards, stage documents, phases, Sync (the generated cards, callouts, and board)
internal/describe/      the page in the wiki that describes the work, how far the work moved since, and the snapshot a page cites (with the work's TODO and FIXME lines); reads only, capture writes the snapshot
internal/capture/       inbox listing with a hint per file, staging into inbox/ (files, and the work's snapshot), capture into .raw/captured/, and a source's measure (PDF pages, lines, outline)
internal/ledger/        the source ledger
internal/lint/          the health check (and its signals: stubs, wanted pages, uncited pages), and the link resolver the mirror shares
internal/mirror/        members: the closure of a project's members, the transform of both halves, sync (the wiki as one operation, the threads as files), FindThread, and the redirect of a page that moved into the hub
internal/overlap/       what a hub's origins hold in common: the pages scored by name, content, and links; the shared names and tags; the report atlas-merge and wiki-review read
internal/mcpserver/     the tools, thin over the packages above
internal/plugin/        tests only: the plugin's skills, agents, links, and every skill name the code and docs use
internal/hooks/         session-start (the project, its wiki, the page that describes the work, the open threads, the inbox, and hot.md), guard, touched, stop
internal/claudecode/    Claude Code's plugin registry, `claude plugin`, launching claude in the work, and the first message for each launch intent (`Intent`, `Prompt`)
internal/registry/      the scan of the projects the config lists, the entries, the registry state file
internal/refresh/       derive one entry's state, rewrite the registry, list an entry's signals
internal/manage/        init, edit, forget, and register a project
internal/links/         the facts git reports about a folder, and CleanName
internal/tui/           Bubble Tea screens: the view (a force-directed map of the projects and their member links on a braille canvas, an overview of the clusters and each cluster opened onto the screen, the lenses (wiki, threads, version control) that color it, size a disk under each node, and pick the card, the selection's summary, find, settings, launch keys)
internal/terminal/      open a terminal window at a folder
internal/ide/           open the project root in the preferred IDE (VS Code for now)
internal/obsidian/      Obsidian's vault registry, obsidian:// URIs, restart
internal/home/          ~/.atlas-obsidian and config.json
internal/console/       prompts and step lines
```

`~/.atlas-obsidian/` holds `config.json` and `state/registry.json`. The projects
are the user's and live wherever the user puts them. The atlas has no default
location and never searches the disk: the config lists every project's work
folder under `projects` (`manage.Init`, `manage.Forget`) and nothing else.
A project never goes inside another (`project.CheckNew`). A session heals its own entry by id
when its folder moved or the config does not list it
(`manage.RegisterProject`).

## The rename

5.0.0 renamed everything a user sees: the binary, the module path, the plugin
(`atlas-obsidian@nathanaday-atlas-obsidian`), the marketplace, the skills
(`/atlas-obsidian:wiki`), the tools (`mcp__plugin_atlas-obsidian_atlas__*`),
the home (`~/.atlas-obsidian`), and the environment variables
(`ATLAS_OBSIDIAN_HOME`, `_BIN`, `_PROJECT`, `_SESSION*`, `_VAULT*`). The folder
inside the work stays `atlas/<name>/`: it names a place in the user's
repository, not the tool.

5.1.0 removed every path back to an earlier version. Nothing reads a
claude-atlas or claude-obsidian name any more, and the schemas restarted at
`.v1`. See "Versions" below.

## Constraints

- Dependencies: `gopkg.in/yaml.v3`, the official MCP `go-sdk` pinned to v1.4.0
  (later versions need Go 1.25; the machine runs 1.24), and Bubble Tea, Bubbles,
  and Lip Gloss. Nothing else. Do not pull `golang.org/x/*` at `@latest`.
- The atlas tools call `actions.Bind` once per call over a config loaded for
  that call, and never use `plan`/`apply`: nothing they do deletes user data,
  and the skill's one-line statement is the preview. `atlas` without
  `refresh` writes nothing; every write tool scans afresh to resolve names.
- `git` is a runtime requirement. `python3` is not. macOS and Linux only.
- The binary and the plugin are installed separately. `scripts/atlas` finds the
  binary on PATH, in `~/go/bin`, in the Homebrew prefixes, or at
  `$ATLAS_OBSIDIAN_BIN`. `plugin.json` and `marketplace.json` carry the version
  the binary should match; `status` and `doctor` warn on a mismatch.
- Every write into the wiki goes through `txn.Prepare` and `txn.Apply`.
  `project.Init`, `project.Refresh`, and `project.UpdateConfig` are the only
  code that writes the project's own files directly, and only before or outside
  an operation. The template includes the CSS snippet
  and an appearance file that enables it; a later `Refresh` rewrites the
  snippet and merges the settings, so a user who turned it off keeps it off.
- A project's identity is `atlas/<name>/project.json` (`project.Config`:
  schema, id, name, description, mode, created). There is no `kind` and no
  `knowledge`; `scope` folded into `description`. `project.Init` writes the
  folder, the template, and one `setup` commit; `project.Save` and
  `project.UpdateConfig` are the only other writers. The project folder is
  `links.CleanName` of the project's name; `Save` moves it when the name
  changes, and a taken folder refuses the save. `project.Locate` finds the
  folder as the one child of `atlas/` that holds `project.json`, and refuses
  two. `Open` refuses any schema but `project.Schema`. `project.Init` also runs `git init` in a work folder that is in no
  repository (on `main`), unless the caller asks for none, because the wiki
  needs a history. A session heals the config (`manage.RegisterProject`): an
  unknown project is added, one whose id sits at another path is moved, and one
  listed at a path that is gone is taken for the moved one only when it is the
  only one gone.
- The threads have no engine. `threads` reads and writes plain files.
  `Load` reads the cards and the documents and derives each thread's stage;
  a page it cannot use is a `Problem`, never an error. `Start`, `File`,
  `Set`, `Reopen`, `Touch`, and the phase functions change pages, and every
  one ends in `Sync`, which rewrites each card's `stage`, `outcome`, body,
  and folder, the first callout of each document (`replaceLead`, which
  replaces only a callout of a stage's own type), and `threads/threads.md`.
  `Sync` writes a file only when its content differs and never changes
  `updated`; the session-start hook runs it. `setField` rewrites one
  frontmatter line and keeps every other line, so a hand-added property
  survives. A document names its thread by id, never by file name. A
  thread's `phase` must name a phase page; `Set` refuses an unknown one,
  `RemovePhase` refuses while a thread names it, `RenamePhase` rewrites
  every card that does. There is no ledger; `updated` on the card is the
  last touch, `Stale` reads it, and the `touched` hook sets it when a
  document is edited.
- The template writes `.obsidian/snippets/atlas-obsidian.css` (the wiki's folder
  colors, the stage callouts and folder colors) and `appearance.json` only when
  there is none. `EnsureFolders` rebuilds the folders a clone left out.
- In the atlas, "thread" means a project's thread only. The bullets under
  `## Active Threads` in `wiki/hot.md` are shown as "Hot topics"
  (`refresh.HotTopics`).
- A project may list `members` in `project.json`, other projects by id, and
  `mirror.Sync` copies the transitive closure of their wikis, flat and each
  once, into `wiki/projects/<folder>/` as one `sync` operation
  (`docs/members-design.md`). The closure excludes the project itself,
  `mirror.Validate` refuses a cycle or an unknown id when the list is edited,
  and `project.MaxMembers` bounds both the list and the closure. A member
  never knows; nothing writes into one. The same sync mirrors each member's
  threads under `threads/projects/<folder>/` as plain files when both track
  threads; `mirror.FindThread` finds the project that owns a thread named on
  a hub, and the `thread` tool and command make the change there, then
  `mirror.SyncThreads`. There is no reverse sync. Sync reads the member's working tree
  and records its newest engine commit as a fact. The transform stamps
  `project`, `mirror_of`, and `commit` into each page's frontmatter and
  rewrites every resolvable link to a full vault path through `lint.Vault`,
  so the mirror and lint agree on what a link means. Lint checks a mirrored
  page's links and makes no other finding about it; a bare name resolves to
  the pages nearest the link (`preferNear`). The session-start hook syncs a
  project with members; `refresh` does not, because refresh writes nothing
  git tracks.
- A merge writes a member only as the member's own operation: `plan` takes
  `project`, and the `merge` kind writes under `wiki/` like a save. The pointer
  it leaves (`moved_to`, `moved_to_project`) is the one member page a hub
  reads differently: `mirror.movedInto` skips it and sends every link to it to
  the hub's page. `overlap` compares the hub's own pages and its mirrors as
  they sit on disk, never the members' working trees, so it needs no config.
- A kind bounds a plan's writes (`txn.allowed`), and every model kind writes
  only under `wiki/`, and never under `wiki/projects/`, which only `sync`
  writes. Reserved: `wiki/log.md`, the source ledger, `project.json`,
  `ideas/`, `threads/`, `.git`, `.vault-meta`, `.obsidian`, `.raw` except
  through capture, `inbox` except deletes in an ingest.
- A new project lints clean, and `lint.TestNewProjectHasNoFindings` holds the
  template to that. Lint runs over the project's folder but reads only `wiki/`
  as pages; every other file is a link target, and a bare name resolves to a
  wiki page first (`preferWiki`), because a thread's documents repeat one file
  name by design. A folder's index page takes the folder's name
  (`wiki/canvases/canvases.md`), so no page shares the basename of
  `wiki/index.md`.
- The wiki commits into the git repository that holds the work
  (`p.Repo()` over `gitx.At`, scoped to the project's folder), and an
  operation runs through `p.Engine()`, the same repository narrowed to
  `project.EngineScope`. `p.Work()` is the whole work, which only what reports
  on the work reads. `gitx.Repo.Scope` carries the narrowing; `AddAll` leaves
  out a scoped path that names nothing, because a project holds folders an
  operation may never write.
- `undo` is exact and never uses git's revert, which needs a clean tree the
  user's code rarely has: it reads the paths the operation changed, refuses when
  one of them changed since (`gitx.Unchanged`), and restores each from the
  commit's parent (`gitx.RestoreFrom`).
- The server and the hooks resolve the session through `place.Resolve`: an
  explicit path, `ATLAS_OBSIDIAN_PROJECT`, then the nearest
  `atlas/<name>/project.json` at or above the working directory.
- The scan is the truth. `registry.Scan` reads `atlas/<name>/project.json`
  under every path in `config.projects`, and nothing else. An entry the atlas
  knows but cannot read becomes an entry with a `Path`, an `Error`, and a
  `Reason` code (`ReasonUnreadable`, `ReasonSchema`, `ReasonMissing`,
  `ReasonNotProject`); `list` and `doctor`
  decide on the code, and the view files it under `problems`. Every command
  that acts on an entry scans afresh; `registry.json` is for display only.
- The page that describes the work is an entity page in the project's own wiki
  with `entity_type: project` and a `project` property holding the project's id
  or name (`describe.Page`); its `commit` is what `Behind` counts from when the
  work is a repository, and `describe.AtlasDirs` keeps the project's own folder
  out of that count and out of the snapshot.
- Lint is read-only; refresh writes nothing git tracks.
- TUI models keep all logic in `Update`; tests drive them with `tea.KeyMsg`.
  The map's simulation (`tui.graph`) is deterministic for a set of projects,
  steps on `tea.Tick` only while it has energy, and the canvas (`tui.canvas`)
  renders exactly the screen's size, so a test can assert on both.
- Tests never touch a real `~/.atlas-obsidian`, never install a plugin, and skip
  when `git` is missing. MCP tools are tested in-process over the SDK's
  in-memory transport.
- Prose follows the user's global writing guide.

## Claude Code plugin facts, verified on 2.1.270

Codex uses `.codex-plugin/plugin.json` and `.agents/plugins/marketplace.json`.
Both hosts share `.mcp.json`, `skills/`, and `hooks/hooks.json`; Codex supplies
`CLAUDE_PLUGIN_ROOT` for compatibility. Its `apply_patch` hook input contains
patch text in `tool_input.command`. The guard checks all patch paths and both
sides of moves. Codex users must review and trust hooks in `/hooks`, then
restart the session. Setup selects the host with `--agent claude|codex`.
Keep both plugin manifests and the Claude marketplace version in step.
`open-codex NAME [--thread ID]` launches Codex in a registered project's work
folder; `doctor --agent codex` checks its plugin via the Codex CLI. The terminal
view's `c` shortcut and `open-agent` use the global `preferred_harness`
(`claude` by default, or `codex`), editable in the view's settings panel or with
`config preferred-harness`. Automatic launch offers remain Claude-specific.

- A plugin's `.mcp.json` may run `${CLAUDE_PLUGIN_ROOT}/...`. The server starts
  in the project directory with `CLAUDE_PROJECT_DIR` and `CLAUDE_PLUGIN_ROOT`
  set, one process per session. Tools are named
  `mcp__plugin_<plugin>_<server>__<tool>`.
- A PreToolUse hook that prints `hookSpecificOutput.permissionDecision: deny`
  blocks the tool; the model sees `permissionDecisionReason`.
- SessionStart hook stdout becomes context. Stop hooks report through
  `systemMessage`.
- Plugin agents may list MCP tools in `tools:`.
- The repository's `main` branch is a marketplace named
  `nathanaday-atlas-obsidian`; a local checkout works as a marketplace source
  for development (`atlas-obsidian setup --plugin-source /path/to/checkout`).
- A session started inside a checkout of this repository reports that a
  project MCP server `${CLAUDE_PLUGIN_ROOT}/scripts/atlas` failed to start:
  Claude Code reads the checkout's own `.mcp.json` as a project server, and
  that variable is set only for plugins. The installed plugin's copy works;
  the message is noise.

## Obsidian facts

- `obsidian://open?path=` only opens vaults Obsidian already knows. Obsidian
  reads its registry (`obsidian.json` under its config dir) once at launch,
  prunes entries whose path is gone, and rewrites the file whenever its state
  changes. `open-vault` quits Obsidian first (macOS, AppleScript), adds one
  entry, relaunches, then opens the URI. Verified on Obsidian 1.8.7 / 1.13.7.

## Build and test

```
make build      # build/atlas-obsidian
make install    # go install into $(go env GOPATH)/bin
make test
```

The installed plugin is a git clone of this repository at a commit, and
`claude plugin update` fetches a new one only when the version in
`.claude-plugin/plugin.json` and `marketplace.json` went up. So a skill change
reaches Claude Code after: bump both versions, commit, then

```
claude plugin marketplace update nathanaday-atlas-obsidian
claude plugin update atlas-obsidian@nathanaday-atlas-obsidian
make install
```

The marketplace clone under `~/.claude/plugins/marketplaces/` fetches from
GitHub, not from the checkout the config names, so the update sees a new
version only after `main` is pushed. `make install` stamps the binary with the
same version, so `doctor` and `status` can tell when the two drift. Uncommitted skill edits can be tried
with `claude --plugin-dir .` from inside a project. End-to-end by hand:
`claude -p "..."` inside a project with
`--allowedTools "mcp__plugin_atlas-obsidian_atlas__*,Read,Grep,Glob,Skill"`.

## Versions

5.1.0 is the clean start. It removed the migration commands, every schema an
earlier version wrote, and every folder and file name those versions used. The
schemas restarted at `.v1`:

| File | Schema |
|---|---|
| `atlas/<name>/project.json` | `atlas-obsidian.project.v1` |
| `~/.atlas-obsidian/config.json` | `atlas-obsidian.config.v1` |
| `~/.atlas-obsidian/state/registry.json` | `atlas-obsidian.registry.v1` |
| the source ledger | `atlas-obsidian.source-ledger.v1` |

Every one of these is read exactly, and an unknown schema is refused by name.
Nothing converts a file; `upgrade`, `manage.PlanUpgrade`, `threads.Migrate`,
`project.Absorb`, `project.MoveFlat`, and `home.adopt` are gone, and so are the
lint findings and registry reasons that named an earlier layout
(`kind_errors`, `ReasonFlat`, `ReasonV3Split`).

The binary and the plugin keep the 5.x numbering so `claude plugin update`
still sees a version going up. The design record of how the layout arrived
here is in `docs/v4-design.md`; the designs it replaced are history, listed
under "Sources of truth".

## Open questions

- Extend `preferred_ide` and `open-ide` beyond VS Code: Cursor, Windsurf, Zed,
  JetBrains IDEs, and Sublime Text. The TUI `i` shortcut opens the work root;
  the settings panel and `config preferred-ide vscode` persist the preference.

- A `search` tool with BM25 ranking, once Grep proves insufficient.
- Distribution: a Homebrew tap and release binaries; then the wrapper can
  download a checksummed binary into `${CLAUDE_PLUGIN_DATA}`.
