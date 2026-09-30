# wiki-map

> Map a tag from the documents under it. Write or refresh its overview. Where two child tags hold one subject, **widen** the topic's tags to the parent. Where they hold related subjects, **bridge** them with a topic under the parent. This is how knowledge grows up the tag tree.

**Use for**: organize the wiki, map this tag, what do these repositories share, map the relationships, write the page for this tag, lift this to p3.

**Tools**: `context`, `search`, `match`, `change`. **Agents**: [[wiki-draft]]. **References**: `references/changes.md`, `references/pages.md`.

## Procedure

1. Call `context` with the tag (or with no tag for the vault). Read its page, when one exists, and note its child tags and their counts.
2. Collect the topics under the tag with `search` (`tags: [<tag>]`, `types: [topic]`).
3. Call `match` with `tags: [<tag>]` and `across: true`. Each hit or near pair joins two child tags.
4. Decide each pair or group:
   - **the same subject** under two or more child tags → widen: keep one topic, merge the other's claims into it, set its tags to the parent tag (or add both child tags), and remove the other with `redirect`;
   - **related subjects** → bridge: a concept under the parent tag that explains how they relate, linked from both;
   - **a policy** that holds under every child tag → widen its tags to the parent;
   - **different subjects under one name** → rename one, so no reader confuses them.
   In `tagging: open` mode, also propose a new child tag when a cluster of documents under this tag shares much and no child tag holds it.
5. **The overview.** Write the tag's overview topic (`kind: overview`, `defines: <tag>`), or modify it: `## Summary` for a reader, `## Context` for an agent, `## Related`. Leave out a tag with few documents and nothing to say.
6. Send [[wiki-draft]] with the pairs when there are more than eight groups.
7. Build one change for the tag.

## Gate

Propose, show the preview (the overview, the widenings, the bridges, and the renames as four lists), wait for the yes, apply.

## Hand off

The same skill on the parent tag, when the user wants to go on up.
