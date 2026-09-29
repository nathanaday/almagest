# repo-unlink

> Unlink a repository from the vault: mark its document `unlinked`, and decide what happens to the work that depends on it. The document stays, so every link to it holds. The repository on disk is never touched.

**Use for**: unlink, remove this repository, forget this repo, the repository moved away.

**Tools**: `context`, `search`, `work`, `change`. **References**: `references/changes.md`, `references/work.md`.

## Procedure

1. Call `context` for the repository. Count the documents that hold its own tag (`search` with `tags: [<its tag>]`) and the open or started plans that name it in `repositories`.
2. Ask one question with the counts and a recommended answer for each group:
   - open plans: keep them and drop the repository from `repositories` (recommended), or drop the plans;
   - the topics under its tag: keep them (recommended; they still describe the system, and the tag and its page remain), or remove the ones the user names.
3. Build the change: a modify of the repository document with `unlinked: true` and an empty `path` ([[Repository#Rules]]). Add a remove for each topic the user chose to remove.
4. For the plans: `work` set `repositories`, or `work` drop with the reason, per the answer.

## Gate

Propose the change. Show the preview and the plan moves. Wait for the yes, then apply the change and make the `work` writes.

## Hand off

None.
