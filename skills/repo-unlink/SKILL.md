---
name: repo-unlink
description: "Unlink a repository from the vault: mark its document unlinked, and decide what happens to the threads and topics that depend on it. The document stays, so every link to it holds. The repository on disk is never touched. Use for unlink, remove this repository, forget this repo, the repository moved away. Linking is repo-link."
---

# repo-unlink

A repository can leave the vault while its code stays on disk. The repository document
stays too, marked `unlinked`, so the task lists that name it and the topics that cite it
keep a live link. Agents no longer work in the old folder. The open threads with tasks
for the repository and the topics under its tag must go somewhere, so this skill asks
once, with a recommendation for each group, and then makes the change and the `thread`
writes.

Tools: `context`, `search`, `thread`, `change`. References:
[changes.md](../atlas/references/changes.md),
[threads.md](../atlas/references/threads.md).

## Procedure

1. Call `context` with `repository`: its own tag (`defines`), and the open threads with
   a task list for it.
2. Count the two groups:
   - the threads that are not ended and have a task list for it: `thread` list with
     `repository`;
   - the topics under its own tag: `search` with `tags: [<its tag>]` and
     `types: [topic]`.
3. Ask one question with the counts and a recommended answer for each group:
   - open threads: keep them and drop the open tasks of that repository's list
     (recommended), or drop the threads;
   - topics under its tag: keep them (recommended; they still describe the system, and
     the tag and the repository document remain), or remove the ones the user names.
4. Build the change: a modify of the repository document with
   `fields: {unlinked: true}`. Code empties `path`, drops the git facts, and turns the
   lead callout into `[!repository-missing]`. Add a remove for each topic the user chose
   to remove, with `redirect` to a document that covers its subject when one exists.
   Never remove the repository document: `change` refuses it while a task list names it.
5. For the threads, per the answer: `thread` check with `task`, `state: dropped`, and
   `reason` for each open task of that list, or `thread` drop with `thread` and `reason`.

## Gate

Propose the change. Show the preview and the thread moves together. Wait for the yes,
then apply the change and make the `thread` writes. Apply runs sync, which takes the
path out of `.claude/settings.local.json`. Nothing touches the folder on disk.

## Hand off

None.
