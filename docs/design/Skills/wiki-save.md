
# wiki-save

> Keep something from the conversation in the wiki: an answer, a decision, a comparison, a finding. The citation is this session's document.

**Use for**: save this, keep this answer, file this decision, remember this in the wiki. Not for files; that is [[wiki-ingest]].

**Tools**: `source` capture, then [[wiki-sync]]. **References**: `references/changes.md`, `references/pages.md`.

## Procedure

1. Name exactly what to keep. When the user's "this" could mean two things, quote both and ask.
2. Write the passage to keep, in full: the answer, the decision and its reasons, and the documents the conversation relied on, as links.
3. Call `source` capture with `text` and a `title`. The source has `origin: pasted` and authority `synthetic`, and it records this session as its locator. The claims now cite a document that holds them.
4. Hand the new source to [[wiki-sync]]. Saving is then the same pipeline as every other way knowledge enters.

## Gate

The gate of [[wiki-sync]].

## Hand off

None.
