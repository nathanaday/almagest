---
name: wiki-edit
description: "Change knowledge that exists: rewrite, rename, merge, split, remove with a redirect, retag, confirm a topic is current, or repair what lint found, as one reviewed change. Use for rewrite this topic, rename, merge these topics, split this topic, tag this under p3, rename a tag, this page is still right, fix the lint findings, fix the dead links. New knowledge from documents is wiki-sync; unlinking a repository is repo-unlink."
---

# wiki-edit

Every change to knowledge that exists goes through one change document. Renames,
removes, and retags keep every link and tag true, because code rewrites them in the
same commit. A remove moves the document to `trash/`; undo moves it back.

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
   - remove: a `remove` with `redirect` set to the document that links now name; tell
     the user that the document goes to `trash/`;
   - new tags on a document: a `modify` of `fields.tags` (the whole list);
   - rename or merge a tag: a `retag` with `from` and `to`; code rewrites the tag in
     every document, the tags below it too; a `to` that exists merges the two;
   - a topic checked and still true: a `confirm` with `id`;
   - a dead link: create the target, or change the link in a `modify`.
   Give every write a `why`: one line that says why the change makes it.
   Leave `base` out unless you read the document's hash; code records it at propose,
   and apply refuses a write whose document changed since.
   Use a tag that exists before you make a new one. In `tagging: known` mode, set
   `new_tags: true` only after the user agreed to each new tag.

## Safe delete with backlinks

The palette's "Resolve with an agent" message names a document that the user wants to
delete, and that other files link: "Remove [[<title>]] …: point each backlink elsewhere,
or drop it, then propose a remove".

1. Find the backlinks. The message names at most ten ("4 more"), so also `search`
   the title and grep the vault for `[[<title>`.
2. For a knowledge document, build one plan:
   - when another document covers its subject, a `remove` with `redirect` set to it;
     code points the links there, but not in `scratchpad/`, `threads/`, or
     `journals/`;
   - otherwise, a `modify` of each linking knowledge document that points the link to
     another document or drops it, then a `remove`.
3. A link in `journals/` is the user's: name the file, and ask the user to change it.
4. For a file that is not a knowledge document (a note, an original), no change removes
   it. Point the links of knowledge documents elsewhere in a change, then tell the user
   to run Safe delete again. A source's original in `source-core/originals/` leaves only
   with its source.

## Gate

Propose with `change` `action: propose`, with `id` when a work document runs. End the
turn with one or two lines: the change document as a link, each new tag, the count of
documents a retag rewrites, and for a remove, that the document goes to `trash/`. Wait
for the yes in the chat, then apply, or let the user decide in the document. The change
tool refuses apply in the same turn as the proposal.

## Hand off

[wiki-review](../wiki-review/SKILL.md) to check the result, after a large repair.
