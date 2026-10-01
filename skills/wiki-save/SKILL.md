---
name: wiki-save
description: "Keep something from this conversation in the wiki: an answer, a decision, a comparison, a finding, cited to this session. Use for save this, keep this answer, file this decision, remember this in the wiki. Files are wiki-ingest; an idea to act on later is thread-stub."
---

# wiki-save

A saved passage enters the wiki the way every other document does: as a source that a
change absorbs. The source is the passage itself, so the topics cite a document that
holds the claims, and its locator names this session.

Tools: `source` capture. Skills: [wiki-sync](../wiki-sync/SKILL.md). References:
[changes.md](../atlas/references/changes.md), [pages.md](../atlas/references/pages.md).

## Procedure

1. Name exactly what to keep. When the user's "this" could mean two things, quote both
   and ask.
2. Write the passage to keep, in full: the answer, the decision and its reasons, and the
   documents the conversation relied on, as links.
3. Call `source` with `action: capture`, `text`, a `title`, `tags` (the tags that exist
   and fit its subject; name a new one to the user, and set `new_tags: true` in
   `tagging: known` mode only after the user agreed), and `locator` set to this
   session's document as a link (the session-start context names it:
   `[[2026-09-27 1432 a1b2c3]]`). The source gets `origin: pasted`.
4. Hand the new source to wiki-sync. In its change, the modify of the source sets
   `authority: synthetic`.

## Gate

The gate of [wiki-sync](../wiki-sync/SKILL.md).

## Hand off

None.
