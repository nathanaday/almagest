---
name: wiki-rollup
description: "Map a parent scope from its children: where two children hold one subject, upgrade it to the parent; where they hold related subjects, bridge them with a page at the parent. Use for organize the wiki, roll up the area, what do these repositories share, map the relationships, lift this to the area."
---

# wiki-rollup

The wiki grows from the leaves up. Each repository is mapped alone; then, in each area,
the children are compared. What two children share moves up to the area, and what they
hold in relation gets a bridge page there. Repeat to the top.

Tools: `context`, `search`, `match`, `change`. Agents:
[wiki-draft](../../agents/wiki-draft.md). References:
[changes.md](../atlas/references/changes.md), [pages.md](../atlas/references/pages.md).

## Procedure

1. Call `context` for the area (or the vault). Its `children` are the scopes to compare.
2. Collect the pages of each child with `search` (`scope` set to the child,
   `types: [concept, entity, policy]`, a high `limit`).
3. Call `match` with those page ids and `siblings: true`. Each hit or near pair joins
   two children.
4. Decide each pair or group, reading the pages:
   - **the same subject** in two or more children → upgrade: keep one page, merge the
     other's claims into it with their citations, set its `scope` to the area, and
     remove the other with `redirect` to the page that stays;
   - **related subjects** → bridge: a concept page at the area that explains how they
     relate, linked from both;
   - **a policy** that holds in every child → upgrade it to the area;
   - **different subjects under one name** → rename one, so no reader confuses them.
   With the `areas` setting `many`, also propose a new area when a cluster of
   repositories inside this one shares much.
5. Send [wiki-draft](../../agents/wiki-draft.md) with the pairs when there are more than
   eight groups.
6. Build one change for the area.

## Gate

Propose, then show the preview as three lists: the upgrades, the bridges, and the
renames. Wait for the yes, apply.

## Hand off

The same skill on the parent, when the user wants to go on up.
