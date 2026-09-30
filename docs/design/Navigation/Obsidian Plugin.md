# Obsidian Plugin

The Obsidian plugin is the navigation layer's live half. It renders what core Obsidian cannot, keeps the views fresh while you edit, and gives buttons for the writes that are yours to make. It is thin. It reads the documents through Obsidian's metadata cache, and it calls the `atlas-obsidian` binary for every write. It holds no rule that the binary does not hold.

The vault works without the plugin. The views are markdown with inline Bases, the callouts render in Obsidian's default style, and the binary writes the views at every tool call and session start. The plugin adds colors, the live panes, and freshness between agent sessions.

Each feature below says whether 6.5 has it (kept), has it in another form (changed), or not (new in 7.0).

## Constraints

- **Desktop only** (`isDesktopOnly: true`). The plugin runs the binary through `child_process`.
- **Finding the binary.** The plugin never searches `PATH` or the system folders, so another program's binary never runs in its place. It looks at `~/.atlas/bin/atlas-obsidian`, then `~/go/bin/atlas-obsidian`, and a setting overrides both.
- **One version.** The binary, the agent plugin, and the Obsidian plugin share one version. `vault init` installs the Obsidian plugin into the vault, and `atlas-obsidian doctor` reports a mismatch.
- **Private API.** Where the plugin uses an API that is not public (the graph's color groups, the file explorer's items), each access is checked, so a change in Obsidian turns that feature off and breaks nothing else.

## Styles

*Changed from 6.5.*

One stylesheet:

- a callout color and icon for each lead type: `source`, `repository`, `repository-missing`, `concept`, `entity`, `policy`, `overview`, `stub`, `stub-resolved`, `stub-dropped`, `spec`, `spec-done`, `spec-dropped`, `design`, `design-superseded`, `event-<kind>` for each event kind, `session`, `change`, `atlas`, `view`, and `tag`;
- a badge in the file explorer for the status of a stub or a plan, and an icon for each event kind, so the flat `wiki/documents/` reads at a glance;
- a color for each top folder: `inbox`, `scratchpad`, `wiki`, `sessions`, `changes`, `views`.

The type folders and scope folders of 6.5 go.

## The views folder

*Changed from 6.5.*

The 6.5 scope folders become the views' tag tree ([[Views#Tag views]]). In the file explorer:

- a folder under `views/tags/` has a bold name, and a click on it opens the tag view inside it; a click on the arrow only opens or closes the folder;
- the tag view is hidden inside its folder, since the folder stands for it;
- `views/` sorts first among the top folders.

The plugin never renames or moves a view: the binary writes them. The 6.5 rename sync between a scope folder and its page goes. A setting turns the click and the hidden note off.

## Tag navigator

*New in 7.0.*

A pane in the left sidebar that narrows the documents by tags, to any depth, with no file written.

```text
Tags ▸ school/cs513 ▸ self-driving                         [×]
12 documents
  With: project 5 · paper 4 · ml/vision 3 · …
  Open work (2)   Filter pedestrians by depth · spec · started
                  Try a smaller backbone · stub
  Topics (6)      Occupancy grids · concept …
  Sources (4)     …
  Events (9)      the last five …
```

- The top of the pane is the path of chosen tags. The first choice lists every top-level tag with its count, and its children under a fold.
- Each step lists the tags that occur with the chosen ones and their counts. A click adds one. The `×` on a chosen tag removes it.
- Below, the documents that hold every chosen tag, grouped by type (open work first), each with its description on hover. A click opens it.
- A button opens the chosen tags in Obsidian's search, and a button opens the first tag's view.
- The pane reads the metadata cache, and follows it live. It counts a document for a tag when it holds the tag or a tag below it ([[Documents#What a tag reaches]]).
- A setting makes a click on a `#tag` in a document open the navigator at that tag, in place of Obsidian's search.

## Repository status

*New in 7.0.*

The plugin renders the `atlas-repo` code block of a repository document ([[Repository#Body]]) as a panel:

- the branch, and a note when it is not the document's `branch`;
- the files changed and not committed, as a count and a list under a fold;
- the commits ahead of and behind the remote's branch, from the last fetch (the plugin never fetches);
- the last three commits;
- a Refresh button.

It reads the facts from `atlas-obsidian context <id> --json` when the document opens, and on the button. It never writes the document. A path that is gone shows the reason.

## Refresh

*Kept from 6.5.*

A ribbon icon and a command: `atlas-obsidian vault sync`, then a notice with what changed. Sync is safe at any time.

## Sync on change

*Changed from 6.5.*

When a markdown file changes in Obsidian (a hand edit, a rename, a move, a delete, a checked box), the plugin runs `atlas-obsidian vault sync --views` two seconds after the metadata cache settles, at most one run at a time. The views, the statuses, and the lead callouts then follow a hand edit at once. Sync writes a file only when its derived content differs, so its own writes end the loop.

In 6.5 this ran on a thread document's change. In 7.0 it runs on any change outside `views/`, because the views read every document.

## Review a change

*Kept from 6.5.*

On a change document with `status: proposed`, a bar over the page with **Apply**, **Reject**, and the counts. Apply saves the open file first, so an edit you just typed goes in, then runs `atlas-obsidian change apply <id>`; Reject asks for a reason and runs `atlas-obsidian change reject`. A click on Apply is your yes, so a large ingest can be reviewed and approved entirely in Obsidian.

## Work buttons

*New in 7.0.*

On a stub or a plan, a small bar with the moves that are yours to make without an agent: **Drop** (asks for a reason), **Reopen**, **Block** (asks for the line), and **Unblock**. Each runs the `work` CLI command, which writes the event with `by: user` and no session. Starting and finishing work stay with agents, because `work done` needs the result.

## Migration notice

*New in 7.0.*

On a vault with `layout` below 3, a notice with a button that runs `atlas-obsidian vault migrate --dry-run`, shows the report in a modal, and runs the migration on a second click ([[Migration#Running it]]).

## Mentions

*Kept from 6.5.*

A mention is a task line addressed to the agent:

```markdown
- [ ] @atlas add these papers to the wiki
- [ ] @atlas the login timeout is too short on p3-cloud
```

- The plugin highlights `@atlas` in the editor (a CodeMirror 6 decoration) and in reading view (a post-processor).
- The binary finds every open mention outside `wiki/documents/`, `changes/`, `sessions/`, and `views/`. [[Vault Status]] lists them, the session-start context counts them, and `View · Work` lists them.
- The [[atlas]] skill offers them. Whatever answers a mention (a new stub, a change, a session), the skill closes it with `vault` `action: mention` and a link to the answer.
- Closing a mention checks its box and appends ` → [[link]]`. That edits one line of your note. It is the one write code makes into a note of yours outside a rewrite, and it is the answer you asked for.

## Sessions pane

*Kept from 6.5.*

A pane that lists the live sessions, from the session documents: the description, the plan, the last progress line, and the time since the last hook event. A waiting session shows first, with a badge on the ribbon icon. A click opens the session document.

A button resumes a session in a terminal (`claude --resume <session id>` in the session's `cwd`). Chat inside Obsidian is not planned: the harness has no public API for a live session.

## Graph colors

*Changed from 6.5.*

The plugin colors the nodes of Obsidian's graph in one of four modes. Buttons over the graph view select the mode, and a legend under them names each color and counts its nodes. A command for each mode and a setting do the same. The default mode is Tag.

| Mode | Groups |
|---|---|
| Tag | One color for each top-level tag. A document takes the top segment of its first tag; an event takes its subject's; a session or a change takes that of its first spec or first absorbed document. The eight top-level tags that some document held first (by the earliest `created`) get the eight colors, so a new tag never repaints the others. The tags after the eighth share one gray group, and a document with no tag is uncolored. |
| Type | Sources, repositories, concepts, entities, policies, overviews, stubs and specs, events, and sessions with changes. |
| Work | Open work, done work, and no work (gray). A stub or a plan takes its own state (open or started is open; done, dropped, or resolved is done). An event takes its subject's. Another document takes the state of each plan or stub it shares a link with, in either direction. Open wins over done. |
| Activity | Four quarters of the markdown files, newest first. The sort key is the day of `updated`, then the file's modification time. |

The colors come from one categorical palette in a fixed order, with light and dark steps. Activity uses one blue ramp, where the newest quarter has the most contrast with the background. The plugin uses the steps of the current theme and applies them again when the theme changes.

How the groups reach the graph (kept from 6.5, verified on Obsidian 1.13.7):

- **One query per group lists its paths**, as a regular expression that matches whole paths: `path:/^(?:wiki\/documents\/Idea\.md|…)$/`. The plugin computes each group from the metadata cache, because a search on a property cannot follow an event to its subject.
- **The groups follow the vault.** The plugin computes them again one second after the metadata cache resolves a change, after a rename or a delete, after a layout change, and after a theme change. It writes them only when they differ.
- **Atlas owns the queries of that one form.** On each write the plugin removes every group whose query starts with `path:/^(?:` and ends with `)$/`, puts its own groups first, and keeps yours after them. Obsidian colors a node with the first group that matches, so your groups color only the nodes that Atlas's groups leave out. Off, or a disabled plugin, removes Atlas's groups and keeps yours.
- **The plugin writes to the graph's options and to every open graph** (`colorGroups` in the core graph plugin's options, and `view.dataEngine.setOptions` on each open global and local graph).
- **`.obsidian/graph.json` stays out of git** ([[Vault Layout#Out of git]]).

`views/` is in Obsidian's excluded files, so view notes do not show in the graph.

## Later

- **Graph presets.** Buttons that open the graph filtered to one tag, to open work, or to one plan's tree.
- **A work canvas.** Plans as cards grouped by root tag, with edges for `depends`, written by the view writer. 6.5's `Threads.canvas` showed that users draw order by hand; a derived canvas would draw `depends` instead.
- **Live canvas edits.** An agent's write to a `.canvas` file reloads the open canvas. A race exists while Obsidian delays its save. Build a merge only if the race matters.
