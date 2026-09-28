---
name: wiki-sync
description: "Absorb documents into the wiki through one pipeline: chunk, extract, match, draft, change. It serves every way knowledge enters: a captured source, a repository snapshot, a spec, a receipt, a saved passage. Use for sync the wiki, absorb the pending documents, update the wiki from this thread, wikify this, what does the wiki still need to learn."
---

# wiki-sync

A document is pending until an applied change absorbs it. This skill drains pending
documents: it cuts each into chunks, sends a worker to extract what each chunk says,
matches the subjects against the wiki in code, sends drafters to decide and write, and
proposes one change the user reviews.

Tools: `vault`, `source` (chunks, read), `match`, `change`. Agents:
[wiki-extract](../../agents/wiki-extract.md), [wiki-draft](../../agents/wiki-draft.md).
References: [changes.md](../atlas/references/changes.md),
[pages.md](../atlas/references/pages.md).

## Procedure

1. **Documents.** Take the ids given, or the `pending` list from `vault`. Sources come
   first, then receipts, then specs. One change holds at most about ten documents or
   forty chunks; more is several changes, one after another.
2. **Chunks.** Call `source` with `action: chunks` for each document.
3. **Extract.** Send one [wiki-extract](../../agents/wiki-extract.md) per chunk, in waves
   of at most eight, each with the document id, the chunk index, the scope, and the
   vault's description. A single small chunk, extract yourself: `source` read, then
   the procedure of wiki-extract. For a receipt, `## Learned` is the first thing to
   read.
4. **Check.** Each Item Map names its chunk and is complete. Send a `partial` chunk
   again, narrower.
5. **Match.** Call `match` with every Item Map at once, so a subject named in three
   chunks is one subject.
6. **Slice.** Put every subject that hits or nears one page into one slice, so two
   drafters never write one page. At most eight subjects per slice.
7. **Draft.** Send one [wiki-draft](../../agents/wiki-draft.md) per slice, or draft
   yourself when there are one or two slices.
8. **Assemble** the plan:
   - every drafted write;
   - for each source: a modify of its source page with its `description`,
     `authority`, `## Summary`, and `## Structure` (from the chunk summaries; keep the
     embed line at the top of the body);
   - `absorbs`: the document ids; `thread`: the thread, when every document belongs to
     one;
   - `title`: a short name ("Ingest the DINOv2 paper"); `notes`: what the change does,
     and every skipped subject with its reason.
   Past 25 new pages, keep the ones the most claims support and list the rest in the
   notes for a later run.
9. **Propose** the change.

## Gate

Show the Change Preview and link the change document, so the user can read the pages
in Obsidian and edit one before the yes. Say the coverage: every chunk read, any chunk
partial, the subjects skipped. Wait for the yes, then apply. The guard refuses apply in
the same turn as the proposal.

A change with no writes (the documents held nothing new) needs no yes. Apply it and say
so in one line: the documents are no longer pending.

## Hand off

[wiki-rollup](../wiki-rollup/SKILL.md) when new pages landed in two sibling scopes;
[wiki-review](../wiki-review/SKILL.md) after a large sync.
