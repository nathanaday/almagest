
# Obsidian Plugin

The Obsidian plugin makes the vault read like one product: colors and icons for each document type, a sessions sidebar, and buttons that approve a change or refresh the vault. It is thin. It reads the documents through Obsidian's own metadata cache, and it calls the `atlas` binary for every write. It holds no rule that the binary does not hold.

The vault works without the plugin. The Bases render the board, the sessions, the changes, and the wiki in Obsidian's core, and the callouts render in Obsidian's default style. The plugin adds what core Obsidian cannot.

This page supersedes [[V2 Obsidian Plugin Brainstorm]] where the two differ. The largest difference: agent status needs no separate state folder, because the session documents are the status.

## Constraints

- **Desktop only** (`isDesktopOnly: true`). The plugin runs the binary through `child_process`.
- **Finding the binary.** Obsidian started from the macOS Dock does not get the shell's `PATH`. The plugin searches the same places as the agent plugin's wrapper (`~/.atlas/bin`, the npm global prefix, the Homebrew prefixes), and a setting overrides it.
- **One version.** The binary, the agent plugin, and the Obsidian plugin share one version. `vault init` installs the Obsidian plugin into the vault, and `atlas doctor` reports a mismatch.
- **Distribution.** A local install from `vault init` first; BRAT next; the community store last, because the store reviews plugins that spawn processes more closely.

## Phase 1

### Styles

One stylesheet in the plugin, in place of V1's CSS snippet:

- a callout color and icon for `stub`, `spec`, `task`, `receipt`, `killed`, `session`, and `change`;
- a color for each top folder (`inbox`, `scratchpad`, `sessions`, `threads`, `changes`, `wiki`) and each wiki folder;
- a badge in the file explorer for a stub's stage and a session's status.

### Refresh

A ribbon icon and a command: `atlas vault sync`, then a notice with what changed. Sync is safe at any time.

### Review a change

On a change document with `status: proposed`, a bar over the page with **Apply**, **Reject**, and the counts. Apply saves the open file first, so an edit you just typed goes in, then runs `atlas change apply <id>`; Reject asks for a reason and runs `atlas change reject`. A click on Apply is the user's yes, so a large ingest can be reviewed and approved entirely in Obsidian. The agent's next turn sees the change applied.

## Phase 2

### Mentions

A mention is a task line addressed to the agent:

```markdown
- [ ] @atlas add these papers to the wiki
- [ ] @atlas the login timeout is too short on p3-cloud
```

- The plugin highlights `@atlas` in the editor (a CodeMirror 6 decoration) and in reading view (a post-processor).
- The binary finds every open mention outside `wiki/`, `changes/`, and `sessions/`. [[Vault Status]] lists them, and the session-start context counts them.
- The [[atlas]] skill offers them. Whatever answers a mention (a new stub, a change, a session), the skill closes it with `vault` `action: mention` and a link to the answer. One action closes every mention.
- Closing a mention checks its box and appends ` → [[link]]`. That edits one line of your note. It is the one write code makes into a document you own, and it is the answer you asked for.


### Sessions sidebar

A pane that lists the sessions with status `running` or `waiting`, from the session documents: the description, the thread and task, the last progress line, and the time since the last hook event. A waiting session shows first, with a badge on the ribbon icon. A click opens the session document.

A button resumes a session in a terminal (`claude --resume <session id>` in the session's `cwd`). Chat inside Obsidian is not planned: the harness has no public API for a live session.

### Sync on change

When a thread document changes, the plugin runs `atlas vault sync` after a short delay, so the board and the callouts follow a hand edit at once. Sync writes a file only when its derived content differs, so its own writes end the loop.

### Graph colors

The plugin colors the nodes of Obsidian's graph in one of four modes. Buttons over the graph view select the mode, and a legend under them names each color and counts its nodes. A command for each mode and a setting do the same. The default mode is Area.

| Mode | Groups |
|---|---|
| Area | One color for each area. A document takes its nearest area: an area is its own; a repository or a knowledge page takes the last area in its `chain`; a stub takes the area of its first scope; a spec, task, receipt, or change takes its thread's area; a session takes the area of its first thread or repository. The eight oldest areas (by `created`) get the eight colors, so a new area never repaints the others. The areas after the eighth share one gray group. |
| Type | Areas, repositories, concepts, entities, policies, sources, threads (stub, spec, task, receipt), and sessions with changes. |
| Threads | Open threads, closed threads, and no threads (gray). A thread document takes its own thread's state. Another document takes the state of each thread that it belongs to (a session's or a change's) or shares a link with, in either direction. An open thread wins over a closed one. A thread is closed when its stub has `stage: closed`. |
| Activity | Four quarters of the markdown files, newest first. The sort key is the day of `updated`, then the file's modification time. A file with no `updated` uses the day of its modification time. |

The colors come from one categorical palette in a fixed order, with light and dark steps. Activity uses one blue ramp, where the newest quarter has the most contrast with the background. The plugin uses the steps of the current theme and applies them again when the theme changes.

How the groups reach the graph:

- **One query per group lists its paths.** The query is a regular expression that matches whole paths: `path:/^(?:wiki\/concepts\/Idea\.md|…)$/`. A quoted `path:"Notes.md"` would also match `inbox/Notes.md`. A search on a property cannot follow a thread to its stub or a scope to its area, so the plugin computes each group from the metadata cache.
- **The groups follow the vault.** The plugin computes the groups again one second after the metadata cache resolves a change, after a rename or a delete, after a layout change, and after a theme change. It writes them only when they differ.
- **Atlas owns the queries of that one form.** On each write the plugin removes every group whose query starts with `path:/^(?:` and ends with `)$/`, puts its own groups first, and keeps the user's groups after them. Obsidian colors a node with the first group that matches, so the user's groups color only the nodes that Atlas's groups leave out. The plugin stores no list of its queries. Off, or a disabled plugin, removes Atlas's groups and keeps the user's groups.
- **The plugin writes to the graph's options and to every open graph.** It sets `colorGroups` in the core graph plugin's options and saves them, and it passes the groups to the engine of each open global and local graph. These are not public API. Each access is checked, so a change in Obsidian turns the colors off and breaks nothing else.
- **`.obsidian/graph.json` stays out of git.** Obsidian rewrites it on every zoom, and the plugin rewrites it when a document changes, so a tracked file would add a snapshot commit to most writes. `EnsureFolders` excludes it. A vault that tracked it before gets one commit, `untrack machine files: .obsidian/graph.json`, and the file stays on disk.

## Later

- **Live canvas edits.** An agent's write to a `.canvas` file reloads the open canvas, and a user's edit goes to the same file. A race exists while Obsidian delays its save. Build a node-by-node merge only if the race matters in practice, because the canvas view's API is undocumented.
- **The order of threads on a canvas.** Stubs as file nodes, edges as "after". Works today with no code; the plugin could draw the order onto the board later.
- **Graph presets.** Buttons that open the graph filtered to the wiki, the threads, or the context graph.
