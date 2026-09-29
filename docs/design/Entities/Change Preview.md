# Change Preview

> What [[change]] returns about one change document: enough to preview it in the chat.

```yaml
ref: {Doc Ref}                        # the change document
status: proposed
counts: {create: 3, modify: 1, promote: 0, rename: 0, remove: 0, confirm: 0, retag: 0, link_rewrites: 2, tag_rewrites: 0}
writes:
  - op: create
    title: "Self-supervised learning"
    type: topic
    kind: concept
    tags: [ml/self-supervised, vision]
    lines: "+41"
  - op: modify
    title: DINOv2
    type: source
    lines: "+28 −3"
    note: "tags: + ml/self-supervised"
rewrites: [{Doc Ref}]                 # documents outside the writes whose links or tags the change rewrites
new_tags: [ml/self-supervised]        # tags no document held before
absorbs: [{Doc Ref}]
warnings:
  - "Self-supervised learning: the link [[ViT]] resolves to nothing"
commit: ""                            # after apply: the commit's hash
```

The skill shows this preview, names every new tag, links the change document for review in Obsidian, and waits for the user's yes. The change tool refuses apply until the user has had a turn ([[Changes#The gate]]).
