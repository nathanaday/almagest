---
name: wiki-ingest
description: "Triage what waits in the inbox, and capture what the wiki should learn from: each item goes to the wiki as a source, to a thread as a stub, or both. Use for ingest, process the inbox, add this file to the wiki, read and file this, batch ingest. Saving part of this conversation is wiki-save."
---

# wiki-ingest

The inbox holds files and notes the user dropped in. A paper teaches the wiki; a note
that asks for work belongs to a thread. This skill sorts them, captures the sources,
opens the stubs, and hands the sources to wiki-sync, which writes the pages.

Tools: `vault`, `source` capture, `thread` open. References:
[changes.md](../atlas/references/changes.md).

## Procedure

1. Call `vault` for the inbox. For text the user pasted, go to step 4 with `text` and a
   `title`.
2. Triage each item:
   - a document to learn from (a paper, notes, an article, a design) → **wiki**;
   - a note that asks for work ("fix the login timeout", "idea: …") → **thread**;
   - both → capture it, then open a stub that links the new source page (without
     `inbox`, because capture already removed the file);
   - unclear → ask.
   Give each item a scope: the repository or area its subject belongs to, from its name
   and content against the scope descriptions (`search` with
   `types: [repository, area]`), or the vault.
3. Show the triage as one table with the destination and the scope of each item, and
   wait for one yes.
4. Call `source` with `action: capture`, the `inbox` names (or `text` and `title`), and
   `scope`. Group the items by scope, one call per scope. A file captured before is
   returned with `duplicate` set and leaves the inbox.
5. Call `thread` with `action: open` for every thread item: `text` is the note's words,
   and `inbox` is its name, so the note leaves the inbox in the same commit.
6. Say the plan in one line: the sources, their size, the chunks, and the workers
   wiki-sync will send. Ask first only when the total is past about 600 pages.

No step checks for a clean git state: every write tool commits hand edits as a snapshot
before it writes.

## Gate

The triage table (step 3). The pages come later, through the gate of wiki-sync.

## Hand off

[wiki-sync](../wiki-sync/SKILL.md), with the captured source ids.
