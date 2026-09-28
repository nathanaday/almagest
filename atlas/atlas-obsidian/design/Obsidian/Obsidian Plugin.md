
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

## Later

- **Live canvas edits.** An agent's write to a `.canvas` file reloads the open canvas, and a user's edit goes to the same file. A race exists while Obsidian delays its save. Build a node-by-node merge only if the race matters in practice, because the canvas view's API is undocumented.
- **The order of threads on a canvas.** Stubs as file nodes, edges as "after". Works today with no code; the plugin could draw the order onto the board later.
- **Graph presets.** Buttons that open the graph filtered to the wiki, the threads, or the context graph.
