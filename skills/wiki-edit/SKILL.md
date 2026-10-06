---
name: wiki-edit
description: "Change knowledge that exists: rewrite, rename, merge, split, remove with a redirect, retag, confirm a topic is current, or repair what lint found, as one reviewed change. Use for rewrite this topic, rename, merge these topics, split this topic, tag this under p3, rename a tag, this page is still right, fix the lint findings, fix the dead links. New knowledge from documents is wiki-sync; unlinking a repository is repo-unlink."
---

# wiki-edit

Every change to knowledge that exists goes through one change document. Renames,
removes, and retags keep every link and tag true, because code rewrites them in the
same commit.

Tools: `search`, `lint`, `change`. References: [changes.md](../atlas/references/changes.md),
[pages.md](../atlas/references/pages.md).

## Procedure

1. Resolve the documents to ids with `search`. For lint repairs, call `lint` (with
   `tags` to check only the documents under them) and take the findings the user chose.
2. Read each document with Read, and the documents that link to it (`search` its title,
   or grep for `[[Title`).
3. Build the Wiki Change Plan, one write per move:
   - rewrite: a `modify` with the whole new `body`;
   - rename: a `rename` with the new `title`; code rewrites the links;
   - merge: a `modify` of the topic that stays, with both contents and both citations,
     and a `remove` of the other with `redirect` set to the topic that stays;
   - split: a `modify` of the topic, and a `create` (`type: topic`, `kind`, `title`,
     `fields`, `body`) for each new topic, linked both ways;
   - remove: a `remove` with `redirect` set to the document that links now name;
   - new tags on a document: a `modify` of `fields.tags` (the whole list);
   - rename or merge a tag: a `retag` with `from` and `to`; code rewrites the tag in
     every document, the tags below it too; a `to` that exists merges the two;
   - a topic checked and still true: a `confirm` with `id`;
   - a dead link: create the target, or change the link in a `modify`.
   Leave `base` out unless you read the document's hash; code records it at propose,
   and apply refuses a write whose document changed since.
   Use a tag that exists before you make a new one. In `tagging: known` mode, set
   `new_tags: true` only after the user agreed to each new tag.

## Gate

Propose with `change` `action: propose`. Show the preview: name each new tag, and count
the documents a retag rewrites. Wait for the yes, then apply. The change tool refuses
apply in the same turn as the proposal.

## Hand off

[wiki-review](../wiki-review/SKILL.md) to check the result, after a large repair.
