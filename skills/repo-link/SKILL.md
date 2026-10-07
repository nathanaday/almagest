---
name: repo-link
description: "Link a git repository on this machine to the vault: a repository document that points at it, with its category tags and its own tag, written as one change. Use for link this repository, add a repo, work with ~/code/x, connect a repository. Describing its code is repo-ingest; unlinking is repo-unlink."
---

# repo-link

A repository document is the link between the vault and a repository on disk. An agent
may edit a repository only when a document names its path. The document's `tags` place
the repository among the user's categories, and its `defines` names the tag of its own
knowledge, so a lookup by tag finds the repository, its pages, and the policies that
apply to it. This skill writes one change: a create per repository.

Tools: `context`, `search`, `vault`, `change`. References:
[changes.md](../atlas/references/changes.md), [pages.md](../atlas/references/pages.md).

## Procedure

1. For each path: check that it exists and is the root of a git work tree
   (`git -C <path> rev-parse --show-toplevel` prints the path itself). Call `context`
   with `path`; when it resolves, the repository is linked already, so say so and skip
   it. A path inside the vault is refused.
2. Read the tag vocabulary: the opening context lists the most used tags and the
   tagging mode (`open` or `known`); `vault` status lists every tag with its count.
   Call `search` with `types: [repository]` to see the tags of the repositories linked
   already.
3. Choose each repository's `tags`: the category tags that exist and fit, by its name,
   its remote, its README, and the tags of its neighbors. Prefer a tag that exists over
   a new one. A tag is lower case: letters, digits, `-`, and `/` for nesting.
4. Choose its own tag, `defines`: its name under its main category tag (`work/p3` →
   `work/p3/p3-edge`), or the name alone when it has no category. The repository then
   holds the parent of that tag in `tags` (`work/p3`). One document defines a tag, so
   check that no document defines it already (`search` with `tags: [<the tag>]`).
5. In `tagging: known`, a tag that no document holds needs the user's yes. The new
   `defines` tag counts as new when no document holds it. Ask once, for every new tag
   of the change together, with the tags that exist as the other choice.
6. Read the repository's README and its instruction files (AGENTS.md, CLAUDE.md). Write
   its document: a `description` of one sentence, `aliases` (short names people use), a
   short `## What it is`, and a short `## Instructions` (the paths of the instruction
   files and what they require). Leave the other sections to repo-ingest. Give `path`
   as an absolute path or with `~/`. Code fills `remote`, `branch`, `head`, and the
   live status block.
7. Build one plan: a create per repository, `{op: create, type: repository, title,
   fields: {description, tags, aliases, defines, path}, body, why}`. Set `new_tags: true`
   on the plan only after the user agreed to the new tags.

## Gate

Propose the change. Show the preview: each repository with its path, its tags, and its
own tag, and each new tag. Wait for the yes, then apply. Apply runs sync, which lists the
repositories in `.claude/settings.local.json`, so later sessions in the vault may edit
them. For this session, tell the user to run `/add-dir <path>` for each, or to start a
new session.

## Hand off

[repo-ingest](../repo-ingest/SKILL.md), one repository at a time.
