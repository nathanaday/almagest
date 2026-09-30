# wiki-ingest

> Triage what waits in the inbox, and capture what the wiki should learn from. Each inbox item goes to its destination: the wiki (as a source), a stub, or both.

**Use for**: ingest, process the inbox, add this file to the wiki, read and file this, batch ingest. Saving part of this conversation is [[wiki-save]].

**Tools**: `vault`, `source` capture, `work` stub. **References**: `references/changes.md`, `references/work.md`.

## Procedure

1. Call `vault` for the inbox. For pasted text, go to step 4 with `text` and a title. Untyped notes in `wiki/documents/` (lint's `untyped`) are inbox items too.
2. Triage each item:
   - a document to learn from (a paper, notes, an article, a design) → **wiki**;
   - a note that asks for work or holds an idea ("fix the login timeout", "idea: …") → **stub**;
   - both → capture it, then plant a stub that links the new source (without `inbox`, because capture already removed the file);
   - a note that asks to read something later that is not in the inbox → **stub**, resolved by the capture when the file comes;
   - unclear → ask.
   Give each item its tags: the tags that exist and fit its subject, from the tag list in the session-start context or `vault`. Name each new tag.
3. Show the triage as one table, with the destination and the tags of each item, and wait for one yes. In `tagging: known` mode, the yes covers the new tags it names.
4. Call `source` capture once for every wiki item, with `new_tags` when the user agreed to one. When an open stub asked for the source, set `resolves`.
5. Call `work` stub for every stub item, with `inbox` set, so the note leaves the inbox in the same commit.
6. Tell the plan in one line: the sources, their size, the chunks, and the workers. Ask first only when the total is past about 600 pages.

No step checks for a clean git state: every write tool commits a dirty vault as a snapshot before it writes.

## Gate

The triage table (step 3). The topics come later, through the gate of [[wiki-sync]].

## Hand off

[[wiki-sync]], with the captured source ids.
