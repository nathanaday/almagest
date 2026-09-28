
# wiki-rollup

> Map a parent scope from its children. Where two children hold one subject, **upgrade** it to the parent. Where they hold related subjects, **bridge** them with a page at the parent. This is how the wiki grows up the context graph, and it replaces V1's members and mirrors.

**Use for**: organize the wiki, roll up the area, what do these repositories share, map the relationships, lift this to the area.

**Tools**: `context`, `search`, `match`, `change`. **Agents**: [[wiki-draft]]. **References**: `references/changes.md`, `references/pages.md`.

## Procedure

1. Call `context` for the area (or the vault). Its children are the scopes to compare.
2. Collect the pages of each child with `search` (`scope: <child>`).
3. Call `match` with those page ids and `siblings: true`. Each hit or near pair joins two children.
4. Decide each pair or group:
   - **the same subject** in two or more children → upgrade: keep one page, merge the other's claims into it, set its scope to the area, and remove the other with `redirect`;
   - **related subjects** → bridge: a concept page at the area that explains how they relate, linked from both;
   - **a policy** that holds in every child → upgrade it to the area;
   - **different subjects under one name** → rename one, so no reader confuses them.
   With the `areas` setting `many`, also propose a new area when a cluster of repositories inside this one shares much.
5. Send [[wiki-draft]] with the pairs when there are more than eight groups.
6. Build one change for the area.

## Gate

Propose, show the preview (the upgrades, the bridges, and the renames as three lists), wait for the yes, apply.

## Hand off

The same skill on the parent, when the user wants to go on up.
