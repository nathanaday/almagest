---
name: wiki-sync
description: "Absorb sources into the wiki through one pipeline: chunk, extract, match, draft, change. It serves every way knowledge enters: a captured file, a repository snapshot, a saved passage. Use for sync the wiki, absorb the pending sources, update the wiki from this source, what does the wiki still need to learn. Capturing files is wiki-ingest; changing topics that exist is wiki-edit."
---

# wiki-sync

A source is pending until an applied change absorbs it. This skill drains pending
sources: it cuts each into chunks, sends a worker to extract what each chunk says,
matches the subjects against the wiki in code, sends drafters to decide and write, and
proposes one change the user reviews. When wiki-ingest hands it a work document, every
step goes into that document, and the change fills it.

Tools: `vault`, `source` (chunks, read), `match`, `change`. Agents:
[wiki-extract](../../agents/wiki-extract.md), [wiki-draft](../../agents/wiki-draft.md).
References: [changes.md](../atlas/references/changes.md),
[pages.md](../atlas/references/pages.md).

## Procedure

1. **Sources.** Take the ids given, or the `pending` list from `vault`. One change holds
   at most about ten sources or forty chunks; more is several changes, one after another.
   From the same `vault` call, keep the tag list (every tag with its count), the
   `tagging` mode, and the vault's description. With a work document id, report each
   step below with `change` `action: progress`, one line each: "extracted 9 of 12
   chunks", "matched 31 subjects: 12 hits, 4 near, 15 new", "drafted 14 writes in 3
   slices".
2. **Chunks.** Call `source` with `action: chunks` and `doc` for each source.
3. **Extract.** Send one [wiki-extract](../../agents/wiki-extract.md) per chunk, in waves
   of at most eight. Give each the source id, the chunk index, the source's tags, the
   tag list, and the vault's description. A single small chunk, extract yourself:
   `source` with `action: read`, then the procedure of wiki-extract.
4. **Check.** Each Item Map names its chunk and is complete. Send a `partial` chunk
   again, narrower.
5. **Match.** Call `match` with every Item Map at once in `items`, so a subject named in
   three chunks is one subject.
6. **Slice.** Put every subject that hits or nears one topic into one slice, so two
   drafters never write one topic. At most eight subjects per slice.
7. **Draft.** Send one [wiki-draft](../../agents/wiki-draft.md) per slice, with the ids
   of the sources being absorbed, their tags, the tag list, and the `tagging` mode.
   Draft yourself when there are one or two slices.
8. **Assemble** the Wiki Change Plan:
   - every drafted write;
   - for each source: a modify of the source, with its `description`, `tags`,
     `authority`, `## Summary`, and `## Structure` (from the chunk summaries; keep the
     embed line at the top of the body);
   - `absorbs`: the source ids;
   - `new_tags: true` when a write adds a tag that no document holds, and the user
     agreed to it in `tagging: known` mode;
   - `title`: a short name ("Ingest the DINOv2 paper"); `notes`: what the change does,
     the triage lines from wiki-ingest, the coverage (every chunk read, any chunk
     partial), and every skipped subject with its reason;
   - a `why` on every write: one line that says why the change makes it.
   Past 25 new topics, keep the ones the most claims support and list the rest in the
   notes for a later run.
9. **Propose** the change: `change` with `action: propose`, and `id` set to the work
   document when one runs. When the sources make several changes, the first fills the
   work document and the others are new change documents.

When `progress` or `propose` refuses with "the user cancelled …; stop the work", stop at
once: send no more workers, and say in one line that the user cancelled the work.

## Gate

End the turn with one or two lines: the change document as a link, the count of writes,
and each new tag. The document's Summary and Notes hold the rest; do not repeat them in
the chat. The user decides once, in the document (Approve or Cancel), or with a yes in
the chat. On a yes in the chat, apply. The change tool refuses apply in the same turn as
the proposal. A change the user approves or cancels in Obsidian needs nothing more from
you.

A change with no writes (the sources held nothing new) needs no yes. Apply it and say
so in one line: the sources are no longer pending.

## Hand off

[wiki-map](../wiki-map/SKILL.md) when new topics landed under two child tags of one tag;
[wiki-review](../wiki-review/SKILL.md) after a large sync.
