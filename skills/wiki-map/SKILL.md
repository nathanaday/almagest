---
name: wiki-map
description: "Map a tag from the documents under it: write or refresh its overview topic; group the topics directly below its page under hub topics and set their up; where two child tags hold one subject, widen the topic's tags to the parent; where they hold related subjects, bridge them with a topic under the parent. Use for organize the wiki, map this tag, the page is crowded, group these topics, what do these repositories share, map the relationships, write the page for this tag, lift this to p3. Renaming or merging a tag is wiki-edit."
---

# wiki-map

The wiki grows up the tag tree. Topics start under narrow tags, such as a repository's
tag. For a tag, this skill compares the topics under its child tags. What two child
tags share widens to the parent tag, and what they hold in relation gets a bridge topic
under the parent. The tag's overview topic says what the tag holds. Repeat on the
parent tag, up to the top.

The skill also draws the map a reader walks down. A topic with an empty `up` sits
directly below the page of its tag, so a page can hold a hundred topics. This skill
groups them under hubs, topics that each give broad details about one domain, and sets
each member's `up` (Up in [pages.md](../almagest/references/pages.md)).

Tools: `context`, `search`, `match`, `change`. Agents:
[wiki-draft](../../agents/wiki-draft.md). References:
[changes.md](../almagest/references/changes.md), [pages.md](../almagest/references/pages.md).

## Procedure

1. Call `context` with `tags: [<tag>]` (or with no tags for the vault). Read the tag's
   page in `pages`, when one exists, and note the child tags and their counts in
   `tags`.
2. Collect the topics under the tag with `search` (`tags: [<tag>]`, `types: [topic]`,
   a high `limit`). Read them.
3. Call `match` with `tags: [<tag>]` and `across: true`. It compares each topic under the
   tag only with topics under a different child tag. Each hit or near pair joins two
   child tags.
4. Decide each pair or group, reading the topics:
   - **the same subject** under two or more child tags → widen: keep one topic, merge
     the other's claims into it with their citations, set its `tags` to the parent tag
     (or add both child tags), and remove the other with `redirect` to the topic that
     stays;
   - **related subjects** → bridge: a `create` of a concept under the parent tag that
     explains how they relate, linked from both;
   - **a policy** that holds under every child tag → widen its `tags` to the parent tag;
   - **different subjects under one name** → rename one, so no reader confuses them.
   In `tagging: open` mode, also propose a new child tag when a cluster of documents
   under this tag shares much and no child tag holds it: a `modify` of each document's
   `tags`. In `tagging: known` mode, name the new tag and set `new_tags: true` only
   after the user agreed.
5. **Hubs.** Read what sits directly below the tag's page: the topics under the tag
   whose `up` is empty or names the page (lint's `crowded` names a page with more than
   30). When more than about ten sit there, group them by subject. An overview's list
   of groups, such as a Topic map, is a first draft of the groups. For each group:
   - the hub: a topic that exists and is the broad article of the group (Transfer
     Learning for Fine-Tuning), or a `create` of a concept that gives broad details
     about the domain, links each member under `## Related`, and cites the sources of
     its members. Its `up` names the page, or a broader hub;
   - a `modify` of each member's `fields.up` to name its hub. A topic in two groups
     names both hubs. Keep the page in `up` only for a member that belongs directly
     below it too.
   Aim for groups of 5 to 15 topics. A group of more than 30 gets hubs of its own. Leave
   the sources: they take no `up`.
6. **The overview.** Write the tag's overview topic, or modify the one that exists: a
   `create` with `type: topic`, `kind: overview`, and `fields.defines` set to the tag,
   or a `modify` of the topic that defines it. Its body: `## Summary` (what the tag
   holds, for a reader), `## Context` (what an agent must know to work under the tag),
   and `## Related`. Code writes `## Map`. One document defines a tag; a repository
   already defines its own tag, so it gets no overview. Leave out a tag with few
   documents and nothing to say.
7. Send [wiki-draft](../../agents/wiki-draft.md) with the pairs when there are more than
   eight groups, at most eight per worker.
8. Build one change for the tag: `change` with `action: propose`, `title` ("Map
   #work/p3"), `notes` with each decision and its reason as five lists (the overview,
   the hubs, the widenings, the bridges, and the renames), and a `why` on every write.
   One change takes at most 100 writes; map a large page in several changes, one per
   group of hubs.

## Gate

Propose, then end the turn with one or two lines: the change document as a link, and
each new tag. The document's Summary and Notes hold the decisions. Wait for the yes,
then apply, or let the user decide in the document. The change tool refuses apply in the same
turn as the proposal.

## Hand off

The same skill on the parent tag, when the user wants to go on up.
