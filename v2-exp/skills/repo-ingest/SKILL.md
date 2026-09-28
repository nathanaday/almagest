---
name: repo-ingest
description: "Describe a linked repository in the wiki from its code: its components, design patterns, conventions, and open issues, and bring the description up to date after the code moves on. Use for describe this repository, map the repo, ingest the codebase, the repository page is behind, what is in this repo. Linking a repository is repo-link."
---

# repo-ingest

A repository is a codebase, not a document, so describing it means reading the code for
what the wiki should know: each component, each pattern or design decision, each
convention, and the work its TODO lines point at. The snapshot capture takes is the
source every claim cites.

Tools: `context`, `source` (capture, chunks, read), `match`, `change`. Agents:
[wiki-extract](../../agents/wiki-extract.md), [wiki-draft](../../agents/wiki-draft.md).
References: [changes.md](../atlas/references/changes.md),
[pages.md](../atlas/references/pages.md).

## Procedure

1. Call `context` for the repository: `repository.described`, `repository.behind`, the
   instruction files, and the policies already scoped to it.
2. Call `source` with `action: capture` and `repository`. The snapshot is a source: the
   tree, the instruction and convention files, the manifests, the docs, and every TODO
   and FIXME with its location, at the head commit.
3. Decide the reading:
   - never described (`described` is empty): read the snapshot, the instruction files,
     the entry points, and the main packages;
   - behind: read `git -C <path> log --stat <described>..HEAD` and the diff, and the
     parts they touch.
4. Build the Item Maps:
   - one [wiki-extract](../../agents/wiki-extract.md) per chunk of the snapshot;
   - from your own reading of the code: each component (an entity of kind `component`),
     each pattern or design decision (a concept), each convention from the instruction
     files, the lint and format configs, and the contributing docs (a policy scoped to
     the repository). Locate each claim by `path:line`.
5. Call `match` with every Item Map. Send [wiki-draft](../../agents/wiki-draft.md) for
   each slice, or draft yourself when the slices are one or two.
6. Build one plan: the drafted writes, and a modify of the repository page with
   `## What it is`, `## How it is built`, `## Layout`, `## Components`, and
   `## Instructions`. `absorbs` names the snapshot.
7. List the TODO, FIXME, and roadmap items as candidate thread stubs. They are work,
   not knowledge, so they never become wiki pages.

## Gate

Propose the change, show the preview, and wait for the yes. Apply sets the page's
`described` to the snapshot's commit. Then offer the candidate stubs as one list, and
call `thread` open for each one the user picks.

## Hand off

[wiki-rollup](../wiki-rollup/SKILL.md) when the repository's area now holds two or more
described repositories.
