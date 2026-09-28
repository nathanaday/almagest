
# Changes

A change is the only way the wiki changes. It is a document: the model proposes it, you review it in the chat or in Obsidian, and apply writes the pages and makes one git commit. The folder `changes/` is the wiki's log, and `Changes.base` shows it.

## Why a document

V1 held a plan in the MCP server's memory between `plan` and `apply`. A change document does four more things:

1. You can review a large change (25 new pages from one paper) as rendered markdown in Obsidian, not as a diff in a terminal.
2. You can edit a proposed page before you approve it. Apply reads the document, so your edit goes in.
3. A proposal survives a restart of the session or the server.
4. The change is its own record. It replaces `log.md` and the history tool of V1, and it records which documents the wiki absorbed, which makes [[#Pending documents]] a fact code can derive.

## Lifecycle

```text
propose ──▶ proposed ──apply──▶ applied ──undo──▶ undone
               │
               ├──reject──▶ rejected
               └──propose with supersedes──▶ superseded
```

- **propose** validates the plan and writes the document. It commits nothing.
- **apply** reads the document again, validates it again, and commits. It needs the user's yes, which the skill asks for. A change with no writes (the model found nothing new) needs no yes, because it changes no page.
- **reject** sets the status and keeps the document, with the reason.
- **undo** restores the paths of one applied change. It refuses when a path changed since.

## The change document

`changes/2026-09/2026-09-27 Ingest the DINOv2 paper.md`

```yaml
---
id: chg-r8m3tb
type: change
created: 2026-09-27
updated: 2026-09-27
summary: "Ingest the DINOv2 paper: its source page, two concepts, one entity"
status: proposed                    # proposed | applied | rejected | superseded | undone
absorbs: ["[[DINOv2]]"]             # the documents this change absorbs into the wiki
thread: ""                          # the thread it serves, if any
# owned by code:
session: "[[2026-09-27 1432 a1b2c3]]"   # written by the PostToolUse hook
counts: "3 create, 1 modify, 0 rename, 0 remove"
applied: ""                         # the date; the commit is found by its trailer
supersedes: ""
reason: ""                          # for a rejected change
---
```

The body:

````markdown
> [!change] Proposed · 3 create, 1 modify · absorbs [[DINOv2]]
> Review the pages below. Edit any of them here if you want. Then say yes in the chat.

## Summary

What the change does and why, in the model's words. Skipped subjects, with the reason for each.

## Absorbed

| Document | Id | Hash |
|---|---|---|
| [[DINOv2]] | src-p2x7nd | 3f9c1e2a7b8d |

## Writes

### create · concept · Self-supervised learning

`````markdown
---
id: con-k3m9qa
type: concept
scope: "[[p3]]"
description: "Training a model on data with no labels, from a signal in the data itself."
sources: ["[[DINOv2]]"]
…
---
## Definition
…
`````

### modify · [[DINOv2]] · base 5b1d0e9a

`````markdown
the full new content of the page
`````

### rename · [[Old title]] → New title

### remove · [[Stale page]] → redirect [[Replacement]]
````

- Code writes the whole document except `## Summary`, which is the model's.
- A page's content sits in a fence of backticks longer than any run of backticks inside it, so any markdown survives. Obsidian draws no link from inside a fence, so a proposal adds no false edges to the graph.
- Code writes the fields it owns into each proposed page (`id`, `type`, `created`, `updated`) at propose time, so the page you review is the page that lands.
- `## Absorbed` is code's. It records each absorbed document's content hash at propose time.

## Validation

`propose` and `apply` refuse a plan that breaks one of these rules. Each refusal names the write and the rule.

1. Every write is under `wiki/`, and never under `wiki/sources/files/`. Only capture writes there.
2. A create names a type and a title. Code routes the path. The title and every alias are unique in the vault.
3. The page matches its type's schema ([[Wiki]]): the required fields exist and hold allowed values; `scope` and `parent` name scope pages; `sources` name documents that exist or that the same change creates.
4. The model sets no field that code owns. Code drops such a field and reports it as a warning.
5. A modify, rename, or remove names a page by id. Its `base` is the hash the model read; when the model gives none, code records the hash at propose time. Apply refuses when the file changed since.
6. A rename adds a modify for every page that links to the old title. A remove with `redirect` does the same toward the redirect. A remove without one lists the pages whose links it breaks, as warnings.
7. `absorbs` names documents of a type in `wikify` ([[Vault Layout#Atlas.md]]).
8. At most 100 writes. A larger batch is two changes.

Links in new content that resolve to nothing are warnings, not refusals, because a later change may create the target.

## Apply

1. Take `.git/atlas.lock`.
2. If the vault's tree is dirty, commit it as `snapshot: N files edited by hand`.
3. Parse the change document. Validate it again (you may have edited it).
4. Check every `base` against the file on disk. A mismatch aborts with `conflict` and names the path. The model reads the page again and proposes a change that supersedes this one.
5. Write each file (temporary file, then rename). Delete each removed file.
6. Derive: `updated` on each page; `described` on a repository page whose snapshot the change absorbs; `.claude/settings.json` when a repository page is created or removed.
7. Set the change document's `status: applied` and `applied`.
8. Commit the written paths and the change document as `change: <summary>` with the trailer `Atlas-Change: chg-r8m3tb`.
9. On any failure before the commit, restore every path from `HEAD`, or remove it when it is new. Return the error.

## Undo

1. Take the lock, and commit a dirty tree as a snapshot.
2. Find the change's commit by its trailer.
3. Refuse when any path of the change differs from what the commit wrote (`git diff <commit> -- <paths>` is not empty). The refusal names the paths.
4. Restore each path from the commit's parent, or remove it when the change created it.
5. Set `status: undone` and commit as `undo: <summary>`.

Undo never uses `git revert`, so it needs no clean tree and never touches a path outside the change.

## Pending documents

A document is **pending** when its type is in `wikify` and no applied change records its current content hash in `## Absorbed`.

| Type | Content hash |
|---|---|
| `source` | the `sha256` of the captured file, which never changes |
| `spec`, `receipt`, and any other type in `wikify` | the sha256 of the body below the lead callout, with whitespace runs made one space |

So a source is pending from its capture until an ingest absorbs it, and a spec is pending again when someone edits it after the wiki absorbed it. An undone change no longer counts. The `vault` tool lists pending documents, and [[wiki-sync]] drains them.

## Changes.base

Views: proposed (waiting for you); applied, newest first; by thread; by session; rejected and undone.
