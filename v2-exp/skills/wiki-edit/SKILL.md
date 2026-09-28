---
name: wiki-edit
description: "Change pages that exist: rewrite, rename, merge, split, move to another scope, or repair what lint found, as one reviewed change. Use for rewrite this page, rename, merge these pages, split this page, move this to the p3 area, fix the lint findings, fix the dead links."
---

# wiki-edit

Every change to a page that exists goes through one change document. Renames and
removes keep every link true, because code rewrites the links in the same commit.

Tools: `search`, `lint`, `change`. References: [changes.md](../atlas/references/changes.md),
[pages.md](../atlas/references/pages.md).

## Procedure

1. Resolve the pages to ids with `search`. For lint repairs, call `lint` and take the
   findings the user chose.
2. Read each page with Read, and the pages that link to it (`search` its title, or grep
   for `[[Title`).
3. Build the plan:
   - rewrite: a modify with the whole new body;
   - rename: a rename; code rewrites the links;
   - merge: a modify of the page that stays, with both contents, and a remove of the
     other with `redirect` set to the page that stays;
   - split: a modify of the page, and a create for each new page, linked both ways;
   - move to another scope: a modify of `scope`;
   - a dead link: create the target, or change the link in a modify.

## Gate

Propose, show the preview, wait for the yes, apply.

## Hand off

[wiki-review](../wiki-review/SKILL.md) to check the result, after a large repair.
