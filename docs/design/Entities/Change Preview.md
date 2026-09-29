
# Change Preview

> What [[change]] returns about one change document: enough to preview it in the chat.

```yaml
ref: {Doc Ref}                        # the change document
status: proposed
counts: {create: 3, modify: 1, rename: 0, remove: 0, link_rewrites: 2}
writes:
  - op: create
    title: "Self-supervised learning"
    path: wiki/work/p3/concepts/Self-supervised learning.md   # where the page lands
    lines: "+41"
  - op: modify
    title: DINOv2
    path: wiki/work/p3/p3-cloud/sources/DINOv2.md
    lines: "+28 −3"
    note: "moves from wiki/work/p3/sources/DINOv2.md"         # a new scope moves the page
link_rewrites: [{Doc Ref}]              # documents outside wiki/ whose links the change rewrites
folders:                              # scope folders that move, with the files each carries
  - {from: wiki/work/p3, to: wiki/work/p3 product, files: 41}
absorbs: [{Doc Ref}]
warnings:
  - "Self-supervised learning: the link [[ViT]] resolves to nothing"
commit: ""                            # after apply: the commit's hash
```

The skill shows this preview, links the change document for review in Obsidian, and waits for the user's yes. The change tool refuses apply until the user has had a turn ([[Changes#The gate]]).
