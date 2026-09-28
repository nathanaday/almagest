---
name: wiki-save
description: "Keep something from this conversation in the wiki: an answer, a decision, a comparison, a finding, cited to this session. Use for save this, keep this answer, file this decision, remember this in the wiki. Files and URLs are wiki-ingest."
---

# wiki-save

A saved passage enters the wiki the way every other document does: as a source that a
change absorbs. The source is the passage itself, so the pages cite a document that
holds the claims, and its locator names this session.

Tools: `source` capture. Skills: [wiki-sync](../wiki-sync/SKILL.md). References:
[changes.md](../atlas/references/changes.md), [pages.md](../atlas/references/pages.md).

## Procedure

1. Name exactly what to keep. When the user's "this" could mean two things, quote both
   and ask.
2. Write the passage to keep, in full: the answer, the decision and its reasons, and the
   documents the conversation relied on, as links.
3. Call `source` with `action: capture`, `text`, a `title`, the `scope` the passage
   belongs to, and `locator` set to this session's document as a link (the opening
   context names it: `[[2026-09-27 1432 a1b2c3]]`).
4. Hand the new source to wiki-sync. In its change, set the source's `authority` to
   `synthetic`.

## Gate

The gate of [wiki-sync](../wiki-sync/SKILL.md).

## Hand off

None.
