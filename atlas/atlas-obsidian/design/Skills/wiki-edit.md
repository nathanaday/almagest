
# wiki-edit

> Change pages that exist: rewrite, rename, merge, split, move to another scope, or repair what lint found.

**Use for**: rewrite this page, rename, merge these pages, split this page, move this to the p3 area, fix the lint findings, fix the dead links.

**Tools**: `search`, `lint`, `change`. **References**: `references/changes.md`, `references/pages.md`.

## Procedure

1. Resolve the pages to ids. For lint repairs, call `lint` and take the findings the user chose.
2. Read each page, and the pages that link to it.
3. Build the [[Wiki Change Plan]]:
   - rewrite: a modify, with `base`;
   - rename: a rename; code rewrites the links;
   - merge: a modify of the page that stays, with both contents, and a remove of the other with `redirect`;
   - split: a modify of the page, and a create for each new page, linked both ways;
   - move to another scope: a modify of `scope`;
   - a dead link: create the target, or change the link.

## Gate

Propose, show the preview, wait for the yes, apply.

## Hand off

[[wiki-review]] to check the result, after a large repair.
