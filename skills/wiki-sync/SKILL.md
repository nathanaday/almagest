---
name: wiki-sync
description: "Absorb documents into the wiki through one pipeline: chunk, extract, match, draft, change. It serves every way knowledge enters: a captured source, a repository snapshot, the spec and the verification of a verified thread, a closed chord, a saved passage. Use for sync the wiki, absorb the pending documents, update the wiki from this work, wikify this, what does the wiki still need to learn. Capturing files is wiki-ingest; changing topics that exist is wiki-edit."
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

1. **Documents.** Take the ids given, or the `pending` list from `vault`. Pending
   documents are sources, the spec and the passing verification of a verified thread,
   a chord whose threads are closed, and events of kind dropped and note. Sources come
   first, then events, then specs with their verifications. A spec and its verification
   go into one change: together they close their thread. One change holds at most about ten
   documents or forty chunks; more is several changes, one after another. From the
   same `vault` call, keep the tag list (every tag with its count), the `tagging` mode,
   and the vault's description.
2. **Chunks.** Call `source` with `action: chunks` and `doc` for each document.
3. **Extract.** Send one [wiki-extract](../../agents/wiki-extract.md) per chunk, in waves
   of at most eight. Give each the document id, the chunk index, the document's tags,
   the tag list, and the vault's description. A single small chunk, extract yourself:
   `source` with `action: read`, then the procedure of wiki-extract. For a
   verification, the findings with the outcome `knowledge` and `## Notes` are the first
   things to read.
4. **Check.** Each Item Map names its chunk and is complete. Send a `partial` chunk
   again, narrower.
5. **Match.** Call `match` with every Item Map at once in `items`, so a subject named in
   three chunks is one subject.
6. **Slice.** Put every subject that hits or nears one topic into one slice, so two
   drafters never write one topic. At most eight subjects per slice.
7. **Draft.** Send one [wiki-draft](../../agents/wiki-draft.md) per slice, with the ids
   of the documents being absorbed, their tags, the tag list, and the `tagging` mode.
   Draft yourself when there are one or two slices.
8. **Assemble** the Wiki Change Plan:
   - every drafted write;
   - for each source: a modify of the source, with its `description`, `tags`,
     `authority`, `## Summary`, and `## Structure` (from the chunk summaries; keep the
     embed line at the top of the body);
   - `absorbs`: the document ids; `work`: the thread or the chord, when every document
     serves one;
   - `new_tags: true` when a write adds a tag that no document holds, and the user
     agreed to it in `tagging: known` mode;
   - `title`: a short name ("Ingest the DINOv2 paper"); `notes`: what the change does,
     and every skipped subject with its reason.
   Past 25 new topics, keep the ones the most claims support and list the rest in the
   notes for a later run.
9. **Propose** the change: `change` with `action: propose`.

## Gate

Show the Change Preview and link the change document, so the user can read the topics
in Obsidian and edit one before the yes. Say the coverage: every chunk read, any chunk
partial, the subjects skipped, and each new tag. Wait for the yes, then apply. The
change tool refuses apply in the same turn as the proposal.

A change with no writes (the documents held nothing new) needs no yes, unless it absorbs
a spec, a verification, or a chord: that change closes a thread or a chord, and the
change tool waits for the user's answer. Apply the first kind and say so in one line:
the documents are no longer pending.

## Hand off

[wiki-map](../wiki-map/SKILL.md) when new topics landed under two child tags of one tag;
[wiki-review](../wiki-review/SKILL.md) after a large sync.
