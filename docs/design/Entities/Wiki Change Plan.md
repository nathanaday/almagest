# Wiki Change Plan

> The writes one change will make. [[wiki-draft]] returns fragments; the skill joins them; [[change]] propose takes the whole.

```yaml
title: "Ingest the DINOv2 paper"      # short: the file name and the commit subject
notes: "…"                            # what the change does and why; the skipped subjects; becomes ## Notes
absorbs: [doc-p2x7nd]                 # the documents this change absorbs
work: ""                              # the stub or spec it serves, if any
supersedes: ""                        # a proposed change this one replaces
new_tags: false                       # true: allow tags no document holds, in tags: known mode
writes:
  - op: create
    type: topic
    kind: concept
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
    base: 5b1d0e9a                    # the hash the model read; optional
    fields: {description: "…", authority: primary}   # merged over the document's fields
    body: "…"                         # replaces the body below the lead callout; leave out to keep it
  - op: promote
    id: doc-c7v2kq                    # an open stub
    kind: concept
    title: "Motion scoring"           # optional: a new title
    fields: {description: "…", sources: [doc-p2x7nd]}
    body: "## Definition\n…"          # the stub's ## Idea is kept as ## Origin by code
  - op: rename
    id: doc-k3m9qa
    title: "Self-supervised representation learning"
  - op: remove
    id: doc-z2b7rf
    redirect: doc-k3m9qa              # optional: links to the removed document go here
  - op: confirm
    id: doc-m5r1ty
    base: 0c44e1b9
  - op: retag
    from: p3
    to: work/p3
```

- `rename` exists so code can rewrite the links ([[Changes#Link rewrites]]). A remove and a create would break them.
- `retag` exists so code can rewrite a tag everywhere at once ([[Changes#Tag rewrites]]).
- `fields` and `body` are apart, so code writes the fields it owns and checks the rest against the schema.
- A new document gives a type, a kind for a topic, and a title. Code mints the id; the path is always `wiki/documents/<title>.md`.
- `absorbs` is how code knows what is still [[Changes#Pending documents|pending]].
