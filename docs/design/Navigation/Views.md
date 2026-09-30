# Views

A view is a note that code writes from the documents, so you can find the document you want to open. `wiki/documents/` is flat and holds hundreds of files; the views give it a tree, a work board, and a timeline. A view never copies a document's content. It lists, counts, and links.

Views live in `views/`. Code writes them, git ignores them, and a view you edit is written over at the next sync. Delete the folder, and the next sync writes it again. So the presentation can change in any release with no change to a document.

## Layout

```text
views/
├── View · Home.md                    the entry: what is where, what waits for you
├── View · Work.md                    stubs, plans, to-do lines, mentions
├── View · Timeline.md                what happened, newest first, the last 30 days
├── View · Library.md                 the knowledge: topics, sources, repositories
├── View · Repositories.md            every linked repository with its live git status
├── timeline/
│   └── View · Timeline 2026-08.md    one per earlier month
└── tags/                             the tag tree: a folder per tag, a note per folder
    ├── school/
    │   ├── Tag · school.md
    │   └── cs513/
    │       └── Tag · school › cs513.md
    └── self-driving/
        └── Tag · self-driving.md
```

- A view's title begins with `View · ` or `Tag · `, and no document's title may ([[Vault Layout#Titles and file names]]). So a view never takes a link target from a document.
- Every view note opens with a callout that says it is derived: `> [!view] Written by Atlas from the documents. Edits here are lost at the next sync.`
- Views link only to documents and to other views. Documents never link to views, so a clone without `views/` has no dead link.
- `init` and the migration add `views/` to Obsidian's excluded files, so views stay out of the graph and search, and rank low in the quick switcher.

## How a view lists documents

Two ways, chosen for each section:

| Way | Is | Used for |
|---|---|---|
| an inline Base | a `base` code block with filters; Obsidian renders it live | lists that follow a field: open plans, topics by kind, sources |
| a written list | markdown that code writes at sync | what a Base cannot do: to-do lines inside bodies, the timeline across types, the tag tree's counts |

A Base filters on the document fields, and always on `file.inFolder("wiki/documents")`. A tag filter lists the tag and every tag below it by name (`file.hasTag("school/cs513", "school/cs513/hw1")`), because code knows the tree, so a view does not depend on how Obsidian matches nested tags.

## Home

`View · Home` is the entry to the vault.

1. A callout with the counts: documents by type, open stubs, started plans, pending documents, proposed changes, live sessions.
2. **Waiting for you**: proposed changes, sessions that wait for an answer, and open mentions, each linked.
3. **Tags**: the top-level tags, each with its count and a link to its tag view, the most used first.
4. **Views**: a link to each other view.
5. **Recent**: the last ten events, as in the timeline.

## Tag views

One note per tag, in a folder per tag: `views/tags/school/cs513/Tag · school › cs513.md`. The file explorer then shows the tag tree. With the Obsidian plugin, a click on a tag's folder opens its note, and the note is hidden inside its folder ([[Obsidian Plugin#The views folder]]).

A tag view holds:

1. The callout: the tag, the count of documents that hold it, the tag's page when a document defines it, the parent tag's view, and the child tags' views with their counts.
2. **Narrow**: the tags that occur with this one, each with its count, most first. Each is a link that opens Obsidian's search for both tags (`obsidian://search?vault=…&query=tag:#school/cs513 tag:#self-driving`). With the plugin, the [[Obsidian Plugin#Atlas navigator|Atlas navigator]] does this in place, to any depth.
3. **Open work**: a Base of the stubs and plans that hold the tag, open or started, by priority.
4. **Topics**: a Base grouped by kind.
5. **Sources**: a Base, pending first.
6. **Repositories**: a Base with the path, the branch, and `behind`.
7. **History**: a Base of the events that hold the tag, the last 30 days.

A section with no documents is left out. Code writes a tag view for every tag that some document holds, and for every tag above one. It removes a tag view when no document holds its tag.

## Work

`View · Work` is the board.

1. **Waiting for you**: proposed changes, and sessions with status `waiting`.
2. **Active now**: plans that a live session has started, with the session and the last progress line.
3. **Plans**: a Base of the plans, grouped by status (`started`, `open`), with ready plans first (every dependency done), then by priority. A column shows `root`, so the parts of one plan sit together.
4. **Blocked**: the plans with `blocked`, and the reason.
5. **Stubs**: a Base of the open stubs, by priority, then by `created`.
6. **To-do lines**: a written list of every open task line that holds `#todo`, in any markdown file of the vault outside `views/`, `changes/`, and `scratchpad/`. Each line gives the document, a link to it, and the line's text. Checking the box in the document removes the line at the next sync.
7. **Mentions**: a written list of every open `@atlas` line ([[Obsidian Plugin#Mentions]]).
8. **Done lately**: the `completed` and `dropped` events of the last 14 days.

## Timeline

`View · Timeline` is a written list of what happened, newest first, a heading per day:

```markdown
### 2026-09-30

- 11:02 · completed · [[Score boxes by motion]] → [[Score boxes by motion · completed 2026-09-30 1102|result]] · [[2026-09-30 1040 f3e9a1]]
- 10:40 · continued · [[Score boxes by motion]] · [[2026-09-30 1040 f3e9a1]]
- 09:12 · change applied · [[2026-09-30 Ingest the OTA paper]] · 6 create, 2 modify
- 08:55 · planted · [[Try a new tracker]] · #work/p3/p3-edge
```

It reads three things:

- every event, by `at`;
- every stub and spec, by `created`, as `planted` or `written`;
- every applied change, by `applied`, with its counts.

The note holds the last 30 days. Each earlier month has its own note under `views/timeline/`. Only the notes whose content changed are written.

## Library

`View · Library` is the knowledge, for browsing:

1. **Topics**: a Base grouped by kind, then by the first tag.
2. **Sources**: a Base with the media, the authority, and the status; pending first.
3. **Repositories**: a Base with the path, the branch, the head, and `behind`.
4. **Needs care**: Bases of the topics that are `draft` or `contested`, that have no sources, or whose `refreshed` is older than a document they cite.

## Repositories

`View · Repositories` shows every linked repository on one page, by title. Each gets a section with:

1. a heading that links the repository document, and its description;
2. its path; then, as in its tag view, its own tag (`Tag`), the tag it sits under (`Under`), and any other tags it holds (`Also`), each linked to its tag view;
3. the open and started plans that name it;
4. the `atlas-repo` block of its document, which the Obsidian plugin renders as the live status panel ([[Repository#Body]]).

The unlinked repositories follow as a list of links.

## Freshness

| When | Who | Writes |
|---|---|---|
| every `work`, `change`, and `source` write, after its commit | the tool | every view the write touched |
| `vault sync`, at session start and on the plugin's Refresh | the binary | every view |
| a document changes in Obsidian (a hand edit, a rename, a checked box) | the plugin, two seconds after the metadata cache settles | `atlas-obsidian vault sync --views`: every view |

The view writer reads every document's frontmatter and every markdown body once, builds each view in memory, and writes a file only when its content differs. It removes view files that no longer stand for anything. It holds the lock for its writes, like every writer. A hook never writes views, so hooks stay fast.

## Rules

- A view holds no fact that is not in a document. Code can write every view from the documents alone.
- A view is never read by a tool, a skill, or an agent. Lookup reads the documents.
- The model never writes `views/`. The guard refuses it.
