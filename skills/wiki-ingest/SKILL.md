---
name: wiki-ingest
description: "Triage what waits in ingest/, and capture what the wiki should learn from: each item goes to the wiki as a source, or to a note in scratchpad/ when it holds work or an idea. Use for ingest, process ingest/, process the inbox, add this file to the wiki, read and file this, batch ingest. Saving part of this conversation is wiki-save; an idea to act on later is a note in scratchpad/ (the almagest skill)."
---

# wiki-ingest

`ingest/` holds files and notes the user dropped in. A paper teaches the wiki; a note
that asks for work or holds an idea goes to `scratchpad/`. This skill sorts the items,
captures the sources with their tags, moves the work notes, and hands the sources to
wiki-sync, which writes the topics. One work document records the ingest from the first
step to the decision; the user watches it in Obsidian and decides once, at the end.

Tools: `vault`, `change` (start, progress), `source` (capture). References:
[changes.md](../almagest/references/changes.md).

## Procedure

1. Call `vault` for the files in `ingest/`, the tag list, and the vault's `tagging`
   mode. For text the user pasted, use `text` and a `title` in step 5. A note with no
   type in `source-core/documents/` (lint's `untyped`) waits to be ingested too: move it
   into `ingest/` first, because capture takes only names in `ingest/`.
2. **The work document.** When the request names one (the palette's message says "Your
   work document is [[…]] (<id>)"), use its id. Otherwise call `change` with
   `action: start`, `kind: ingest`, and `files` (the names in `ingest/` that the request
   takes). Report each later step with `change` `action: progress`, one line each.
3. Triage each item:
   - a document to learn from (a paper, notes, an article, a design) → **wiki**;
   - a note that asks for work or holds an idea ("fix the login timeout", "idea: …")
     → **scratchpad**;
   - both → capture it, then write the work part in a note in `scratchpad/` that links
     the new source;
   - a note that asks to read something later that is not in `ingest/` →
     **scratchpad**.
   Give each wiki item its tags: the tags that exist and fit its subject. Use a tag that
   exists before you make a new one.
4. Record the triage and go on; do not wait for a yes. Report it as one progress line
   ("triaged 5 items: 3 to the wiki, 2 to scratchpad/"), and keep one line per item
   (its destination and tags) for the change's notes. Ask the user only in these cases,
   with one question for every such item:
   - an item that fits no class above;
   - in `tagging: known` mode, an item that needs a tag no document holds.
5. Call `source` with `action: capture` for the wiki items: `ingest` (the names), or
   `text` and `title`, and `tags`. One call takes one tag list, so put items with the
   same tags in one call. Set `new_tags: true` only when the user agreed to a new tag.
   A file captured before comes back with `duplicate` set and leaves `ingest/`. Report
   "captured <titles>".
6. Move every scratchpad item from `ingest/` to `scratchpad/` with `mv`, and keep its
   name. Report "moved <names> to scratchpad/".
   Never move or capture a file from `journals/`: the journals are the user's.
7. Ask first only when the sources total more than about 600 pages. Otherwise go on.

When no item goes to the wiki, end the work document with `change` `action: reject` and
the reason ("every item went to scratchpad/"), and say so in one line.

When `progress` or `propose` refuses with "the user cancelled …; stop the work", stop at
once and say in one line that the user cancelled the ingest.

No step checks the vault's git state, and you never report it: every write tool commits
the user's hand edits as a snapshot before it writes.

## Gate

None in the chat, except the questions of step 4. The topics wait for the user's
decision in the work document, through the gate of wiki-sync.

## Hand off

[wiki-sync](../wiki-sync/SKILL.md), with the captured source ids, the work document's id,
and the triage lines for the change's notes.
