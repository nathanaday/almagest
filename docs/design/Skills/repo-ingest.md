# repo-ingest

> Update the wiki from the contents of a repository. A repository is a codebase, so the work is to describe its components, its design patterns, its conventions, and its open issues from the code, which differs from ingesting a plain document.

**Use for**: describe this repository, map the repo, ingest the codebase, the repository document is behind, what is in this repo. Linking a repository is [[repo-link]].

**Tools**: `context`, `source` (capture, chunks, read), `match`, `change`. **Agents**: [[wiki-extract]], [[wiki-draft]]. **References**: `references/changes.md`, `references/pages.md`.

## Procedure

1. Call `context` for the repository: `described`, `behind`, the instruction files, the tag pages, and the policies that apply already.
2. Call `source` capture with `repository`. The snapshot is a source that holds the repository's own tag: the tree, the instruction files, the manifests, the docs, and every TODO and FIXME with its location, at the head commit.
3. Decide the reading:
   - never described: read the snapshot, the instruction files, the entry points, and the main packages;
   - behind: read `git log` and the diff from `described` to the head, and the parts they touch.
4. Build the Item Maps:
   - [[wiki-extract]] on each chunk of the snapshot;
   - from your own reading of the code: each **component** (an entity, tagged `component`), each **pattern** or design decision (a concept), each **convention** from the instruction files, lint and format configs, and contributing docs (a policy that holds the repository's own tag). Each claim is located by `path:line`.
5. Call `match` with every Item Map. Send [[wiki-draft]] for each slice, or draft yourself when the slices are few. Every new topic holds the repository's own tag, and more tags when the subject reaches further.
6. Build one [[Wiki Change Plan]]: the drafted writes, and a modify of the repository document (`## What it is`, `## How it is built`, `## Layout`, `## Components`, `## Instructions`). `absorbs` names the snapshot.
7. List the TODO, FIXME, and roadmap items as candidate stubs. They are work, not knowledge, so they never become topics.

## Gate

Propose the change, show the preview, and wait for the yes. Apply sets the document's `described` to the snapshot's commit. Then offer the candidate stubs as one list; `work` stub for each one the user picks, with the repository's own tag.

## Hand off

[[wiki-map]] on the repository's parent tag, when two or more repositories under it are described.
