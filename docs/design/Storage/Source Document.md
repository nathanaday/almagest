# Source Document

An outside document the vault captured: a paper, a PDF, an image, a set of meeting notes, a web page, a snapshot of a repository. The source document is the card for the original, as a file page of an encyclopedia is: what the file is, where it came from, when, how much to trust it, and what it says. The original sits in `wiki/assets/` and never changes.

Capture writes the source with the fields code owns. The ingest change writes the rest: the description, the authority, the summary, and the structure.

## Fields

```yaml
---
id: doc-p2x7nd
type: source
description: "The DINOv2 paper: self-supervised vision features at scale."
tags: [ml/self-supervised, vision, paper]
aliases: [DINOv2 paper]
created: 2026-09-27T14:40:12
updated: 2026-09-27T14:51:03
refreshed: 2026-09-27T14:40:12
authority: primary                  # official | primary | secondary | community | synthetic | unknown
authors: [Maxime Oquab, Timothée Darcet]    # optional
published: 2023-04-14               # optional: the date the original carries
# owned by code:
status: absorbed                    # pending | absorbed
file: "[[doc-p2x7nd.pdf]]"          # the original in wiki/assets/
media: pdf                          # pdf | image | markdown | text | office | audio | video | other
sha256: 3f9c1e2a…
origin: inbox                       # inbox | pasted | url | repository
locator: "DINOv2.pdf"               # the inbox file name, the URL, or <repository>@<commit>
measure: "31 pages"                 # pages, lines and sections, or the pixel size
captured: 2026-09-27T14:40:12
from: ""                            # the stub that asked for it, when capture resolved one
---
```

- The model sets `authority`, `authors`, and `published` in the ingest change. Capture writes `authority: unknown`.
- Capture takes the title from the file name without its extension, or from the `title` given. A repository snapshot takes `<repository> @ <short commit>` (`p3-cloud @ 4ac19e2`), so it never takes the repository document's title.
- Capture writes `Captured, not yet ingested` as the description. The ingest change replaces it.

## Lead callout

```markdown
> [!source] PDF · 31 pages · primary
> Oquab, Darcet, and others · published 2023-04-14
> Captured 2026-09-27 from `DINOv2.pdf` (inbox) · absorbed by [[2026-09-27 Ingest the DINOv2 paper]]
```

A pending source says `pending: not yet ingested` in place of the change. A URL source links its locator.

## Body

1. The lead callout.
2. **The original**, code's: an embed, `![[doc-p2x7nd.pdf]]`, for a PDF, an image, audio, video, or markdown of at most 200 lines. For anything else, a link: `[[doc-p2x7nd.docx|Open the original (docx, 84 KB)]]`. A repository snapshot is always a link, since it is long.
3. `## Summary`: what the document says, in a paragraph or two. The model's, from the ingest.
4. `## Structure`: one short line per part of the document (a section, a chapter, a page range), each linking the documents that grew from it. The model's, from the chunk summaries.
5. `## Notes`: yours.

The documents that cite the source are its backlinks, so no section lists them.

## Rules

- The captured file is immutable. A changed original is a new capture and a new source.
- A file with the sha256 of a source that exists makes no new source. Capture returns the old one as a duplicate, and still removes the inbox file.
- A repository snapshot is a source with `origin: repository`. Its file is a markdown report that capture writes: the tree to three levels, the instruction files, the manifests, the docs, and every TODO and FIXME line with its location, at one commit.
- Capture may resolve a stub (`resolves: <stub>`): the source gets `from`, the stub gets `became`, and a `resolved` event goes in the capture commit ([[Stub#Becoming something]]).
- `status` is `pending` until an applied change records the source's sha256 in `## Absorbed`, then `absorbed` ([[Changes#Pending documents]]).
