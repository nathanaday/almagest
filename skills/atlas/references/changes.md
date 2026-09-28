# The change contract

A change is the only way the wiki changes. You build a plan, `change` propose writes it
as a change document, the user reads it and says yes, and `change` apply makes one git
commit. A hook refuses Write and Edit under `wiki/`, and refuses apply in the turn that
proposed it.

## The plan

```yaml
title: "Ingest the DINOv2 paper"      # short: the file name and the commit subject
notes: "…"                            # what the change does and why; every skipped subject with its reason
absorbs: [src-p2x7nd]                 # the documents the change absorbs (sources, specs, receipts)
thread: ""                            # the thread it serves, if any
supersedes: ""                        # a proposed change this one replaces
writes:
  - op: create
    type: concept                     # area, repository, concept, entity, or policy
    title: "Self-supervised learning"
    fields:                           # the type's fields; links as ids or titles
      scope: are-w4q8ze
      description: "Training a model on data with no labels, from a signal in the data itself."
      aliases: [SSL]
      status: stable
      sources: [src-p2x7nd]
    body: "## Definition\n…"
  - op: modify
    id: src-p2x7nd
    fields: {description: "…"}        # merged over the page's fields; null removes one
    body: "…"                         # replaces the body; leave it out to keep the body
  - op: rename
    id: con-k3m9qa
    title: "Self-supervised representation learning"
  - op: remove
    id: con-z2b7rf
    redirect: con-k3m9qa              # links to the removed page go here
```

- Give a type and a title for a new page. Code mints the id, routes the path, and writes
  `id`, `type`, `created`, and `updated`. It drops any field it owns and warns.
- Name a page that exists by its id. Give `base` only when you read the page's hash;
  code records it otherwise, and apply refuses when the file changed since.
- Use rename, never a remove and a create: code rewrites every link to the old title,
  in every document of the vault, in the same commit.
- A source page comes from `source` capture. A change modifies it; it never creates one.

## Validation

`change` propose and apply refuse a plan that breaks a rule, and name the write and the
rule. Fix the plan and propose again.

1. Every write is a page under `wiki/`, never under `wiki/sources/files/`.
2. A title and every alias is unique in the vault, without case.
3. A page matches its type (see [pages.md](pages.md)): the required fields exist and hold
   allowed values; `scope` and `parent` name scope pages; `sources` name documents that
   exist or that the same change creates; a repository's `path` is the root of a git
   work tree outside the vault that no other page holds.
4. `absorbs` names documents of a type the vault's `wikify` setting lists.
5. At most 100 writes. Split a larger change into several.

A link in new content that resolves to nothing is a warning, not a refusal.

## Propose, show, wait

1. Call `change` with `action: propose` and the plan.
2. Show the Change Preview in a few lines: each write with its op and title, the link
   rewrites, the documents absorbed, the warnings. Link the change document by its path,
   so the user can read every page in Obsidian and edit one before saying yes.
3. Stop and wait for the user's answer. Do not apply in the same turn: the guard refuses
   it. The user may also press Apply in Obsidian; then the change is applied when you
   read it again.
4. On yes, call `change` with `action: apply` and the change's id. Say the commit in one
   line.
5. On no, call `change` with `action: reject`, the id, and the user's reason.

A change with no writes (the documents held nothing new) changes no page. Apply it at
once and say in one line that the documents are no longer pending.

## Conflicts

Apply refuses with `conflict` when a page changed since the proposal. Read the page
again, build the plan again from what is there now, and propose with `supersedes` set to
the old change's id.

## Undo

`change` undo restores the paths of one applied change from the commit before it. It
refuses when one of them changed since; then make the fix as a new change.

## Pending documents

A document of a type in `wikify` (sources, specs, and receipts by default) is pending
until an applied change lists it in `absorbs`. A spec edited after the wiki absorbed it
is pending again. `vault` status lists them; the wiki-sync skill absorbs them.
