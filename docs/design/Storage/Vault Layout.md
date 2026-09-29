# Vault Layout

The folders of a vault, the vault document, who writes where, and the rules for titles, links, and git. The document schema is on [[Documents]].

## Layout

```text
<vault>/
├── Atlas.md                  the vault document: identity, settings, the vault's own context
├── inbox/                    you drop files and notes here; wiki-ingest triages them
├── scratchpad/               yours; no schema; no tool reads it, and no agent unless you ask
├── wiki/
│   ├── documents/            every typed document, flat: source, repository, topic, stub, spec, event
│   └── assets/               the captured originals (<source id>.<ext>), and your attachments
├── sessions/
│   ├── Sessions.base         what runs now, and what ran
│   └── 2026-09/              one document per agent session, by month
├── changes/
│   ├── Changes.base          proposed and applied changes
│   └── 2026-09/              one document per change, by month
├── views/                    derived by code; out of git (see [[Views]])
├── .claude/settings.local.json   untracked; code lists the linked repositories here
├── .obsidian/                app settings and the Atlas plugin
└── .git/                     the vault's history; the write lock is .git/atlas.lock
```

- `wiki/documents/` has no subfolders. Code refuses to write a typed document anywhere else, and sync moves a typed document it finds elsewhere under `wiki/` into it ([[#Hand moves]]).
- `wiki/assets/` holds two kinds of file. Capture writes each original as `<source id>.<ext>` and never edits it again. Obsidian saves your pasted images and attachments there too, because `init` sets it as the attachment folder. Code never edits or removes a file there that no source names.
- `atlas-obsidian vault init` writes this layout, `Atlas.md`, the two Bases, and one `setup` commit. `EnsureFolders` makes a folder that a clone left out, because git keeps no empty folder, and writes `views/` when it is missing.

## Who writes where

| Path | The model | Code | You |
|---|---|---|---|
| `Atlas.md` | no | `init` only | yes |
| `inbox/` | no | `source` capture and `work` stub remove the file they replace | yes |
| any note of yours | no | a link or tag rewrite; closing a mention | yes |
| `scratchpad/` | no | no; search, lint, and the views skip it | yes |
| `wiki/documents/`: source, repository, topic | only through `change` | `change` apply, `source` capture, sync of derived fields | yes |
| `wiki/documents/`: stub, spec, event | Edit of the prose of a document that exists | `work`, `change` (events of a promotion), sync | yes |
| `wiki/assets/` | no | `source` capture | yes |
| `sessions/` | Edit of three sections of its own session document | the hooks | yes |
| `changes/` | through `change` | `change` | yes, a proposed change before you approve it |
| `views/` | no | `vault sync` | no; sync writes over your edits |
| `*.base` in `sessions/`, `changes/` | no | `init`; sync replaces an unedited file of an earlier release | yes |
| `.claude/settings.local.json` | no | `vault sync` | yes, every key but the one list code keeps |

The guard hook enforces the model's column ([[Hooks#guard]]).

## Atlas.md

The vault document. Every session reads it first. It is not in `wiki/documents/`, and it is not one of the six types.

```yaml
---
id: vlt-k3m9qa
type: vault
name: Work
description: "Work notes: the p3 product and the tools around it."   # one line; agents read it first
created: 2026-09-27T10:02:11
updated: 2026-09-29T09:15:40
tags: known                        # open | known: how freely the agent adds a tag
wikify: [source, spec, event]      # the types that are pending until a change absorbs them
stale_hours: 12                    # a live session with no hook event for this long is lost
layout: 3                          # code's: 3 is the flat layout of 7.0; sync refuses an older vault
---
```

The body is yours: what this vault is for, and the context that every agent in it must know.

The `tags` setting comes from the onboarding question:

| Answer | `tags` | The agent |
|---|---|---|
| "Tag freely; I will tidy later" | `open` | adds a tag when no tag that exists fits, and says so in the preview |
| "Use my tags; ask before a new one" | `known` | uses only tags that some document holds; asks before it adds one |

`wikify` names types. For `event` it means the events that carry prose: `completed`, `dropped`, and `note` ([[Event#Pending]]).

## Titles and file names

- The file name is the title. Code writes the file name from the title the model gives, with the characters `/ \ : * ? " < > | [ ] # ^` and a leading dot removed.
- A title is unique in the vault, across every folder, compared without case. Code refuses to create or rename a document to a title or alias that another document holds. A bare `[[Title]]` then has one target.
- No document title begins with `Tag · ` or `View · `. Those belong to the view notes ([[Views]]).
- Event titles are code's: the subject's title, the kind, and the time ([[Event#Title]]).
- Session and change documents take a date and a short name: `2026-09-29 1432 a1b2c3`, `2026-09-29 Ingest the DINOv2 paper`.

## Links

- Links between documents are Obsidian wikilinks by title: `[[DINOv2]]`. A link in frontmatter is quoted: `subject: "[[Score boxes by motion]]"`.
- Tools take and return ids. A document states its links by title, so Obsidian's graph, backlinks, and Bases work on them.
- A rename is a write of its own (`change` rename, `work` set title). Code rewrites every link to the old title in the same commit ([[Changes#Link rewrites]]).

## Hand moves

You may move or rename files yourself, in Obsidian or in a shell.

- A rename of a document in Obsidian keeps its links, because Obsidian rewrites them. The next sync checks that the new title is unique, and lint reports a clash.
- A typed document moved out of `wiki/documents/` into another folder of `wiki/` goes back at the next sync, and the next snapshot commit records the move. A typed document moved out of `wiki/` altogether (to `scratchpad/`, say) stays there; lint reports it as `misplaced`, and tools do not see it until it returns.
- A file with no type in `wiki/documents/` stays where it is. Lint reports it as `untyped`, and [[wiki-ingest]] offers to capture it.

## Git

The vault is one git repository, on `main`. It never contains another repository: `init` refuses a folder inside another repository's work tree, and `change` refuses a repository document whose path is inside the vault.

| Commit | Made by | Holds |
|---|---|---|
| `setup: …` | `vault init` | the layout |
| `snapshot: N files edited by hand` | every write tool, before it writes, when the tree is dirty | your edits, the session documents, the model's prose edits |
| `change: <title>` | `change` apply | the change's writes, its events, its link and tag rewrites, and the change document; trailer `Atlas-Change: chg-…` |
| `undo: <title>` | `change` undo | the restored paths and the change document |
| `capture: <title>` | `source` capture | the captured file, the source document, the removed inbox file, a `resolved` event when capture resolves a stub |
| `work: <summary>` | `work` | the documents the call wrote, its events, and for a rename its link rewrites; trailer `Atlas-Work: doc-…` |
| `layout: migrate to 7.0` | `vault migrate` | every move and rewrite of [[Migration]] |

Every write to a document, from a tool, a hook, or `vault sync`, takes `.git/atlas.lock` (`flock`). The lock is held only for the write, so a hook waits milliseconds at most, and two sessions never write one file at once. The lock is not re-entrant: a write takes it once, and inner functions assume it held.

### Out of git

`EnsureFolders` keeps these entries in `.git/info/exclude`, so `init` edits no file of yours:

- `views/`;
- `.claude/settings.local.json`;
- `.obsidian/workspace*.json` and `.obsidian/graph.json`, which Obsidian rewrites as you work.

A vault that tracked one of them gets one commit, `untrack machine files: …`, and the file stays on disk.

## Settings for the harness

`vault sync` keeps one list in `.claude/settings.local.json`: `permissions.additionalDirectories`, with the path of every repository document. An agent that starts in the vault can then edit a linked repository with no extra prompt.

- Sync adds every path the repository documents name. It removes only the paths that the documents named before a write and no longer name, so a path you added by hand stays.
- The session-start hook runs sync. Claude Code reads the file at start, so a repository linked during a session needs `/add-dir <path>` or a new session; [[repo-link]] says so.
- Claude Code does not load a `CLAUDE.md` from these directories, so the [[context]] tool returns the repository's instruction files.
- A session that starts inside a repository must reach the vault. `setup` offers to add each vault to `permissions.additionalDirectories` in `~/.claude/settings.json`, and asks first.

## The machine file

`~/.atlas/config.json` lists your vaults and nothing else:

```json
{ "schema": "atlas.config.v1", "vaults": ["~/notes/work", "~/notes/home"] }
```

A hook reads it to find the vault of a session that starts inside a linked repository instead of in a vault.
