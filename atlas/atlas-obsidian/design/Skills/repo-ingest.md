
# repo-ingest

> Update the wiki using the contents of the repository. A repository is a codebase, so the work is to describe its components, its design patterns, its conventions, and its open issues from the code, which differs from ingesting a plain document.

**Use for**: describe this repository, map the repo, ingest the codebase, the repository page is behind, what is in this repo.

**Tools**: `context`, `source` (capture, chunks, read), `match`, `change`. **Agents**: [[wiki-extract]], [[wiki-draft]]. **References**: `references/changes.md`, `references/pages.md`. See [[repo-ingest.canvas|the repo-ingest flow]].

## Procedure

1. Call `context` for the repository: `described`, `behind`, the instruction files, the policies already scoped to it.
2. Call `source` capture with `repository`. The snapshot is a source: the tree, the instruction files, the manifests, the docs, and every TODO and FIXME with its location, at the head commit.
3. Decide the reading:
   - never described: read the snapshot, the instruction files, the entry points, and the main packages;
   - behind: read `git log` and the diff from `described` to the head, and the parts they touch.
4. Build the Item Maps:
   - [[wiki-extract]] on each chunk of the snapshot;
   - from your own reading of the code: each **component** (an entity of kind `component`), each **pattern** or design decision (a concept), each **convention** from the instruction files, lint and format configs, and contributing docs (a policy scoped to the repository). Each claim is located by `path:line`.
5. Call `match` with every Item Map. Send [[wiki-draft]] for each slice, or draft yourself when the slices are few.
6. Build one [[Wiki Change Plan]]: the drafted writes, and a modify of the repository page (`## What it is`, `## How it is built`, `## Layout`, `## Components`, `## Instructions`). `absorbs` names the snapshot.
7. List the TODO, FIXME, and roadmap items as candidate thread stubs. They are work, not knowledge, so they never become wiki pages.

## Gate

Propose the change, show the preview, and wait for the yes. Apply sets the page's `described` to the snapshot's commit. Then offer the candidate stubs as one list; `thread` open for each one the user picks.

## Hand off

[[wiki-rollup]] when the repository's area now holds two or more described repositories.
