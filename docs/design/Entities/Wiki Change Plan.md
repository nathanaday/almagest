
# Wiki Change Plan

> The writes one change will make. [[wiki-draft]] returns fragments; the skill joins them; [[change]] propose takes the whole.

```yaml
title: "Ingest the DINOv2 paper"      # short: the file name and the commit subject
notes: "…"                            # what the change does and why; the skipped subjects; becomes ## Notes
absorbs: [src-p2x7nd]                 # the documents this change absorbs
thread: ""                            # the thread it serves, if any
supersedes: ""                        # a proposed change this one replaces
writes:
  - op: create
    type: concept
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
    base: 5b1d0e9a                    # the hash the model read; optional
    fields: {description: "…"}        # merged over the page's fields
    body: "…"                         # replaces the body; leave out to keep it
  - op: rename
    id: con-k3m9qa
    title: "Self-supervised representation learning"
  - op: remove
    id: con-z2b7rf
    redirect: con-k3m9qa              # optional: links to the removed page go here
```

- `rename` exists so code can rewrite the links ([[Changes#Link rewrites]]). A remove and a create would break them.
- `fields` and `body` are apart, so code writes the fields it owns and checks the rest against the schema.
- A new page gives a type and a title. Code mints the id and routes the path.
- `absorbs` is how code knows what is still [[Changes#Pending documents|pending]].
