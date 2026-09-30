# Migration

7.0 reads only the flat layout (`layout: 3` in `Atlas.md`). `atlas-obsidian vault migrate` moves a 6.x vault to it in one commit. The migration is code, so it follows fixed rules; this page lists them.

## Running it

```bash
atlas-obsidian vault migrate --dry-run     # prints the plan; writes nothing
atlas-obsidian vault migrate               # writes the plan as one commit
```

- On a vault with `layout` below 3, the session-start hook prints one line that names the command, and every write tool refuses with the same line. Read tools work, so an agent can still answer questions.
- The Obsidian plugin shows a notice with a button that runs the dry run, shows its report, and runs the migration on a second click.
- The migration runs from the CLI only. The guard refuses it from an agent's shell, as it refuses `change apply`, because it rewrites the whole vault. You type it, or `!` it in a session.

## Before it writes

The migration refuses, and names what to do, when:

- a change is `proposed` or `applying`: apply or reject it first, since its writes use the 6.x schemas;
- two documents would take one title after the rules below.

A live session (`running` or `waiting`) does not stop it. The report warns about each one, since its threads and tasks become the work and specs of the new layout.

Then it takes the lock, recovers, and commits a dirty tree as a snapshot, as every write does.

## The rules

### Tags from the scope tree

Each area and repository gets a tag: the names on its chain from the top, normalized ([[Documents#Form]]), joined by `/`. The area `Machine Learning` under the vault is `machine-learning`; the repository `p3-edge` under `work` › `p3` is `work/p3/p3-edge`. A document scoped to the vault gets no tag.

### Knowledge

| 6.x | 7.0 |
|---|---|
| area | topic, `kind: overview`, `defines` its tag, `tags` [its parent's tag]; the body becomes `## Context`, the description opens `## Summary`; `## Map` is added |
| repository | repository, `defines` its tag, `tags` [its parent's tag]; the body is kept; the live status block, `## Work`, and `## Knowledge` are added |
| concept, entity, policy | topic with that `kind`; `scope` becomes `tags` [the scope's tag]; an entity's `kind` (`tool`, `person`, …) becomes a second tag; a policy keeps `strength` |
| source | source; `scope` becomes a tag; `file` points into `wiki/assets/`; `media` and `status` are derived |

### Threads

| 6.x | 7.0 |
|---|---|
| a thread at stage `stub` | a stub; `## Stub` becomes `## Idea`; `priority` stays; its scopes become `tags` |
| a thread past `stub` | the stub becomes the root plan, in place (id, title, and links kept); the spec document's sections move into it; `## Stub` becomes `## Origin`; its repository scopes become `repositories`, and the tag of every scope, area or repository, goes into `tags` |
| the spec document | removed; every link to `<Thread> — Spec` is rewritten to `<Thread>` |
| a task | a leaf plan, `parent` the root; `repository` becomes `repositories`; `depends` and `order` stay; `## What` becomes `## Goal`; `## Where`, `## Verify`, `## Conventions`, `## Progress` stay |
| a task title `<Thread> — T2 Tune the threshold` | `Tune the threshold`, when no other document holds it; else the old title stays |
| the receipt | a `completed` event on the root (its four sections), or `dropped` for `killed` (`## Why killed` becomes `## Why`) |
| a superseded receipt | its event, then a `reopened` event at the date in its title |
| the thread's `blocked` | a `blocked` event on the root |

The events carry the times the 6.x documents give:

- a done task: `completed` at its `updated`, with `## Result` as `## Delivered`;
- a dropped task: `dropped` at its `updated`, with `## Why`: "Dropped before 7.0";
- a task with a `## Progress` line: `started` at the date of its first line;
- a root with a started, done, or dropped task: `started` at the earliest of them;
- a receipt: its event at its `created`.

Code-owned fields of 6.x that 7.0 derives in other ways go: `stage`, `outcome`, `active`, `tasks`, `chain`, `scope`, `parent`, `thread`, `thread_id`.

### Ids

Every document keeps its id, since an id never changes. A migrated concept keeps `con-k3m9qa`, and a task keeps `tsk-9d4mzt`. Code reads any id of three letters, a hyphen, and six base32 characters; only a new document gets `doc-` ([[Documents#The common fields]]).

### Records

- Sessions: `threads` becomes `work`, `tasks` becomes `specs`, with the links rewritten.
- Changes: `thread` becomes `work`. `## Writes` is left alone: it records what was true then.
- One change document, `Migrate to 7.0`, with `status: applied`, absorbs every document whose 6.x version an applied change had absorbed, at its new hash. So a spec or a receipt that the wiki had learned is not pending again only because its body moved.

### Files

- Every typed document moves to `wiki/documents/<title>.md`.
- `wiki/sources/files/*` moves to `wiki/assets/`. Embeds name the file alone, so they still resolve.
- Other files inside `wiki/` and `threads/`: an image or an attachment moves to `wiki/assets/` (with ` (2)` on a clash); a markdown note with no type moves to `inbox/from 6.x/`, keeping its path below the old folder, so [[wiki-ingest]] offers to type it. Links to it still resolve by title.
- `threads/Threads.canvas`, and `Threads.base` or `Wiki.base` when you edited one, move to `scratchpad/from 6.x/`. Unedited Bases are removed.
- The folders `wiki/<scope>/…` and `threads/` are removed once empty.

### Settings

- `.obsidian/plugins/atlas/`: the vault gets the Obsidian plugin that the binary carries, since a 6.x plugin cannot read the new layout. Reload Obsidian after the migration.
- `Atlas.md`: `areas: manual` becomes `tagging: known`; `few` and `many` become `tagging: open`. In `wikify`, `receipt` becomes `event`, and `task` goes. `layout: 3`.
- `.obsidian/app.json`: the attachment folder becomes `wiki/assets`; `views/` is added to the excluded files.
- `.git/info/exclude`: `views/` is added.

## After it writes

1. Sync writes the views, the statuses from the new events, and the lead callouts.
2. One commit, `layout: migrate to 7.0`, holds every move and rewrite, with a trailer `Atlas-Migrate: 6.5 → 7.0`.
3. Lint runs, and the report ends with its findings.

To go back, `git revert` the commit and install 6.5.
