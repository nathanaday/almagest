
# Vault Layout

The layout of a vault, the vault document, and the rules for ids, titles, links, and git.

## Layout

```text
<vault>/
├── Atlas.md                  the vault document: identity, settings, the vault's own context
├── inbox/                    you drop files and notes here; wiki-ingest triages them
├── scratchpad/               yours; no schema; no agent reads it unless you ask
├── sessions/
│   ├── Sessions.base         what runs now, and what ran
│   └── 2026-09/              one document per agent session, by month
├── threads/
│   ├── Threads.base          the board
│   └── <Thread title>/       one folder per thread: stub, spec, tasks, receipt
├── changes/
│   ├── Changes.base          proposed and applied changes
│   └── 2026-09/              one document per change, by month
├── wiki/
│   ├── Wiki.base             every page by type and scope
│   ├── areas/  repositories/                 scope pages
│   ├── concepts/  entities/  policies/       knowledge pages
│   └── sources/  sources/files/              source pages, and the captured originals
├── .claude/settings.json     code lists the linked repositories here
├── .obsidian/                app settings and the Atlas plugin
└── .git/                     the vault's history; apply's lock is .git/atlas.lock
```

`atlas init` writes this layout, the four Bases, and one `setup` commit. `EnsureFolders` rebuilds a folder that a clone left out, because git does not keep an empty folder.

## Who writes where

| Path | The model | Code | You |
|---|---|---|---|
| `Atlas.md` | no | `init` only | yes |
| `inbox/` | no | `source` capture removes a file in its commit | yes |
| `scratchpad/` | no | no | yes |
| `wiki/` | only through `change` | `change` apply, `source` capture | yes |
| `threads/` | Edit on the prose of a document that exists | `thread`, the hooks | yes |
| `sessions/` | through `session` | the hooks | yes |
| `changes/` | through `change` | `change` | yes, a proposed change before you approve it |
| `*.base` | no | `init` only | yes |
| `.claude/settings.json` | no | `change` apply, when a repository page is created or removed | no |

The guard hook enforces the model's column. See [[Hooks#guard]].

## Atlas.md

The vault document is the root of the context graph. Every session reads it first.

```yaml
---
id: vlt-k3m9qa
type: vault
name: Work
created: 2026-09-27
updated: 2026-09-27
areas: few                          # many | few | manual: how readily the agent proposes areas
wikify: [source, spec, receipt]     # the types that are pending until a change absorbs them
stale_hours: 12                     # a running session with no hook event for this long is lost
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

The vault is one git repository, on `main`. It never contains another repository: `atlas init` refuses a folder inside a git work tree, and `change` refuses a repository page whose path is inside the vault.

| Commit | Made by | Holds |
|---|---|---|
| `setup: …` | `atlas init` | the layout |
| `snapshot: N files edited by hand` | every write tool, before it writes, when the tree is dirty | your edits, the session documents, the model's prose edits |
| `change: <summary>` | `change` apply | the change's writes and the change document; trailer `Atlas-Change: chg-…` |
| `undo: <summary>` | `change` undo | the restored paths and the change document |
| `capture: <title>` | `source` capture | the captured file, the source page, the removed inbox file |
| `thread: <summary>` | `thread` | the thread documents the call wrote; trailer `Atlas-Thread: thr-…` |

A write tool takes `.git/atlas.lock` (`flock`) for the length of its commit, so two sessions never commit at once.

## Settings for the harness

`.claude/settings.json` holds one list that code derives: `permissions.additionalDirectories`, the path of every repository page. An agent that starts in the vault can then edit a linked repository with no extra prompt. Claude Code does not load a `CLAUDE.md` from these directories, so the `context` tool returns the repository's instruction files. See [[context]].

## The machine file

`~/.atlas/config.json` lists your vaults and nothing else:

```json
{ "schema": "atlas.config.v1", "vaults": ["~/notes/work", "~/notes/home"] }
```

A hook reads it to find the vault of a session that starts inside a linked repository instead of in a vault.
