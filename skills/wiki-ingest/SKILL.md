---
name: wiki-ingest
description: "Triage what waits in ingest/, and capture what the wiki should learn from: each item goes to the wiki as a source, or to a note in scratchpad/ when it holds work or an idea. Use for ingest, process ingest/, process the inbox, add this file to the wiki, read and file this, batch ingest. Saving part of this conversation is wiki-save; an idea to act on later is a note in scratchpad/ (the atlas skill)."
---

# wiki-ingest

`ingest/` holds files and notes the user dropped in. A paper teaches the wiki; a note
that asks for work or holds an idea goes to `scratchpad/`. This skill sorts the items,
captures the sources with their tags, moves the work notes, and hands the sources to
wiki-sync, which writes the topics.

Tools: `vault`, `source` (capture). References:
[changes.md](../atlas/references/changes.md).

## Procedure

1. Call `vault` for the files in `ingest/`, the tag list, and the vault's `tagging`
   mode. For text the user pasted, go to step 4 with `text` and a `title`. A note with
   no type in `source-core/documents/` (lint's `untyped`) waits to be ingested too:
   move it into `ingest/` before step 4, because capture takes only names in `ingest/`.
2. Triage each item:
   - a document to learn from (a paper, notes, an article, a design) → **wiki**;
   - a note that asks for work or holds an idea ("fix the login timeout", "idea: …")
     → **scratchpad**;
   - both → capture it, then write the work part in a note in `scratchpad/` that links
     the new source;
   - a note that asks to read something later that is not in `ingest/` →
     **scratchpad**;
   - unclear → ask.
   Give each wiki item its tags: the tags that exist and fit its subject, from the tag
   list in the session-start context or `vault`. Use a tag that exists before you make a
   new one. Name each new tag.
3. Show the triage as one table, with the destination and the tags of each item, and
   wait for one yes. In `tagging: known` mode, the yes covers the new tags the table
   names.
4. Call `source` with `action: capture` for the wiki items: `ingest` (the names), or
   `text` and `title`, and `tags`. One call takes one tag list, so put items with the
   same tags in one call. Set `new_tags: true` only when the user agreed to a new tag.
   A file captured before comes back with `duplicate` set and leaves `ingest/`.
5. Move every scratchpad item from `ingest/` to `scratchpad/` with `mv`, and keep its
   name.
   Never move or capture a file from `journals/`: the journals are the user's.
6. Say the plan in one line: the sources, their size, the chunks, and the workers
   wiki-sync will send. Ask first only when the total is past about 600 pages.

No step checks the vault's git state, and you never report it: every write tool commits
the user's hand edits as a snapshot before it writes.

## Gate

The triage table (step 3). The topics come later, through the gate of wiki-sync.

## Hand off

[wiki-sync](../wiki-sync/SKILL.md), with the captured source ids.
