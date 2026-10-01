---
name: repo-ingest
description: "Describe a linked repository in the wiki from its code: its components, design patterns, conventions, and open issues, and bring the description up to date after the code moves on. Use for describe this repository, map the repo, ingest the codebase, the repository document is behind, what is in this repo. Linking a repository is repo-link; ingesting a file or a paper is wiki-ingest."
---

# repo-ingest

A repository is a codebase, not a document, so describing it means reading the code for
what the wiki should know: each component, each pattern or design decision, each
convention, and the work its TODO lines point at. The snapshot that capture takes is the
source every claim cites. Every topic this skill writes holds the repository's own tag,
so a lookup under that tag finds it.

Tools: `context`, `source` (capture, chunks, read), `match`, `change`, `thread` (stub).
Agents: [wiki-extract](../../agents/wiki-extract.md), [wiki-draft](../../agents/wiki-draft.md).
References: [changes.md](../atlas/references/changes.md),
[pages.md](../atlas/references/pages.md).

## Procedure

1. Call `context` with `repository`: `repository.described`, `repository.behind`, the
   `instructions`, the tag `pages`, and the `policies` that apply already. Note the
   repository's own tag (`defines`); the rest of this skill calls it the repository's
   tag.
2. Call `source` with `action: capture` and `repository`. The snapshot is a source,
   titled `<repository> @ <commit>`: the tree, the instruction and convention files, the
   manifests, the docs, and every TODO and FIXME with its location, at the head commit.
   With no `tags` given, it holds the repository's tag.
3. Decide the reading:
   - never described (`described` is empty): read the snapshot, the instruction files,
     the entry points, and the main packages;
   - behind: read `git -C <path> log --stat <described>..HEAD` and the diff, and the
     parts they touch.
4. Build the Item Maps:
   - `source` chunks on the snapshot, then one
     [wiki-extract](../../agents/wiki-extract.md) per chunk;
   - from your own reading of the code, one more Item Map with `doc` set to the
     snapshot: each component (`kind: entity`, with the tag `component`), each pattern
     or design decision (`kind: concept`), and each convention from the instruction
     files, the lint and format configs, and the contributing docs (`kind: policy`, with
     its `strength`). Locate each claim by `path:line`.
   - Give each item the repository's tag in `tags`, and more tags that exist when the
     subject reaches past this repository (a language, a shared library, a category).
5. Call `match` with `items` set to every Item Map. Send
   [wiki-draft](../../agents/wiki-draft.md) for each slice of the Match Map, or draft
   yourself when the slices are one or two. A policy for this repository alone holds
   only the repository's tag; a policy that holds more tags reaches more repositories.
6. Build one plan: the drafted writes, and a modify of the repository document with
   `## What it is`, `## How it is built`, `## Layout`, `## Components` (links to the
   component entities, one line each), and `## Instructions`. `absorbs` names the
   snapshot. In `tagging: known`, ask before a tag that no document holds, and set
   `new_tags: true` after the yes.
7. List the TODO, FIXME, and roadmap items as candidate stubs. They are work, not
   knowledge, so they never become topics.

## Gate

Propose the change, show the preview, and wait for the yes. Apply sets the repository
document's `described` to the snapshot's commit. Then offer the candidate stubs as one
numbered list. For each one the user picks, call `thread` with `action: stub`, `text` (the
TODO line and its `path:line`, as written), a short `title`, and `tags` set to the
repository's tag.

## Hand off

[wiki-map](../wiki-map/SKILL.md) on the repository's parent tag, when two or more
repositories under that tag are described.
