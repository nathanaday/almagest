# Capture Request

> The input of [[source]] capture. One of three forms, each with optional tags and an optional stub to resolve.

```yaml
# files waiting in the inbox
inbox: ["DINOv2.pdf", "meeting notes.md"]
tags: [ml/self-supervised, paper]

# text pasted in the conversation
text: "…"
title: "Vendor call, 2026-09-27"
tags: [work/p3, meeting]

# a snapshot of a linked repository at its head
repository: doc-h6t2vc

# on any form
resolves: doc-c7v2kq          # a stub that asked for this source ("read the DINOv2 paper")
new_tags: false               # true: allow a tag no document holds, in tagging: known mode
```

The output is a list of captured sources:

```yaml
captured:
  - ref: {Doc Ref}              # the new source
    sha256: 3f9c1e2a…
    measure: "31 pages"
    chunks: [{Chunk}, …]
    duplicate: ""               # the id of the source already holding this sha256, if any
```

A file already captured (same sha256) makes no new source. Capture returns the old one with `duplicate` set, and still removes the inbox file.
