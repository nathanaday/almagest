# Changes

A change is the only way knowledge changes: a source, a repository, or a topic. It is a document. The model proposes it, you review it in the chat or in Obsidian, and apply writes the documents and makes one git commit. The folder `changes/` is the wiki's log, and `Changes.base` shows it.

Work documents (stub, spec, event) do not go through a change. The `work` tool writes them, one commit per call ([[work]]).

## Why a document

1. You can review a large change (25 new topics from one paper) as rendered markdown in Obsidian, not as a diff in a terminal.
2. You can edit a proposed document before you approve it. Apply reads the change document, so your edit goes in.
3. A proposal survives a restart of the session or the server.
4. The change is its own record. It records which documents the wiki absorbed, which makes [[#Pending documents]] a fact code can derive.

## Lifecycle

```text
propose ──▶ proposed ──apply──▶ applying ──▶ applied ──undo──▶ undone
               │                   │ (a crash)
               │                   └──recovery──▶ proposed
               ├──reject──▶ rejected
               └──propose with supersedes──▶ superseded
```

- **propose** validates the plan and writes the change document. It commits nothing; the next commit of any kind keeps it.
- **apply** reads the document again, validates it again, and commits. The model's call applies only after the user has had a turn since the proposal ([[#The gate]]).
- **reject** sets the status and the reason, and commits nothing.
- **undo** restores the paths of one applied change. It refuses when a path changed since.

## The gate

A change needs your yes. Two paths give it:

1. **In the chat.** The skill shows the preview and waits. The `change` tool refuses the model's apply unless the session named in the change's `session` field has a `last_prompt` later than the change's `proposed` time. The tool checks the document it is about to apply, under the lock, however the call names the change and whatever folder the session runs in. A change with no `session`, or a `proposed` time that does not parse, stays shut; you apply it in Obsidian. So the model cannot propose and apply in one turn. Code cannot tell a yes from a no in your words; the skill reads your reply, and the gate makes sure there was one.
2. **In Obsidian, or a terminal.** The plugin's Apply button runs the CLI, which passes no gate. The click is the yes ([[Obsidian Plugin#Review a change]]).

A change with no writes (the model found nothing new) changes no document, so the gate lets it through at once.

The gate holds against the tools, not against the shell. The guard refuses a Bash command that runs `atlas-obsidian change … apply` or `atlas-obsidian hook`, but a shell can write any file. The gate keeps an agent from applying by mistake or on instructions it read in a source; it is no sandbox.

## The change document

`changes/2026-09/2026-09-29 Ingest the DINOv2 paper.md`. The file name is the date and the plan's `title`; a second change with the same name on one day takes ` (2)`.

```yaml
---
id: chg-r8m3tb
type: change
created: 2026-09-29T14:51:03
updated: 2026-09-29T14:51:03
status: proposed                    # proposed | applying | applied | rejected | superseded | undone
absorbs: ["[[DINOv2]]"]             # the documents this change absorbs into the wiki
work: ""                            # the stub or spec it serves, if any
# owned by code:
proposed: 2026-09-29T14:51:03       # the gate compares it with the session's last_prompt
session: "[[2026-09-29 1432 a1b2c3]]"   # written by the PostToolUse hook of propose
counts: "3 create, 1 modify, 0 rename, 0 remove, 0 promote, 0 confirm, 0 retag; 2 link rewrites"
new_tags: [ml/self-supervised]      # tags no document held before this change
applied: ""                         # the time; the commit is found by its trailer
supersedes: ""
reason: ""                          # for a rejected change
paths: []                           # while applying: every path the apply may write, for recovery
---
```

The body:

``````markdown
> [!change] Proposed · 3 create, 1 modify · absorbs [[DINOv2]]
> Review the documents below. Edit any of them here if you want. Then say yes in the chat, or press Apply.

## Notes

What the change does and why, in the model's words. Skipped subjects, with the reason for each.

## Absorbed

| Document | Id | Hash |
|---|---|---|
| [[DINOv2]] | doc-p2x7nd | 3f9c1e2a7b8d |

## Writes

### create · topic concept · Self-supervised learning · doc-k3m9qa

`````markdown
---
id: doc-k3m9qa
type: topic
kind: concept
description: "Training a model on data with no labels, from a signal in the data itself."
tags: [ml/self-supervised, vision]
sources: ["[[DINOv2]]"]
…
---
## Definition
…
`````

### modify · DINOv2 · doc-p2x7nd · base 5b1d0e9a

`````markdown
the full new content of the document
`````

### promote · Try DINOv2 for alarms → topic concept · doc-c7v2kq · base 8e2a41f0

### rename · Old title → New title · doc-z2b7rf

### remove · Stale page · doc-q4w8xe · redirect doc-k3m9qa

### confirm · Vision transformer · doc-m5r1ty

### retag · p3 → work/p3 · 23 documents

### link rewrites

- Filter vehicle false alarms: `[[Old title]]` → `[[New title]]`
``````

- Code writes the whole document except `## Notes`, which is the model's.
- Every write heading names its document by id, so a document renamed by hand between propose and apply is still the one that changes. Headings carry no wikilinks, so a renamed or removed document leaves no dead link in the record. Lint and search skip `## Writes`.
- A document's content sits in a fence of backticks longer than any run of backticks inside it, so any markdown survives. Obsidian draws no link from inside a fence, so a proposal adds no false edges to the graph.
- Code writes the fields it owns into each proposed document (`id`, `type`, `created`, `updated`, `refreshed`) at propose time, so the document you review is the document that lands.
- `## Absorbed` is code's. It records each absorbed document's content hash at propose time.
- Apply, reject, and undo rewrite the lead callout to say the new status.

## Operations

| Op | Takes | Does |
|---|---|---|
| `create` | a type (`topic` or `repository`), a kind for a topic, a title, `fields`, `body` | writes a new document in `wiki/documents/` |
| `modify` | an id, `fields` (merged), `body` (replaces; leave out to keep), `base` | rewrites a source, repository, or topic |
| `promote` | the id of a stub, `kind`, `fields`, `body`, an optional new title, `base` | makes the stub a topic in place ([[Stub#In place]]), and writes a `promoted` event |
| `rename` | an id, a title | renames the file and rewrites every link ([[#Link rewrites]]) |
| `remove` | an id, an optional `redirect` | deletes the document; links to it go to the redirect |
| `confirm` | an id, `base` | sets `refreshed` and nothing else: a review found the document current |
| `retag` | `from`, `to` | renames a tag in every document ([[#Tag rewrites]]) |

A source is created only by capture, and a stub, spec, or event only by `work`. A change writes a work document in three cases only: a `promote` of a stub, the `promoted` event it writes, and the link and tag rewrites.

## Validation

`propose` and `apply` refuse a plan that breaks one of these rules. Each refusal names the write and the rule.

1. A create names a type of `topic` or `repository`, and a title. Code mints the id; the path is always `wiki/documents/<title>.md`. The title and every alias are unique in the vault, and begin with neither `Tag · ` nor `View · `.
2. A modify, promote, rename, remove, or confirm names a document by id, of a type the op allows.
3. The document matches its type's schema: the required fields exist and hold allowed values; `sources` name documents that exist or that the same change creates; a repository's `path` is the root of a git work tree outside the vault, and no other document holds it; `defines` is unique.
4. The model sets no field that code owns. Code drops such a field and reports it as a warning.
5. Every tag is valid ([[Documents#Form]]). In `tagging: known` mode, a tag that no document holds needs `new_tags: true` on the plan.
6. A modify, promote, or confirm carries `base`, the hash the model read; when the model gives none, code records the hash at propose time. Apply refuses when the file changed since.
7. `absorbs` names documents of a type in `wikify` ([[Vault Layout#Atlas.md]]).
8. At most 100 writes from the model. Link and tag rewrites do not count.

Links in new content that resolve to nothing are warnings, not refusals, because a later change may create the target.

## Link rewrites

One pass, used by every rename and every remove with a redirect: a change's `rename` and `remove`, a `promote` with a new title, and `work` set title.

- It rewrites every wikilink to the old title, with its alias and heading kept, in every markdown file of the vault, your own notes included.
- It skips `## Writes` in change documents, which record what was true then.
- A knowledge document it rewrites is a modify in the change, and the preview shows it. Any other file it rewrites is listed under `### link rewrites` and lands in the same commit. This is the one way a change writes outside knowledge, and it only keeps links true.
- A remove with a redirect rewrites a link in a typed field only when the redirect's type fits the field's schema. A spec's `repositories` never names a topic. A link it leaves is a warning in the preview, and [[lint]] reports it after.

## Tag rewrites

`op: retag` renames a tag `from` to `to` in one pass:

- In every typed document, it rewrites `from` and every tag below it in `tags` and `defines`: `p3` → `work/p3` makes `p3/edge` into `work/p3/edge`.
- In every markdown file, it rewrites the inline tags `#from` and `#from/…` the same way, outside code blocks and fences.
- When `to` exists, the two merge. A document that then holds one tag twice keeps it once. When both tags had a page, the retag is refused; remove or merge one page first.
- It skips `## Writes` in change documents.
- Every file it rewrites lands in the change's commit. The preview counts them, and lists the knowledge documents among them.

## Apply

1. Take `.git/atlas.lock`.
2. **Recover.** When a change has `status: applying`, a write died halfway. Restore each path in its `paths` from `HEAD`, or remove it when it is new, and set it back to `proposed`. Every write tool and `vault sync` run this step, at every session start.
3. If the vault's tree is dirty, commit it as `snapshot: N files edited by hand`.
4. Parse the change document. Validate it again (you may have edited it).
5. Check every `base` against the file on disk. A mismatch aborts with `conflict` and names the path. The model reads the document again and proposes a change that supersedes this one.
6. Set `status: applying` and `paths` in the change document.
7. Write each file (a temporary file, then a rename). Delete each removed file. Make the link and tag rewrites. Write the `promoted` events.
8. Derive: `updated` and `refreshed` on each written document; `described` and `behind` on a repository whose snapshot the change absorbs; `status` of each absorbed source.
9. Set `status: applied`, `applied`, and the lead callout; remove `paths`.
10. Commit the written paths and the change document as `change: <title>` with the trailer `Atlas-Change: chg-r8m3tb`.
11. Run `vault sync`, so the views and the harness settings follow.

## Undo

1. Take the lock, recover, and commit a dirty tree as a snapshot.
2. Find the change's commit by its trailer.
3. Refuse when any path the change wrote differs from what the commit wrote (`git diff <commit> -- <paths>` is not empty). The change document itself is left out of this check. The refusal names the paths.
4. Restore each path from the commit's parent, or remove it when the change created it. This removes the change's events too.
5. Set `status: undone` and the lead callout, and commit as `undo: <title>`.
6. Run `vault sync`, so the statuses, the views, and the harness settings follow.

Undo never uses `git revert`, so it needs no clean tree and never touches a path outside the change.

## Pending documents

A document is **pending** when its type is in `wikify` and no applied change records its current content hash in `## Absorbed`.

| Type | Content hash |
|---|---|
| `source` | the `sha256` of the captured file, which never changes |
| `spec` | the sha256 of the body without the lead callout and the code's sections (`## Parts`, `## History`, `## Implemented by`), with whitespace runs made one space |
| `event` | the same, over its prose; only the kinds that hold prose |
| `stub`, when listed | the same, over `## Idea` |

So a source is pending from its capture until an ingest absorbs it, and a spec is pending again when someone edits it after the wiki absorbed it. An undone change no longer counts. The `vault` tool lists pending documents, and [[wiki-sync]] drains them.

## Changes.base

Views: proposed (waiting for you); applied, newest first; by work; by session; rejected and undone.
