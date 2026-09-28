---
name: repo-link
description: "Link a git repository on this machine to the vault: a repository page that points at it, under the right area, written as one change. Use for link this repository, add a repo, work with ~/code/x, connect a repository. Describing its code is repo-ingest."
---

# repo-link

A repository page is the link between the vault and a repository on disk. The page sits
under an area of the context graph, so an agent that starts in the vault can find it
and reach its code. This skill writes one change: the new area pages, then the
repository pages.

Tools: `context`, `search`, `change`. References: [changes.md](../atlas/references/changes.md),
[pages.md](../atlas/references/pages.md).

## Procedure

1. For each path: check that it exists and is the root of a git work tree
   (`git -C <path> rev-parse --show-toplevel` prints the path itself). Call `context`
   with `path`; when it resolves, the repository is linked already, so say so and skip
   it. A path inside the vault is refused.
2. Choose each repository's parent by the vault's `areas` setting (in `Atlas.md`):
   - `many`: propose an area for each cluster (by name, by remote, or by the areas that
     exist), and create one when none fits;
   - `few`: propose a top-level area, or the vault;
   - `manual`: ask, and offer the existing areas and "none".
   The vault is the root of the graph: a page directly under it leaves `parent` empty.
   A new top-level area leaves its own `parent` empty too.
3. Read the repository's README and its instruction files (AGENTS.md, CLAUDE.md). Write
   its page: a `description` of one sentence, `aliases` (short names people use), and a
   short `## What it is`. Leave the other sections to repo-ingest. Give `path` as the
   user gave it or with `~/`; code fills `remote` and `branch`.
4. Build one plan: a create per new area (type area, `parent` and `description`), then
   a create per repository (type repository, `parent`, `description`, `aliases`,
   `path`, and the body).

## Gate

Propose the change. Show the preview: each repository with its parent, and each new
area. Wait for the yes, then apply. Apply lists the repositories in
`.claude/settings.local.json`, so later sessions in the vault may edit them. For this
session, tell the user to run `/add-dir <path>` for each, or to start a new session.

## Hand off

[repo-ingest](../repo-ingest/SKILL.md), one repository at a time.
