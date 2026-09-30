# The change contract

A change is the only way knowledge changes: a source, a repository, or a topic. You
build a plan, `change` propose writes it as a change document, the user reads it and
says yes, and `change` apply makes one git commit. The guard refuses Write and Edit of a
knowledge document, and the `change` tool refuses the model's apply in the turn that
proposed it.

Work documents (stub, spec, event) do not go through a change. The `work` tool writes
them ([work.md](work.md)).

## The plan

```yaml
title: "Ingest the DINOv2 paper"      # short: the file name and the commit subject
notes: "…"                            # what the change does and why; every skipped subject with its reason
absorbs: [doc-p2x7nd]                 # the documents the change absorbs (sources, specs, events)
work: ""                              # the stub or spec it serves, if any
supersedes: ""                        # a proposed change this one replaces
new_tags: false                       # true: allow tags no document holds, in tagging: known, after the user's yes
writes:
  - op: create
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
  - op: promote
    id: doc-c7v2kq                    # an open stub; it becomes a topic in place
    kind: concept
    title: "Motion scoring"           # optional: a new title
    fields: {description: "…", sources: [doc-p2x7nd]}
    body: "## Definition\n…"          # code keeps the stub's ## Idea as ## Origin
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
| `create` | `type` (topic or repository), `kind` for a topic, `title`, `fields`, `body` | a new document in `wiki/documents/` |
| `modify` | `id`, `fields` (merged; `null` removes one), `body` (replaces; leave out to keep), `base` | rewrites a source, a repository, or a topic |
| `promote` | `id` of an open stub, `kind`, `fields`, `body`, optional `title`, `base` | the stub becomes a topic in place, with its id and every link; a `promoted` event |
| `rename` | `id`, `title` | renames the file and rewrites every link to the old title |
| `remove` | `id`, optional `redirect` | deletes the document; links to it go to the redirect |
| `confirm` | `id`, `base` | sets `refreshed` and nothing else |
| `retag` | `from`, `to` | renames a tag and every tag below it, in every document |

- Give a type, a kind for a topic, and a title for a new document. Code mints the id,
  writes the path (`wiki/documents/<title>.md`), and writes `id`, `type`, `created`,
  `updated`, and `refreshed`. It drops any field it owns and warns.
- Name a document that exists by its id. Give `base` only when you read the document's
  hash; code records it otherwise, and apply refuses when the file changed since.
- Use rename, never a remove and a create: code rewrites every link to the old title,
  in every markdown file of the vault, in the same commit.
- Use retag to rename a tag: code rewrites it in `tags` and `defines` of every typed
  document, and every inline `#tag` in every note. A retag to a tag that exists merges
  the two.
- A source comes only from `source` capture. A change modifies it; it never creates one.
- A stub, a spec, or an event comes only from `work`. A change writes a work document
  only through a `promote`, its `promoted` event, and the link and tag rewrites.
- A repository is unlinked with a modify `{fields: {unlinked: true}}`; code empties its
  `path`. `change` refuses to remove a repository that a spec names.

## Validation

`change` propose and apply refuse a plan that breaks a rule, and name the write and the
rule. Fix the plan and propose again.

1. A create names a type of `topic` or `repository`, and a title. The title and every
   alias are unique in the vault, without case, and begin with neither `Tag · ` nor
   `View · `.
2. A modify, promote, rename, remove, or confirm names a document of a type the op
   allows.
3. A document matches its type (see [pages.md](pages.md)): the required fields exist and
   hold allowed values; `sources` name documents that exist or that the same change
   creates; a repository's `path` is absolute or starts with `~/`, is the root of a git
   work tree outside the vault, and no other repository holds it; only an overview or a
   repository defines a tag, and one document defines each tag.
4. Every tag is valid. In `tagging: known`, a tag that no document holds needs
   `new_tags: true` on the plan. Set it only after the user agreed in the chat.
5. `absorbs` names documents of a type that the vault's `wikify` setting lists.
6. At most 100 writes. Split a larger change into several. Link and tag rewrites do not
   count.

A link in new content that resolves to nothing is a warning, not a refusal.

## Propose, show, wait

1. Call `change` with `action: propose` and the plan's fields.
2. Show the Change Preview in a few lines: each write with its op and title, the new
   tags, the link and tag rewrites, the documents absorbed, the warnings. Link the
   change document by its path, so the user can read every document in Obsidian and
   edit one before saying yes.
3. Stop and wait for the user's answer. Do not apply in the same turn: the tool refuses
   it. The user may also press Apply in Obsidian; then the change is applied when you
   read it again (`change` show).
4. On yes, call `change` with `action: apply` and the change's `id`. Say the commit in
   one line.
5. On no, call `change` with `action: reject`, the `id`, and the user's `reason`.

A change with no writes (the documents held nothing new) changes no document. Apply it
at once and say in one line that the documents are no longer pending.

## Conflicts

Apply refuses with `conflict` when a document changed since the proposal. Read the
document again, build the plan again from what is there now, and propose with
`supersedes` set to the old change's id.

## Undo

`change` undo restores the paths of one applied change from the commit before it. It
refuses when one of them changed since; then make the fix as a new change.

## Pending documents

A document of a type in `wikify` is pending until an applied change lists it in
`absorbs` with its current content. By default that is every source, every spec, and the
events of kind `completed`, `dropped`, and `note`. A spec edited after the wiki absorbed
it is pending again. `vault` status lists them; the wiki-sync skill absorbs them.
