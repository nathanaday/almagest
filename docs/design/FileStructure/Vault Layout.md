
# Vault Layout

The layout of a vault, the vault document, and the rules for ids, titles, links, and git.

## Layout

```text
<vault>/
├── Atlas.md                  the vault document: identity, settings, the vault's own context
├── inbox/                    you drop files and notes here; wiki-ingest triages them
├── scratchpad/               yours; no schema; no tool reads it, and no agent unless you ask
├── sessions/
│   ├── Sessions.base         what runs now, and what ran
│   └── 2026-09/              one document per agent session, by month
├── threads/
│   ├── Threads.base          the board
│   ├── Threads.canvas        the board as a canvas: a card per open thread, grouped by area
│   ├── <Thread title>/       a thread filed under no area yet: stub, spec, tasks, receipt
│   └── <Area>/…/<Thread title>/   a thread in the folder that stands for its home scope
├── changes/
│   ├── Changes.base          proposed and applied changes
│   └── 2026-09/              one document per change, by month
├── wiki/
│   ├── Wiki.base             every page by type and scope
│   ├── concepts/  entities/  policies/  sources/   knowledge pages scoped to the vault
│   ├── sources/files/        the captured originals of every scope
│   └── <Area>/<Area>.md      a scope: a folder with its page, laid out like wiki/ (see [[Wiki#Layout]])
├── .claude/settings.local.json   untracked; code lists the linked repositories here
├── .obsidian/                app settings and the Atlas plugin
└── .git/                     the vault's history; apply's lock is .git/atlas.lock
```

`atlas-obsidian vault init` writes this layout, the four Bases, and one `setup` commit. `EnsureFolders` rebuilds a folder that a clone left out, because git does not keep an empty folder.

## Who writes where

| Path | The model | Code | You |
|---|---|---|---|
| `Atlas.md` | no | `init` only | yes |
| `inbox/` | no | `source` capture removes a file in its commit | yes |
| any note of yours | no | `vault` mention and `thread` open check the box of a mention and link the answer | yes |
| `scratchpad/` | no | no; search, lint, and the mention scan skip it | yes |
| `wiki/` | only through `change` | `change` apply, `source` capture | yes |
| `threads/` | Edit on the prose of a document that exists | `thread`, the hooks | yes |
| `sessions/` | Edit of three sections of its own session document | the hooks | yes |
| `changes/` | through `change` | `change` | yes, a proposed change before you approve it |
| `*.base` | no | `init` only | yes |
| `.claude/settings.local.json` | no | `vault sync` | yes, every key but the one list code keeps |

The guard hook enforces the model's column. See [[Hooks#guard]].

## Atlas.md

The vault document is the root of the context graph. Every session reads it first.

```yaml
---
id: vlt-k3m9qa
type: vault
name: Work
description: "Work notes: the p3 product and the tools around it."   # one line; agents read it first
created: 2026-09-27
updated: 2026-09-27
areas: few                          # many | few | manual: how readily the agent proposes areas
wikify: [source, spec, receipt]     # the types that are pending until a change absorbs them
stale_hours: 12                     # a running session with no hook event for this long is lost
layout: 2                           # code's: the layout of the folders; sync moves an older vault once
---
```

The body is yours: what this vault is for, and the context that every agent in it must know. The `areas` setting comes from the onboarding question:

| Answer | `areas` | The agent |
|---|---|---|
| "A lot of areas, for the most order" | `many` | proposes an area for each cluster it sees |
| "Keep it simple" | `few` | proposes top-level areas only |
| "I don't know yet, or I'll make them" | `manual` | never proposes an area; asks when a repository has no parent |

## Titles and file names

- The file name is the title. Code writes the file name from the title the model gives, with the characters `/ \ : * ? " < > |` and a leading dot removed.
- A title is unique in the vault, across every folder, compared without case. Code refuses to create or rename a document to a title or alias that another document holds. A bare `[[Title]]` then has one target.
- Thread documents take the thread's title and a part: `<Thread> — Spec`, `<Thread> — T1 <task title>`, `<Thread> — Receipt`. The stub takes the thread's title alone.
- Session and change documents take a date and a short name: `2026-09-27 1432 a1b2c3`, `2026-09-27 Ingest the DINOv2 paper`.

## Links

- Links between documents are Obsidian wikilinks by title: `[[DINOv2]]`. A link in frontmatter is quoted: `scope: "[[p3-cloud]]"`.
- Tools take and return ids. A document states its links by title, so Obsidian's graph, backlinks, and Bases work on them.
- A rename is a write of its own (`change` rename, `thread` set title). Code rewrites every link to the old title in the same commit.

## Git

The vault is one git repository, on `main`. It never contains another repository: `atlas-obsidian vault init` refuses a folder inside another repository's work tree, and `change` refuses a repository page whose path is inside the vault.

| Commit | Made by | Holds |
|---|---|---|
| `setup: …` | `atlas-obsidian vault init` | the layout |
| `snapshot: N files edited by hand` | every write tool, before it writes, when the tree is dirty | your edits, the session documents, the model's prose edits |
| `change: <title>` | `change` apply | the change's writes, its link rewrites, and the change document; trailer `Atlas-Change: chg-…` |
| `undo: <title>` | `change` undo | the restored paths and the change document |
| `capture: <title>` | `source` capture | the captured file, the source page, the removed inbox file |
| `thread: <summary>` | `thread` | the thread documents the call wrote, and for a rename its link rewrites; trailer `Atlas-Thread: thr-…` |

Every write to a document, from a tool, a hook, or `vault sync`, takes `.git/atlas.lock` (`flock`). The lock is held only for the write, so a hook waits milliseconds at most, and two sessions never write one file at once.

## Settings for the harness

`vault sync` keeps one list in `.claude/settings.local.json`: `permissions.additionalDirectories`, with the path of every repository page. An agent that starts in the vault can then edit a linked repository with no extra prompt.

- The file is Claude Code's local project settings, which git ignores, so a machine's absolute paths never travel with the vault.
- Sync merges: it adds and removes only the paths that repository pages name, and keeps every other key and entry.
- The session-start hook runs sync, so a session always starts with the list current. Claude Code reads the file at start, so a repository linked during a session needs `/add-dir <path>` or a new session; [[repo-link]] says so.
- Claude Code does not load a `CLAUDE.md` from these directories, so the `context` tool returns the repository's instruction files. See [[context]].
- The other way round, a session that starts inside a repository must reach the vault. `setup` offers to add each vault to `permissions.additionalDirectories` in the user's own settings (`~/.claude/settings.json`), and asks first.

## The machine file

`~/.atlas/config.json` lists your vaults and nothing else:

```json
{ "schema": "atlas.config.v1", "vaults": ["~/notes/work", "~/notes/home"] }
```

A hook reads it to find the vault of a session that starts inside a linked repository instead of in a vault.
