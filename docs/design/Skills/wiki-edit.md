# wiki-edit

> Change knowledge that exists: rewrite, rename, merge, split, retag, promote a stub to a topic, confirm a topic is current, or repair what lint found. One reviewed change.

**Use for**: rewrite this topic, rename, merge these topics, split this topic, tag this under p3, rename a tag, make this stub a topic, this page is still right, fix the lint findings, fix the dead links.

**Tools**: `search`, `lint`, `change`. **References**: `references/changes.md`, `references/pages.md`.

## Procedure

1. Resolve the documents to ids. For lint repairs, call `lint` and take the findings the user chose.
2. Read each document, and the documents that link to it.
3. Build the [[Wiki Change Plan]]:
   - rewrite: a modify, with `base`;
   - rename: a rename; code rewrites the links;
   - merge: a modify of the topic that stays, with both contents, and a remove of the other with `redirect`;
   - split: a modify of the topic, and a create for each new topic, linked both ways;
   - new tags on a document: a modify of `tags`;
   - rename or merge a tag: a `retag`; code rewrites the tag in every document, children too;
   - a stub that is one topic: a `promote`, with the kind, the fields, and the body; code keeps `## Idea` as `## Origin`;
   - a topic checked and still true: a `confirm`, with `base`;
   - a dead link: create the target, or change the link.

## Gate

Propose, show the preview (name each new tag, and count the documents a retag rewrites), wait for the yes, apply.

## Hand off

[[wiki-review]] to check the result, after a large repair.
