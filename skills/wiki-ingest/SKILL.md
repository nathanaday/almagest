---
name: wiki-ingest
description: "Triage what waits in the inbox, and capture what the wiki should learn from: each item goes to the wiki as a source, to a stub, or both. Use for ingest, process the inbox, add this file to the wiki, read and file this, batch ingest. Saving part of this conversation is wiki-save; an idea to act on later is thread-stub."
---

# wiki-ingest

The inbox holds files and notes the user dropped in. A paper teaches the wiki; a note
that asks for work or holds an idea becomes a stub. This skill sorts the items,
captures the sources with their tags, plants the stubs, and hands the sources to
wiki-sync, which writes the topics.

Tools: `vault`, `source` (capture), `thread` (stub). References:
[changes.md](../atlas/references/changes.md),
[threads.md](../atlas/references/threads.md).

## Procedure

1. Call `vault` for the inbox, the tag list, and the vault's `tagging` mode. For text
   the user pasted, go to step 4 with `text` and a `title`. A note with no type in
   `wiki/documents/` (lint's `untyped`) is an inbox item too: move it into `inbox/`
   before step 4, because capture and `thread` stub take only names in `inbox/`.
2. Triage each item:
   - a document to learn from (a paper, notes, an article, a design) → **wiki**;
   - a note that asks for work or holds an idea ("fix the login timeout", "idea: …")
     → **stub**;
   - both → capture it, then plant a stub that links the new source (without `inbox`,
     because capture already removed the file);
   - a note that asks to read something later that is not in the inbox → **stub**; the
     capture of that file resolves it later;
   - unclear → ask.
   Give each item its tags: the tags that exist and fit its subject, from the tag list
   in the session-start context or `vault`. Use a tag that exists before you make a new
   one. Name each new tag.
3. Show the triage as one table, with the destination and the tags of each item, and
   wait for one yes. In `tagging: known` mode, the yes covers the new tags the table
   names.
4. Call `source` with `action: capture` for the wiki items: `inbox` (the names), or
   `text` and `title`, and `tags`. One call takes one tag list, so put items with the
   same tags in one call. Set `new_tags: true` only when the user agreed to a new tag.
   When a stub with no spec asked for the source, set `resolves` to the stub, in a call of its
   own. A file captured before comes back with `duplicate` set and leaves the inbox.
5. Call `thread` with `action: stub` for every stub item: `text` is the note's words as
   given, `title` a short name, `tags`, and `inbox` its name, so the note leaves the
   inbox in the same commit.
6. Say the plan in one line: the sources, their size, the chunks, and the workers
   wiki-sync will send. Ask first only when the total is past about 600 pages.

No step checks for a clean git state: every write tool commits a dirty vault as a
snapshot before it writes.

## Gate

The triage table (step 3). The topics come later, through the gate of wiki-sync.

## Hand off

[wiki-sync](../wiki-sync/SKILL.md), with the captured source ids.
