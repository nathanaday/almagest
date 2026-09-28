
# wiki-ingest

> Triage what waits in the inbox, and capture what the wiki should learn from. Each inbox item goes to its destination: the wiki (as a source), a thread (as a stub), or both.

**Use for**: ingest, process the inbox, add this file to the wiki, read and file this, batch ingest.

**Tools**: `vault`, `source` capture, `thread` open. **References**: `references/changes.md`. See [[wiki-ingest.canvas|the ingest flow]].

## Procedure

1. Call `vault` for the inbox. For pasted text, go to step 4 with `text` and a title.
2. Triage each item:
   - a document to learn from (a paper, notes, an article, a design) → **wiki**;
   - a note that asks for work ("fix the login timeout", "idea: …") → **thread**;
   - both → capture it, then open a stub that links the new source page (without `inbox`, because capture already removed the file);
   - unclear → ask.
   Give each item a scope: the repository or area its subject belongs to, from its name and content against the scope descriptions, or the vault.
3. Show the triage as one table, with the destination and scope of each item, and wait for one yes.
4. Call `source` capture once for every wiki item.
5. Call `thread` open for every thread item, with `inbox` set, so the note leaves the inbox in the same commit.
6. Tell the plan in one line: the sources, their size, the chunks, and the workers. Ask first only when the total is past about 600 pages.

No step checks for a clean git state: every write tool commits a dirty vault as a snapshot before it writes.

## Gate

The triage table (step 3). The pages come later, through the gate of [[wiki-sync]].

## Hand off

[[wiki-sync]], with the captured source ids.
