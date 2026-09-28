
# Capture Request

> The input of [[source]] capture. One of three forms, each with an optional scope.

```yaml
# files waiting in the inbox
inbox: ["DINOv2.pdf", "meeting notes.md"]
scope: rep-h6t2vc

# text pasted in the conversation
text: "…"
title: "Vendor call, 2026-09-27"
scope: are-w4q8ze

# a snapshot of a linked repository at its head
repository: rep-h6t2vc
```

The output is a list of captured sources:

```yaml
captured:
  - ref: {Doc Ref}              # the new source page
    sha256: 3f9c1e2a…
    measure: "31 pages"
    chunks: [{Chunk}, …]
    duplicate: ""               # the id of the source already holding this sha256, if any
```

A file already captured (same sha256) makes no new source. Capture returns the old one with `duplicate` set, and still removes the inbox file.
