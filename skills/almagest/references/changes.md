# The change contract

A change is the only way knowledge changes: a source, a repository, or a topic. You
build a plan, `change` propose writes it as a change document, the user reads it and
decides, and `change` apply makes one git commit. The guard refuses Write and Edit of a
knowledge document, and the `change` tool refuses the model's apply in the turn that
proposed it.

## The plan

```yaml
title: "Ingest the DINOv2 paper"      # short: the file name and the commit subject
notes: "…"                            # what the change does and why; every skipped subject with its reason
absorbs: [doc-p2x7nd]                 # the sources the change absorbs, repository snapshots included
supersedes: ""                        # a proposed change this one replaces
new_tags: false                       # true: allow tags no document holds, in tagging: known, after the user's yes
writes:
  - op: create
    why: "DINOv2 trains its backbone this way; no topic explains it"   # one line, for the Summary; give one for every write
    type: topic                       # topic or repository
    kind: concept                     # a topic's kind: concept, entity, policy, or overview
    title: "Self-supervised learning"
    fields:                           # the type's fields; links as ids or titles
      description: "Training a model on data with no labels, from a signal in the data itself."
      tags: [ml/self-supervised, vision]
      aliases: [SSL]
      status: stable
      sources: [doc-p2x7nd]
    body: "## Definition\n…"
  - op: modify
    id: doc-p2x7nd
    base: 5b1d0e9a                    # the hash you read; optional
    fields: {description: "…", authority: primary}   # merged over the document's fields
    body: "…"                         # replaces the body below the lead callout; leave it out to keep the body
  - op: rename
    id: doc-k3m9qa
    title: "Self-supervised representation learning"
  - op: remove
    id: doc-z2b7rf
    redirect: doc-k3m9qa              # optional: links to the removed document go here
  - op: confirm
    id: doc-m5r1ty                    # a review found it current; sets refreshed only
  - op: retag
    from: p3
    to: work/p3
```

| Op | Takes | Does |
|---|---|---|
| `create` | `type` (topic or repository), `kind` for a topic, `title`, `fields`, `body` | a new document in `tool/source-core/documents/` |
| `modify` | `id`, `fields` (merged; `null` removes one), `body` (replaces; leave out to keep), `base` | rewrites a source, a repository, or a topic |
| `rename` | `id`, `title` | renames the file and rewrites every link to the old title |
| `remove` | `id`, optional `redirect` | moves the document to `tool/trash/`; links to it go to the redirect |
| `confirm` | `id`, `base` | sets `refreshed` and nothing else |
| `retag` | `from`, `to` | renames a tag and every tag below it, in every document |

- Every write takes an optional `why`: one line (code cuts it at 160 characters) that
  says why the change makes this write. The change document's Summary shows it. Give a
  `why` for every write.
- Give a type, a kind for a topic, and a title for a new document. Code mints the id,
  writes the path (`tool/source-core/documents/<title>.md`), and writes `id`, `type`,
  `created`, `updated`, and `refreshed`. It drops any field it owns and warns.
- Name a document that exists by its id. Give `base` only when you read the document's
  hash; code records it otherwise, and apply refuses when the file changed since.
- Use rename, never a remove and a create: code rewrites every link to the old title in
  the same commit. It rewrites no note in `scratchpad/`, `journals/`, or
  `checkout/`.
- Use retag to rename a tag: code rewrites it in `tags` and `defines` of every typed
  document, and every inline `#tag` in every note outside those four folders. A retag
  to a tag that exists merges the two.
- A source comes only from `source` capture. A change modifies it; it never creates one.
- A repository is unlinked with a modify `{fields: {unlinked: true}}`; code empties its
  `path`.
- A remove deletes nothing. Apply moves the document to
  `tool/trash/<YYYY-MM-DD>/<its vault path>`, and undo moves it back. Tell the user that a
  removed document goes to the trash.

## Validation

`change` propose and apply refuse a plan that breaks a rule, and name the write and the
rule. Fix the plan and propose again.

1. A create names a type of `topic` or `repository`, and a title. The title and every
   alias are unique in the vault, without case, and begin with neither `Tag · ` nor
   `View · `.
2. A modify, rename, remove, or confirm names a document of a type the op
   allows.
3. A document matches its type (see [pages.md](pages.md)): the required fields exist and
   hold allowed values; `sources` name documents that exist or that the same change
   creates; a repository's `path` is absolute or starts with `~/`, is the root of a git
   work tree outside the vault, and no other repository holds it; only an overview or a
   repository defines a tag, and one document defines each tag.
4. Every tag is valid. In `tagging: known`, a tag that no document holds needs
   `new_tags: true` on the plan. Set it only after the user agreed in the chat.
5. `absorbs` names sources.
6. At most 100 writes. Split a larger change into several. Link and tag rewrites do not
   count.

A link in new content that resolves to nothing is a warning, not a refusal.

## The work document

A work document is a change document that exists before its writes. The user watches
one document from the first step to the decision. Its status is `running` until the
agent proposes into it.

1. **Start.** The palette in Obsidian starts the work document, and its message to the
   agent names it: "Your work document is [[…]] (<id>)". Use that id. When no message
   names one, call `change` with `action: start`, `kind` (`ingest`, `repair`, or
   `draft`), a `title`, and for an ingest `files` (the names of the files in
   `ingest/`). Keep the `ref.id` of the result. A `draft` work document holds the
   drafting of one topic: Create on a new mark of a wikified note starts one, titled
   "Draft <Title>" ([wiki-edit](../../wiki-edit/SKILL.md), its Draft path). Its
   default title is "Draft a topic".
2. **Progress.** After each step, call `change` with `action: progress`, the `id`, and
   `text`: one line that says what was done ("captured 3 sources", "extracted 9 of 12
   chunks"). Code adds the line with the time under `## Progress`. Code cuts a line at
   200 characters.
3. **Propose.** Call `change` with `action: propose`, the plan's fields, and `id`. Code
   writes the plan into the work document and sets it to `proposed`. The document keeps
   its `## Files` and `## Progress`. Progress refuses a change that is not running, so
   give the last progress line before the proposal.
4. **Cancel.** The user can press Cancel in the running document. Then `progress` and
   `propose` refuse with "the user cancelled <title> (<reason>); stop the work". Stop at
   once: send no more workers and make no more calls. Say in one line that the user
   cancelled the work.

To end a running document that has nothing to propose, call `change` with
`action: reject`, the `id`, and the `reason` ("nothing in ingest/ goes to the wiki").

## The Summary

Code writes a `## Summary` section in every proposed change document, below the
Approve and Cancel widget. It has one line per write, with the write's `why`:

```
- **create** topic (concept) [[Self-supervised learning]] · DINOv2 trains its backbone this way
- **modify** [[DINOv2]] · +12 −3 · the summary and the structure of the paper
- **rename** Old title → [[New title]] · the common name
- **remove** Old idea (to trash) · merged into [[New title]]
- **confirm** [[Vision transformer]] · checked against the paper
- **retag** #p3 → #work/p3 · 4 files
```

The Summary links only the documents that stay, so a removed or renamed title is plain
text. The `## Writes` section below it holds the full text of each write. The user reads
the Summary first, so a good `why` saves the user from reading the Writes.

## Propose and wait

1. Call `change` with `action: propose` and the plan's fields, with `id` when a work
   document runs.
2. End the turn with one or two lines: the change document as a link, the count of
   writes, and each new tag. Do not list the writes in the chat; the Summary lists them.
   Name a warning only when the user must act on it.
3. Stop and wait. Do not apply in the same turn: the tool refuses it. The user decides
   once, in the document (Approve or Cancel), or in the chat. A change the user approves
   or cancels in Obsidian needs nothing more from you; `change` show reports it as
   applied or rejected.
4. On a yes in the chat, call `change` with `action: apply` and the change's `id`. Say
   the commit in one line.
5. On a no, call `change` with `action: reject`, the `id`, and the user's `reason`.

A change with no writes (the sources held nothing new) changes no document. Apply it
at once and say in one line that the sources are no longer pending.

## A session that ended

Apply refuses a change when the session that proposed it ended, or is lost, before the
user answered there. No answer reaches that session. Tell the user to press Approve in
the change document, or propose the plan again from this session with `supersedes` set
to the old change's id.

## Conflicts

Apply refuses with `conflict` when a document changed since the proposal. Read the
document again, build the plan again from what is there now, and propose with
`supersedes` set to the old change's id.

## Undo

`change` undo restores the paths of one applied change from the commit before it. It
refuses when one of them changed since; then make the fix as a new change.

## Safe delete

`almagest vault trash PATH` is the user's safe delete. The palette in Obsidian
runs it on the open file; no MCP action does it, and an agent never runs it.

- When documents or notes link the file, it changes nothing, lists them, and exits 2.
  The palette then offers an agent (wiki-edit) that points each link elsewhere and
  proposes the remove.
- With no links, a knowledge document leaves through a change that applies at once as
  the user's own action ("Delete <title>", one remove), so undo works on it. Any other
  file moves to `tool/trash/` in a commit `trash: <path>`.
- It refuses `Almagest.md`, the setting folders, `changes/`, `tool/sessions/`, `wiki-view/`,
  `tool/trash/`, a Base that Almagest ships, and a folder.

The guard refuses every agent edit in `tool/trash/`. The user empties it.

## Pending sources

A source is pending until an applied change lists it in `absorbs` with its current
content. A repository snapshot is a source too. `vault` status lists the pending sources;
the wiki-sync skill absorbs them.
