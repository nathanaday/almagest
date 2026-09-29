
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
propose ──▶ proposed ──apply──▶ applying ──▶ applied ──undo──▶ undone
               │                   │ (a crash)
               │                   └──recovery──▶ proposed
               ├──reject──▶ rejected
               └──propose with supersedes──▶ superseded
```

- **propose** validates the plan and writes the document. It commits nothing; the next commit of any kind keeps it.
- **apply** reads the document again, validates it again, and commits. The model's call applies only after the user has had a turn since the proposal ([[#The gate]]).
- **reject** sets the status and the reason, and commits nothing.
- **undo** restores the paths of one applied change. It refuses when a path changed since.

## The gate

A change needs your yes. Two paths give it:

1. **In the chat.** The skill shows the preview and waits. The `change` tool refuses the model's apply unless the session named in the change's `session` field has a `last_prompt` later than the change's `proposed` time. The tool checks the document it is about to apply, under the lock, however the call names the change and whatever folder the session runs in. A change with no `session`, or a `proposed` time that does not parse, stays shut; the user applies it in Obsidian. So the model cannot propose and apply in one turn: you always get a turn with the preview in front of you. Code cannot tell a yes from a no in your words; the skill reads your reply, and the gate makes sure there was one.
2. **In Obsidian.** The plugin's Apply button runs the CLI. The click is the yes ([[Obsidian Plugin#Review a change]]).

A change with no writes (the model found nothing new) changes no page, so the gate lets it through at once.

The gate holds against the tools, not against the shell. The guard refuses a Bash command that runs `atlas change apply` or `atlas hook`, but a shell can write any file, including the session document. The gate keeps an agent from applying by mistake or on instructions it read in a source; it is no sandbox.

## The change document

`changes/2026-09/2026-09-27 Ingest the DINOv2 paper.md`. The file name is the date and the plan's `title`; a second change with the same name on one day takes ` (2)`.

```yaml
---
id: chg-r8m3tb
type: change
created: 2026-09-27
updated: 2026-09-27
status: proposed                    # proposed | applying | applied | rejected | superseded | undone
absorbs: ["[[DINOv2]]"]             # the documents this change absorbs into the wiki
thread: ""                          # the thread it serves, if any
# owned by code:
proposed: 2026-09-27T14:51          # the gate compares it with the session's last_prompt
session: "[[2026-09-27 1432 a1b2c3]]"   # written by the PostToolUse hook of propose
counts: "3 create, 1 modify, 0 rename, 0 remove, 2 link rewrites"   # a string, for Obsidian; the Change Preview gives a map
applied: ""                         # the time; the commit is found by its trailer
supersedes: ""
reason: ""                          # for a rejected change
---
```

The body:

``````markdown
> [!change] Proposed · 3 create, 1 modify · absorbs [[DINOv2]]
> Review the pages below. Edit any of them here if you want. Then say yes in the chat, or press Apply.

## Notes

What the change does and why, in the model's words. Skipped subjects, with the reason for each.

## Absorbed

| Document | Id | Hash |
|---|---|---|
| [[DINOv2]] | src-p2x7nd | 3f9c1e2a7b8d |

## Writes

### create · concept · Self-supervised learning · con-k3m9qa

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

### modify · DINOv2 · src-p2x7nd · base 5b1d0e9a

`````markdown
the full new content of the page
`````

### rename · Old title → New title · con-z2b7rf

### remove · Stale page · con-q4w8xe · redirect con-k3m9qa

### link rewrites

- Filter vehicle false alarms — Spec: `[[Old title]]` → `[[New title]]`
``````

- Code writes the whole document except `## Notes`, which is the model's.
- Every write heading names its page by id, so a page renamed by hand between propose and apply is still the page that changes. Headings carry no wikilinks, so a renamed or removed page leaves no dead link in the record. Lint and search skip `## Writes`.
- A page's content sits in a fence of backticks longer than any run of backticks inside it, so any markdown survives. Obsidian draws no link from inside a fence, so a proposal adds no false edges to the graph.
- Code writes the fields it owns into each proposed page (`id`, `type`, `created`, `updated`) at propose time, so the page you review is the page that lands.
- `## Absorbed` is code's. It records each absorbed document's content hash at propose time.
- Apply, reject, and undo rewrite the lead callout to say the new status.

## Validation

`propose` and `apply` refuse a plan that breaks one of these rules. Each refusal names the write and the rule.

1. Every write the model gives is under `wiki/`, and never under `wiki/sources/files/`. Only capture writes there. Code's link rewrites may touch any folder ([[#Link rewrites]]).
2. A create names a type and a title. Code routes the path. The title and every alias are unique in the vault.
3. The page matches its type's schema ([[Wiki]]): the required fields exist and hold allowed values; `scope` and `parent` name scope pages; `sources` name documents that exist or that the same change creates; a repository's `path` is the root of a git work tree outside the vault, and no other page holds it.
4. The model sets no field that code owns. Code drops such a field and reports it as a warning.
5. A modify, rename, or remove names a page by id. Its `base` is the hash the model read; when the model gives none, code records the hash at propose time. Apply refuses when the file changed since.
6. `absorbs` names documents of a type in `wikify` ([[Vault Layout#Atlas.md]]).
7. At most 100 writes from the model. Link rewrites do not count.

Links in new content that resolve to nothing are warnings, not refusals, because a later change may create the target.

## Link rewrites

One pass, used by every rename and every remove with a redirect: a change's rename and remove, and `thread` set title.

- It rewrites every wikilink to the old title, with its alias and heading kept, in every document of the vault, your own notes included.
- It skips `## Writes` in change documents, which record what was true then.
- A wiki page it rewrites is a modify in the change, and the preview shows it. A document outside `wiki/` it rewrites is listed under `### link rewrites` and lands in the same commit. This is the one way a write reaches a folder that is not its own, and it only keeps links true.
- A remove with a redirect rewrites a link in a typed field only when the redirect's type fits the field's schema. A task's `repository` never becomes an area. A link it leaves is a warning in the preview, and [[lint]] reports it after.

## Apply

1. Take `.git/atlas.lock`.
2. **Recover.** When a change has `status: applying`, a write died halfway. Restore each of its paths from `HEAD`, or remove it when it is new, and set it back to `proposed`. Every write tool and `vault sync` run this step.
3. If the vault's tree is dirty, commit it as `snapshot: N files edited by hand`.
4. Parse the change document. Validate it again (you may have edited it).
5. Check every `base` against the file on disk. A mismatch aborts with `conflict` and names the path. The model reads the page again and proposes a change that supersedes this one.
6. Set `status: applying` in the change document.
7. Write each file (temporary file, then rename). Delete each removed file. Make the link rewrites.
8. Derive: `updated` on each page; `described` on a repository page whose snapshot the change absorbs.
9. Set `status: applied`, `applied`, and the lead callout.
10. Commit the written paths and the change document as `change: <title>` with the trailer `Atlas-Change: chg-r8m3tb`.

## Undo

1. Take the lock, recover, and commit a dirty tree as a snapshot.
2. Find the change's commit by its trailer.
3. Refuse when any path the change wrote differs from what the commit wrote (`git diff <commit> -- <paths>` is not empty). The change document itself is left out of this check. The refusal names the paths.
4. Restore each path from the commit's parent, or remove it when the change created it.
5. Set `status: undone` and the lead callout, and commit as `undo: <title>`.
6. Run `vault sync`, so the harness settings follow a restored or removed repository page.

Undo never uses `git revert`, so it needs no clean tree and never touches a path outside the change.

## Pending documents

A document is **pending** when its type is in `wikify` and no applied change records its current content hash in `## Absorbed`.

| Type | Content hash |
|---|---|
| `source` | the `sha256` of the captured file, which never changes |
| any other type in `wikify` | the sha256 of the body below the lead callout, with whitespace runs made one space |

A task counts only when it is `done`. So a source is pending from its capture until an ingest absorbs it, and a spec is pending again when someone edits it after the wiki absorbed it. An undone change no longer counts. The `vault` tool lists pending documents, and [[wiki-sync]] drains them.

## Changes.base

Views: proposed (waiting for you); applied, newest first; by thread; by session; rejected and undone.
