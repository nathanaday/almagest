
# wiki-sync

> Absorb documents into the wiki through one pipeline: chunk, extract, match, draft, change. It serves every way knowledge enters: a captured source, a repository snapshot, a spec, a receipt.

**Use for**: sync the wiki, absorb the pending documents, update the wiki from this thread, wikify this, what does the wiki still need to learn.

**Tools**: `vault`, `source` (chunks, read), `match`, `change`. **Agents**: [[wiki-extract]], [[wiki-draft]]. **References**: `references/changes.md`, `references/pages.md`. See [[wiki-sync.canvas|the wiki-sync flow]].

## Procedure

1. **Documents.** Take the ids given, or the `pending` list from `vault`. Sources come first, then receipts, then specs. One change holds at most about ten documents or forty chunks; more is several changes, one after another.
2. **Chunks.** Call `source` chunks for each document.
3. **Extract.** Send one [[wiki-extract]] per chunk, in waves of at most eight. A single small chunk, you extract yourself. For a receipt, `## Learned` is the first thing to read.
4. **Check.** Each Item Map names its chunk and is complete. Send a `partial` chunk again, narrower.
5. **Match.** Call `match` with every Item Map at once, so a subject named in three chunks is one subject.
6. **Slice.** Put every subject that hits or nears one page into one slice, so two drafters never write one page. At most eight subjects per slice.
7. **Draft.** Send one [[wiki-draft]] per slice, or draft yourself when there are one or two slices.
8. **Assemble** the [[Wiki Change Plan]]:
   - every drafted write;
   - for each source: a modify of its source page, with its description, authority, `## Summary`, and `## Structure` (from the chunk summaries);
   - `absorbs`: the document ids; `thread`: the thread, when every document belongs to one;
   - `summary`: what the change does, the counts, and every skipped subject with its reason.
   Past 25 new pages, keep the ones the most claims support and list the rest for a later run.
9. **Propose** the change.

## Gate

Show the [[Change Preview]] and link the change document, so the user can read the pages in Obsidian and edit one before the yes. Say the coverage: every chunk read, any chunk partial, the subjects skipped. Wait for the yes, then apply.

A change with no writes (the documents held nothing new) needs no yes. Apply it and say so in one line: the documents are no longer pending.

## Hand off

[[wiki-rollup]] when new pages landed in two sibling scopes; [[wiki-review]] after a large sync.
