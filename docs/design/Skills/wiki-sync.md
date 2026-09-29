# wiki-sync

> Absorb documents into the wiki through one pipeline: chunk, extract, match, draft, change. It serves every way knowledge enters: a captured source, a repository snapshot, a spec, a completed event, a saved passage.

**Use for**: sync the wiki, absorb the pending documents, update the wiki from this work, wikify this, what does the wiki still need to learn.

**Tools**: `vault`, `source` (chunks, read), `match`, `change`. **Agents**: [[wiki-extract]], [[wiki-draft]]. **References**: `references/changes.md`, `references/pages.md`.

## Procedure

1. **Documents.** Take the ids given, or the `pending` list from `vault`. Sources come first, then completed events, then specs. One change holds at most about ten documents or forty chunks; more is several changes, one after another.
2. **Chunks.** Call `source` chunks for each document.
3. **Extract.** Send one [[wiki-extract]] per chunk, in waves of at most eight, each with the tag vocabulary. A single small chunk, you extract yourself. For a completed event, `## Learned` is the first thing to read.
4. **Check.** Each Item Map names its chunk and is complete. Send a `partial` chunk again, narrower.
5. **Match.** Call `match` with every Item Map at once, so a subject named in three chunks is one subject.
6. **Slice.** Put every subject that hits or nears one topic into one slice, so two drafters never write one topic. At most eight subjects per slice.
7. **Draft.** Send one [[wiki-draft]] per slice, or draft yourself when there are one or two slices.
8. **Assemble** the [[Wiki Change Plan]]:
   - every drafted write;
   - for each source: a modify of the source, with its description, tags, authority, `## Summary`, and `## Structure` (from the chunk summaries);
   - `absorbs`: the document ids; `work`: the plan or stub, when every document serves one;
   - `new_tags` when a write adds a tag that no document holds, and the user agreed in `tags: known` mode;
   - `title`: a short name ("Ingest the DINOv2 paper"); `notes`: what the change does, and every skipped subject with its reason.
   Past 25 new topics, keep the ones the most claims support and list the rest for a later run.
9. **Propose** the change.

## Gate

Show the [[Change Preview]] and link the change document, so the user can read the topics in Obsidian and edit one before the yes. Say the coverage: every chunk read, any chunk partial, the subjects skipped, the new tags. Wait for the yes, then apply. The change tool refuses apply in the same turn as the proposal.

A change with no writes (the documents held nothing new) needs no yes. Apply it and say so in one line: the documents are no longer pending.

## Hand off

[[wiki-map]] when new topics landed under two child tags of one tag; [[wiki-review]] after a large sync.
