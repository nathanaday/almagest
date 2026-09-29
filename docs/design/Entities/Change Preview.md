
# Change Preview

> What [[change]] returns about one change document: enough to preview it in the chat.

```yaml
ref: {Doc Ref}                        # the change document
status: proposed
counts: {create: 3, modify: 1, rename: 0, remove: 0, link_rewrites: 2}
writes:
  - op: create
    title: "Self-supervised learning"
    path: wiki/concepts/Self-supervised learning.md
    lines: "+41"
  - op: modify
    title: DINOv2
    path: wiki/sources/DINOv2.md
    lines: "+28 −3"
link_rewrites: [{Doc Ref}]              # documents outside wiki/ whose links the change rewrites
absorbs: [{Doc Ref}]
warnings:
  - "Self-supervised learning: the link [[ViT]] resolves to nothing"
commit: ""                            # after apply: the commit's hash
```

The skill shows this preview, links the change document for review in Obsidian, and waits for the user's yes. The change tool refuses apply until the user has had a turn ([[Changes#The gate]]).
